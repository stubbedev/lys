package usb

import (
	"bytes"
	"encoding/binary"
	"unicode/utf16"
)

const (
	osStringIndex      = 0xEE
	osStringSignature  = "MSFT100"
	osStringLength     = 18
	compatIDIndex      = 4
	compatHeaderLength = 16
	compatFunctionSize = 24
)

// osVendorCode is the vendor request code a Microsoft OS string
// descriptor names; false for any other descriptor.
func osVendorCode(b []byte) (uint8, bool) {
	if len(b) < osStringLength || b[1] != 0x03 {
		return 0, false
	}
	units := make([]uint16, len(osStringSignature))
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(b[2+2*i:])
	}
	if string(utf16.Decode(units)) != osStringSignature {
		return 0, false
	}
	return b[16], true
}

// compatMTP is whether an extended compat ID descriptor gives the
// function holding interface iface the compatible ID MTP.
func compatMTP(b []byte, iface uint8) bool {
	if len(b) < compatHeaderLength || binary.LittleEndian.Uint16(b[6:]) != compatIDIndex {
		return false
	}
	count := int(b[8])
	for i := range count {
		start := compatHeaderLength + i*compatFunctionSize
		if start+compatFunctionSize > len(b) {
			return false
		}
		f := b[start : start+compatFunctionSize]
		if (f[0] == iface || count == 1) && string(bytes.TrimRight(f[2:10], "\x00")) == "MTP" {
			return true
		}
	}
	return false
}
