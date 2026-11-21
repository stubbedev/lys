package edid

const (
	displayIDDevice           = 0x0c
	displayIDDeviceSize       = 13
	displayIDDeviceOLED       = 0x50
	displayIDParameters       = 0x21
	displayIDParametersSize   = 29
	displayIDTechnologyOffset = 27
	displayIDTechnologyOLED   = 2
)

// ParseOLED reports whether b says the display is an organic LED panel,
// whose dark pixels draw less power. Only DisplayID states a display's
// technology: the Display Parameters data block of DisplayID 2.x and the
// Display Device data block of DisplayID 1.x. An EDID without either says
// nothing, and ParseOLED reports false. It fails with ErrInvalid on a bad
// base block, a DisplayID extension with a bad checksum or a data block
// that overruns.
func ParseOLED(b []byte) (bool, error) {
	if err := checkBase(b); err != nil {
		return false, err
	}
	oled := false
	err := eachExtension(b, tagDisplayID, "DisplayID", func(block []byte) error {
		return eachDisplayIDDataBlock(block, func(version, tag, _ byte, payload []byte) {
			oled = oled || statesOLED(version, tag, payload)
		})
	})
	if err != nil {
		return false, err
	}
	return oled, nil
}

func statesOLED(version, tag byte, payload []byte) bool {
	switch {
	case version >= displayIDVersion2 && tag == displayIDParameters && len(payload) >= displayIDParametersSize:
		return payload[displayIDTechnologyOffset]>>4&0x7 == displayIDTechnologyOLED
	case version < displayIDVersion2 && tag == displayIDDevice && len(payload) >= displayIDDeviceSize:
		return payload[0]&0xf0 == displayIDDeviceOLED
	}
	return false
}
