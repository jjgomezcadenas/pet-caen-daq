//go:build hdf5

package hdf5store

import (
	"errors"
	"fmt"
	"unsafe"

	hdf5 "github.com/next-exp/hdf5-go"

	"github.com/jmbenlloch/pet-caen-daq/backend/internal/holddelay"
)

type holdDelayPointRow struct {
	DelayNS, EffectiveDelayNS, EventsCollected uint32
	ElapsedNanoseconds                         uint64
}
type holdDelayBinRow struct {
	PointIndex uint32
	Channel    uint8
	Bin        uint16
	Count      uint32
}
type holdDelaySpectrumRow struct {
	PointIndex    uint32
	Channel       uint8
	Gain          uint8 // 0 high, 1 low
	Discriminator uint8 // 1 when the channel's discriminator fired
	Value         uint16
	Count         uint32
}
type holdDelayMissingRow struct {
	PointIndex uint32
	Channel    uint8
	Count      uint32
}

func WriteHoldDelay(path string, metadata []byte, points []holddelay.Point) (err error) {
	compression, err := histogramCompressionName()
	if err != nil {
		return err
	}
	file, err := hdf5.CreateFile(path, hdf5.F_ACC_EXCL)
	if err != nil {
		return fmt.Errorf("create hold-delay HDF5: %w", err)
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	root, err := file.OpenGroup("/")
	if err != nil {
		return err
	}
	defer root.Close()
	if err := writeUintAttribute(root, "schema_version", 1); err != nil {
		return err
	}
	group, err := file.CreateGroup("hold_delay")
	if err != nil {
		return err
	}
	defer group.Close()
	if err := createBytes(group, "metadata_json", metadata); err != nil {
		return err
	}
	pointDataset, err := createTable(group, "points", compoundHoldDelayPoint(), compression)
	if err != nil {
		return err
	}
	defer pointDataset.Close()
	binDataset, err := createTable(group, "nonzero_hg_bins", compoundHoldDelayBin(), compression)
	if err != nil {
		return err
	}
	defer binDataset.Close()
	missingDataset, err := createTable(group, "missing_channels", compoundHoldDelayMissing(), compression)
	if err != nil {
		return err
	}
	defer missingDataset.Close()
	spectrumDataset, err := createTable(group, "full_spectra", compoundHoldDelaySpectrum(), compression)
	if err != nil {
		return err
	}
	defer spectrumDataset.Close()
	var pointRows []holdDelayPointRow
	var binRows []holdDelayBinRow
	var missingRows []holdDelayMissingRow
	var spectrumRows []holdDelaySpectrumRow
	for pointIndex, point := range points {
		for _, bin := range point.FullSpectra {
			row := holdDelaySpectrumRow{PointIndex: uint32(pointIndex), Channel: bin.Channel, Value: bin.Value, Count: bin.Count}
			if bin.LowGain {
				row.Gain = 1
			}
			if bin.Discriminator {
				row.Discriminator = 1
			}
			spectrumRows = append(spectrumRows, row)
		}
		pointRows = append(pointRows, holdDelayPointRow{point.DelayNS, point.EffectiveDelay, point.EventsCollected, uint64(point.Elapsed)})
		for channel := 0; channel < holddelay.ChannelCount; channel++ {
			if count := point.MissingChannels[channel]; count > 0 {
				missingRows = append(missingRows, holdDelayMissingRow{uint32(pointIndex), uint8(channel), count})
			}
			for bin, count := range point.HighGainBins[channel] {
				if count > 0 {
					binRows = append(binRows, holdDelayBinRow{uint32(pointIndex), uint8(channel), uint16(bin), count})
				}
			}
		}
	}
	for dataset, rows := range map[*hdf5.Dataset]any{pointDataset: pointRows, binDataset: binRows, missingDataset: missingRows, spectrumDataset: spectrumRows} {
		target := table{dataset: dataset}
		switch values := rows.(type) {
		case []holdDelayPointRow:
			err = appendRows(&target, values)
		case []holdDelayBinRow:
			err = appendRows(&target, values)
		case []holdDelayMissingRow:
			err = appendRows(&target, values)
		case []holdDelaySpectrumRow:
			err = appendRows(&target, values)
		}
		if err != nil {
			return err
		}
	}
	return file.Flush(hdf5.F_SCOPE_GLOBAL)
}

func compoundHoldDelayPoint() *hdf5.CompoundType {
	var value holdDelayPointRow
	return mustCompound(unsafe.Sizeof(value), []field{
		{"delay_ns", unsafe.Offsetof(value.DelayNS), hdf5.T_STD_U32LE},
		{"effective_delay_ns", unsafe.Offsetof(value.EffectiveDelayNS), hdf5.T_STD_U32LE},
		{"events_collected", unsafe.Offsetof(value.EventsCollected), hdf5.T_STD_U32LE},
		{"elapsed_nanoseconds", unsafe.Offsetof(value.ElapsedNanoseconds), hdf5.T_STD_U64LE},
	})
}
func compoundHoldDelayBin() *hdf5.CompoundType {
	var value holdDelayBinRow
	return mustCompound(unsafe.Sizeof(value), []field{
		{"point_index", unsafe.Offsetof(value.PointIndex), hdf5.T_STD_U32LE},
		{"channel", unsafe.Offsetof(value.Channel), hdf5.T_STD_U8LE},
		{"bin", unsafe.Offsetof(value.Bin), hdf5.T_STD_U16LE},
		{"count", unsafe.Offsetof(value.Count), hdf5.T_STD_U32LE},
	})
}
func compoundHoldDelayMissing() *hdf5.CompoundType {
	var value holdDelayMissingRow
	return mustCompound(unsafe.Sizeof(value), []field{
		{"point_index", unsafe.Offsetof(value.PointIndex), hdf5.T_STD_U32LE},
		{"channel", unsafe.Offsetof(value.Channel), hdf5.T_STD_U8LE},
		{"count", unsafe.Offsetof(value.Count), hdf5.T_STD_U32LE},
	})
}
func compoundHoldDelaySpectrum() *hdf5.CompoundType {
	var value holdDelaySpectrumRow
	return mustCompound(unsafe.Sizeof(value), []field{
		{"point_index", unsafe.Offsetof(value.PointIndex), hdf5.T_STD_U32LE},
		{"channel", unsafe.Offsetof(value.Channel), hdf5.T_STD_U8LE},
		{"gain", unsafe.Offsetof(value.Gain), hdf5.T_STD_U8LE},
		{"discriminator", unsafe.Offsetof(value.Discriminator), hdf5.T_STD_U8LE},
		{"value", unsafe.Offsetof(value.Value), hdf5.T_STD_U16LE},
		{"count", unsafe.Offsetof(value.Count), hdf5.T_STD_U32LE},
	})
}
