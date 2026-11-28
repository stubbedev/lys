package usb

import (
	"testing"
	"unicode/utf16"
)

func osString(signature string, code uint8) []byte {
	b := []byte{osStringLength, 0x03}
	for _, u := range utf16.Encode([]rune(signature)) {
		b = append(b, byte(u), byte(u>>8))
	}
	return append(b, code, 0)
}

func compatID(functions ...[2]string) []byte {
	b := make([]byte, compatHeaderLength)
	b[4], b[6], b[8] = 0x00, compatIDIndex, byte(len(functions))
	for _, f := range functions {
		fn := make([]byte, compatFunctionSize)
		fn[0] = f[0][0] - '0'
		copy(fn[2:10], f[1])
		b = append(b, fn...)
	}
	return b
}

func TestOSVendorCodeReadsTheMicrosoftString(t *testing.T) {
	t.Parallel()
	if code, ok := osVendorCode(osString("MSFT100", 0xfe)); !ok || code != 0xfe {
		t.Fatalf("osVendorCode = %#x, %t", code, ok)
	}
	for _, b := range [][]byte{osString("MSFT200", 1), osString("MSFT100", 1)[:10], {}} {
		if _, ok := osVendorCode(b); ok {
			t.Errorf("osVendorCode accepted %x", b)
		}
	}
}

func TestCompatMTPMatchesTheInterfacesFunction(t *testing.T) {
	t.Parallel()
	if !compatMTP(compatID([2]string{"0", "MTP"}), 0) {
		t.Fatal("a single MTP function is not MTP")
	}
	if !compatMTP(compatID([2]string{"0", "WINUSB"}, [2]string{"1", "MTP"}), 1) {
		t.Fatal("the second function's MTP is not interface 1's")
	}
	if compatMTP(compatID([2]string{"0", "WINUSB"}, [2]string{"1", "MTP"}), 0) {
		t.Fatal("interface 0 took interface 1's MTP")
	}
	if compatMTP(compatID([2]string{"0", "MTP"})[:30], 0) {
		t.Fatal("a truncated descriptor is MTP")
	}
}
