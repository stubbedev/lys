//go:build linux && (amd64 || arm64 || 386 || arm || riscv64 || loong64 || s390x)

package usb

import (
	"testing"
	"unsafe"
)

func TestRequestStructsMatchTheKernelsLayout(t *testing.T) {
	t.Parallel()
	ptr := unsafe.Sizeof(uintptr(0))
	urbSize, ctrlSize, bufferAt := uintptr(44), uintptr(16), uintptr(12)
	if ptr == 8 {
		urbSize, ctrlSize, bufferAt = 56, 24, 16
	}
	if got := unsafe.Sizeof(urb{}); got != urbSize {
		t.Errorf("usbdevfs_urb is %d bytes, want %d", got, urbSize)
	}
	if got := unsafe.Sizeof(ctrlTransfer{}); got != ctrlSize {
		t.Errorf("usbdevfs_ctrltransfer is %d bytes, want %d", got, ctrlSize)
	}
	if got := unsafe.Sizeof(disconnectClaim{}); got != 264 {
		t.Errorf("usbdevfs_disconnect_claim is %d bytes, want 264", got)
	}
	if off := unsafe.Offsetof(urb{}.buffer); off != bufferAt {
		t.Errorf("usbdevfs_urb.buffer is at %d, want %d", off, bufferAt)
	}
	if off := unsafe.Offsetof(urb{}.usercontext); off != urbSize-ptr {
		t.Errorf("usbdevfs_urb.usercontext is at %d, want %d", off, urbSize-ptr)
	}
	if off := unsafe.Offsetof(ctrlTransfer{}.data); off != ctrlSize-ptr {
		t.Errorf("usbdevfs_ctrltransfer.data is at %d, want %d", off, ctrlSize-ptr)
	}
}

func TestRequestNumbersMatchTheKernelHeaders(t *testing.T) {
	t.Parallel()
	want := map[string][2]uintptr{
		"USBDEVFS_CONTROL":          {reqControl, 0xC0105500},
		"USBDEVFS_SUBMITURB":        {reqSubmitURB, 0x802C550A},
		"USBDEVFS_DISCARDURB":       {reqDiscardURB, 0x0000550B},
		"USBDEVFS_REAPURBNDELAY":    {reqReapURBNDelay, 0x4004550D},
		"USBDEVFS_RELEASEINTERFACE": {reqReleaseIface, 0x80045510},
		"USBDEVFS_CLEAR_HALT":       {reqClearHalt, 0x80045515},
		"USBDEVFS_DISCONNECT_CLAIM": {reqDisconnectClaim, 0x8108551B},
	}
	if unsafe.Sizeof(uintptr(0)) == 8 {
		want["USBDEVFS_CONTROL"] = [2]uintptr{reqControl, 0xC0185500}
		want["USBDEVFS_SUBMITURB"] = [2]uintptr{reqSubmitURB, 0x8038550A}
		want["USBDEVFS_REAPURBNDELAY"] = [2]uintptr{reqReapURBNDelay, 0x4008550D}
	}
	for name, v := range want {
		if v[0] != v[1] {
			t.Errorf("%s is %#x, want %#x", name, v[0], v[1])
		}
	}
}
