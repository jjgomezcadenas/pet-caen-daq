package dt5202

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmbenlloch/pet-caen-daq/backend/internal/janusconfig"
)

// The value FERSlib Configure5202 writes with every JANUS default
// (FastShaperInput HG-PA, HV_Adjust_Range 4.5).
const fersDefaultCitirocConfig = 0x00070f21

func TestCitirocModesRegisterMatchesFERSlibDefaults(t *testing.T) {
	cases := []struct {
		name  string
		modes CitirocModes
		want  uint32
	}{
		{"defaults", CitirocModes{InputDACReference45V: true}, fersDefaultCitirocConfig},
		{"fast shaper on low gain", CitirocModes{FastShaperOnLowGain: true, InputDACReference45V: true}, fersDefaultCitirocConfig | 1<<7},
		{"input DAC 2.5 V", CitirocModes{}, fersDefaultCitirocConfig &^ (1 << 8)},
	}
	for _, c := range cases {
		if got := c.modes.Register(); got != c.want {
			t.Errorf("%s: register = %#08x, want %#08x", c.name, got, c.want)
		}
	}
}

func TestCitirocModesSetMatchingStreamBits(t *testing.T) {
	for _, modes := range []CitirocModes{{}, {FastShaperOnLowGain: true}, {InputDACReference45V: true}} {
		var chip CitirocChip
		modes.ApplyTo(&chip.Common)
		stream, err := chip.Stream()
		if err != nil {
			t.Fatal(err)
		}
		register := modes.Register()
		for _, pair := range []struct{ registerBit, streamBit int }{
			{0, 258}, {5, 309}, {7, 328}, {8, 330}, {9, 1133}, {10, 1141}, {11, 1143},
		} {
			want := uint32(register>>pair.registerBit) & 1
			got, err := stream.Field(pair.streamBit, 1)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Errorf("%+v: stream bit %d = %d, register bit %d = %d", modes, pair.streamBit, got, pair.registerBit, want)
			}
		}
	}
}

func TestPlanWritesCitirocConfigFromConfiguration(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("..", "..", "..", "test", "fixtures", "janus", "config_same4_v3_good.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		fastShaper, hvRange string
		want                uint32
	}{
		{"HG-PA", "4.5", fersDefaultCitirocConfig},
		{"LG-PA", "4.5", fersDefaultCitirocConfig | 1<<7},
		{"HG-PA", "2.5", fersDefaultCitirocConfig &^ (1 << 8)},
		{"HG-PA", "DISABLED", fersDefaultCitirocConfig},
	} {
		t.Run(test.fastShaper+"/"+test.hvRange, func(t *testing.T) {
			text := string(fixture) + "\nFastShaperInput " + test.fastShaper + "\nHV_Adjust_Range " + test.hvRange + "\n"
			doc, parseErr := janusconfig.Parse(strings.NewReader(text))
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			plan, planErr := PlanProductionConfiguration(doc, 0)
			if planErr != nil {
				t.Fatal(planErr)
			}
			var got []uint32
			final := make(map[Register]uint32, len(plan.Writes))
			for _, write := range plan.Writes {
				final[write.Address] = write.Value
				if write.Address == CitirocConfig {
					got = append(got, write.Value)
				}
			}
			if len(got) != 1 || got[0] != test.want {
				t.Fatalf("CitirocConfig writes = %#x, want one write of %#08x", got, test.want)
			}
			if err := plan.ValidateReadback(final); err != nil {
				t.Fatalf("readback of the planned values rejected: %v", err)
			}
			stream, err := plan.Citiroc[0].Stream()
			if err != nil {
				t.Fatal(err)
			}
			if bit, _ := stream.Field(328, 1); bit != (test.want>>7)&1 {
				t.Errorf("planned stream fast-shaper bit 328 = %d, register %#08x", bit, test.want)
			}
		})
	}
}
