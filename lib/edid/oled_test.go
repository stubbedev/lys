package edid

import (
	"errors"
	"testing"
)

func displayID(version byte, blocks ...[]byte) []byte {
	b := make([]byte, blockSize)
	b[0], b[1] = tagDisplayID, version
	at := 5
	for _, d := range blocks {
		at += copy(b[at:], d)
	}
	b[2] = byte(at - 5)
	var sum byte
	for _, c := range b[1:at] {
		sum += c
	}
	b[at] = -sum
	seal(b)
	return b
}

func parameters(technology byte) []byte {
	payload := make([]byte, displayIDParametersSize)
	payload[displayIDTechnologyOffset] = technology<<4 | 0x03
	return append([]byte{displayIDParameters, 0, displayIDParametersSize}, payload...)
}

func device(technology byte) []byte {
	payload := make([]byte, displayIDDeviceSize)
	payload[0] = technology
	return append([]byte{displayIDDevice, 0, displayIDDeviceSize}, payload...)
}

func TestParseOLEDReadsTheDisplayTechnology(t *testing.T) {
	for name, c := range map[string]struct {
		edid []byte
		want bool
	}{
		"DisplayID 2 AMOLED":              {withExtension(t, displayID(displayIDVersion2, parameters(2))), true},
		"DisplayID 2 AMLCD":               {withExtension(t, displayID(displayIDVersion2, parameters(1))), false},
		"DisplayID 2 unspecified":         {withExtension(t, displayID(displayIDVersion2, parameters(0))), false},
		"DisplayID 2 too short":           {withExtension(t, displayID(displayIDVersion2, append([]byte{displayIDParameters, 0, 4}, 0, 0, 0, 0))), false},
		"DisplayID 1 organic LED":         {withExtension(t, displayID(0x12, device(0x50))), true},
		"DisplayID 1 active-matrix IPS":   {withExtension(t, displayID(0x12, device(0x15))), false},
		"DisplayID 1 parameters ignored":  {withExtension(t, displayID(0x12, parameters(2))), false},
		"DisplayID 2 device ignored":      {withExtension(t, displayID(displayIDVersion2, device(0x50))), false},
		"no DisplayID":                    {base(t), false},
		"a real OLED without DisplayID":   {readEDID(t, "oled-no-range.bin"), true},
		"an HDR LCD's black level":        {readEDID(t, "amd-vsdb-over-range-limits.bin"), false},
		"an unstated black level":         {readEDID(t, "range-limits-offset.bin"), false},
		"DisplayID without the parameter": {readEDID(t, "displayid2-adaptive-sync.bin"), false},
	} {
		got, err := ParseOLED(c.edid)
		if err != nil || got != c.want {
			t.Errorf("%s: %v, %v, want %v, nil", name, got, err, c.want)
		}
	}
}

func TestParseOLEDRefusesWhatItCannotTrust(t *testing.T) {
	badSection := withExtension(t, displayID(displayIDVersion2, parameters(2)))
	badSection[blockSize+8]++
	badSection[2*blockSize-1]--
	badBlock := withExtension(t, displayID(displayIDVersion2, parameters(2)))
	badBlock[2*blockSize-1]++
	for name, b := range map[string][]byte{
		"a bad DisplayID section checksum": badSection,
		"a bad DisplayID block checksum":   badBlock,
		"no header":                        {1, 2, 3},
	} {
		if got, err := ParseOLED(b); !errors.Is(err, ErrInvalid) || got {
			t.Errorf("%s: %v, %v, want false, ErrInvalid", name, got, err)
		}
	}
}
