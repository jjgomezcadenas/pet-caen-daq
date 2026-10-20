package dt5202

// CitirocConfig register (0x01000108) bit positions, from FERSlib
// FERS_Registers_520X.h (crcfg_*). With automatic ASIC configuration the FPGA
// firmware copies these bits into the Citiroc slow-control stream; the stream
// bit each one drives is given in the comment.
const (
	citirocCfgChargeLatch            = 0  // stream bit 258
	citirocCfgTrackHoldHigh          = 2  // stream bit 306: high gain track & hold instead of peak detector
	citirocCfgTrackHoldLow           = 3  // stream bit 307: low gain track & hold instead of peak detector
	citirocCfgPeakSensingTrigger     = 5  // stream bit 309: peak sensing triggered externally
	citirocCfgFastShaperOnLowGain    = 7  // stream bit 328
	citirocCfgInputDACReference45V   = 8  // stream bit 330
	citirocCfgOTAForceOn             = 9  // stream bit 1133
	citirocCfgNegativeTrigger        = 10 // stream bit 1141
	citirocCfgEnableChannelTriggers  = 11 // stream bit 1143
	citirocCfgEnableGlobalTrigger    = 16 // FPGA: propagate gtrg to the Citiroc global_trig pin
	citirocCfgEnableValidation       = 17 // FPGA: propagate gate (not veto) to the Citiroc val_evt pin
	citirocCfgRepeatChargeDiscrReset = 18 // FPGA: repeat raz_chn while the charge latch is not reset
)

// CitirocModes holds the operator-selectable Citiroc modes that JANUS writes
// through the CitirocConfig register. Every other bit is fixed to the value
// FERSlib (FERS_configure_5202.c, Configure5202) writes with its defaults:
// latched charge discriminator, external peak-sensing trigger, OTA forced on,
// negative trigger polarity, channel triggers enabled, global trigger and
// validation propagated, and repeated charge-discriminator reset.
type CitirocModes struct {
	FastShaperOnLowGain  bool // FastShaperInput LG-PA
	InputDACReference45V bool // HV_Adjust_Range 4.5 or DISABLED
	TrackAndHold         bool // PeakDetectorMode TRACK&HOLD, both gains
}

// Register returns the CitirocConfig register value for these modes.
func (m CitirocModes) Register() uint32 {
	v := uint32(1)<<citirocCfgChargeLatch |
		1<<citirocCfgPeakSensingTrigger |
		1<<citirocCfgOTAForceOn |
		1<<citirocCfgNegativeTrigger |
		1<<citirocCfgEnableChannelTriggers |
		1<<citirocCfgEnableGlobalTrigger |
		1<<citirocCfgEnableValidation |
		1<<citirocCfgRepeatChargeDiscrReset
	if m.FastShaperOnLowGain {
		v |= 1 << citirocCfgFastShaperOnLowGain
	}
	if m.InputDACReference45V {
		v |= 1 << citirocCfgInputDACReference45V
	}
	if m.TrackAndHold {
		v |= 1<<citirocCfgTrackHoldHigh | 1<<citirocCfgTrackHoldLow
	}
	return v
}

// ApplyTo sets the expected-stream fields that the firmware derives from the
// register, so that the planned stream describes what the register requests.
func (m CitirocModes) ApplyTo(common *CitirocCommon) {
	common.ChargeDiscriminatorLatch = true
	common.PeakSensingExternalTrigger = true
	common.FastShaperOnLowGain = m.FastShaperOnLowGain
	common.InputDACReference45V = m.InputDACReference45V
	common.PeakDetectorHigh = m.TrackAndHold
	common.PeakDetectorLow = m.TrackAndHold
	common.OTAForceOn = true
	common.NegativeTriggerPolarity = true
	common.EnableChannelTriggers = true
}
