//go:build linux && (amd64 || arm64 || 386 || arm || riscv64 || loong64 || s390x)

package usb

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

type urb struct {
	typ          uint8
	endpoint     uint8
	status       int32
	flags        uint32
	buffer       unsafe.Pointer
	bufferLength int32
	actualLength int32
	startFrame   int32
	streamID     uint32
	errorCount   int32
	signr        uint32
	usercontext  unsafe.Pointer
}

type ctrlTransfer struct {
	requestType uint8
	request     uint8
	value       uint16
	index       uint16
	length      uint16
	timeout     uint32
	data        unsafe.Pointer
}

type disconnectClaim struct {
	iface  uint32
	flags  uint32
	driver [256]byte
}

const (
	iocNone  = 0
	iocWrite = 1
	iocRead  = 2
	iocType  = 'U'

	reqControl         = (iocRead|iocWrite)<<30 | unsafe.Sizeof(ctrlTransfer{})<<16 | iocType<<8
	reqSubmitURB       = iocRead<<30 | unsafe.Sizeof(urb{})<<16 | iocType<<8 | 10
	reqDiscardURB      = iocNone<<30 | iocType<<8 | 11
	reqReapURBNDelay   = iocWrite<<30 | unsafe.Sizeof(uintptr(0))<<16 | iocType<<8 | 13
	reqReleaseIface    = iocRead<<30 | unsafe.Sizeof(uint32(0))<<16 | iocType<<8 | 16
	reqClearHalt       = iocRead<<30 | unsafe.Sizeof(uint32(0))<<16 | iocType<<8 | 21
	reqDisconnectClaim = iocRead<<30 | unsafe.Sizeof(disconnectClaim{})<<16 | iocType<<8 | 27

	urbTypeInterrupt  = 1
	urbTypeBulk       = 3
	urbZeroPacket     = 0x40
	claimExceptDriver = 0x02
)

func ioctl[T any](fd int, req uintptr, arg *T) (int, error) {
	return syscallIoctl(fd, req, uintptr(unsafe.Pointer(arg))) //nolint:gosec // G103: usbfs takes its request structs by address
}

func dataPointer(b []byte) unsafe.Pointer {
	if len(b) == 0 {
		return nil
	}
	return unsafe.Pointer(unsafe.SliceData(b)) //nolint:gosec // G103: usbfs reads and fills the buffer at this address
}

func urbKey(u *urb) uintptr {
	return uintptr(unsafe.Pointer(u)) //nolint:gosec // G103: the address usbfs reaps a URB by
}

//go:uintptrescapes
func syscallIoctl(fd int, req, arg uintptr) (int, error) {
	for {
		r, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, arg)
		switch errno {
		case 0:
			return int(r), nil
		case unix.EINTR:
			continue
		}
		return 0, errno
	}
}
