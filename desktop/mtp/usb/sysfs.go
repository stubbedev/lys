// Package usb finds still image and MTP interfaces in sysfs and carries
// PTP containers over their pipes through Linux usbfs.
package usb

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/lib/intconv"
)

// SysfsDevices is the sysfs directory listing every USB device and
// interface.
const SysfsDevices = "/sys/bus/usb/devices"

// Protocol is what an interface speaks.
type Protocol uint8

// The protocols told apart.
const (
	// PTP is a still image class interface: a camera.
	PTP Protocol = iota + 1
	// MTP is an interface that names itself MTP or that its Microsoft
	// OS descriptor calls MTP: a phone or media player.
	MTP
	// Vendor is a vendor-specific interface shaped like Android's MTP
	// interface, which only its Microsoft OS descriptor tells apart;
	// Probe asks it.
	Vendor
)

func (p Protocol) String() string {
	switch p {
	case PTP:
		return "PTP"
	case MTP:
		return "MTP"
	case Vendor:
		return "vendor-specific"
	}
	return fmt.Sprintf("protocol %d", uint8(p))
}

// Interface is an interface's number and its three endpoints.
type Interface struct {
	Number    uint8
	BulkIn    uint8
	BulkOut   uint8
	Interrupt uint8
}

// Device is a USB device with a PTP or MTP interface.
type Device struct {
	// Name is the device's sysfs name, such as 1-2.3.
	Name         string
	Bus          int
	Address      int
	VendorID     uint16
	ProductID    uint16
	Manufacturer string
	Product      string
	Serial       string
	Protocol     Protocol
	Interface    Interface
}

// Node is the usbfs device node.
func (d Device) Node() string { return fmt.Sprintf("/dev/bus/usb/%03d/%03d", d.Bus, d.Address) }

// Scan lists the devices under root, a sysfs USB devices directory, that
// have a PTP, MTP or MTP-shaped vendor interface, sorted by Name. A
// device has the first such interface in sysfs order; one unplugged while it
// is read is left out.
func Scan(root string) ([]Device, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("usb: list %s: %w", root, err)
	}
	var devices []Device
	for _, e := range entries {
		name, _, ok := strings.Cut(e.Name(), ":")
		if !ok || (len(devices) > 0 && devices[len(devices)-1].Name == name) {
			continue
		}
		iface, protocol, ok, err := readInterface(filepath.Join(root, e.Name()))
		if errors.Is(err, fs.ErrNotExist) || (err == nil && !ok) {
			continue
		}
		if err != nil {
			return nil, err
		}
		d, err := readDevice(filepath.Join(root, name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		d.Name, d.Protocol, d.Interface = name, protocol, iface
		devices = append(devices, d)
	}
	slices.SortFunc(devices, func(a, b Device) int { return strings.Compare(a.Name, b.Name) })
	return devices, nil
}

const (
	classStillImage = 0x06
	classVendor     = 0xff
	attrTransfer    = 0x03
	transferBulk    = 0x02
	transferIntr    = 0x03
	directionIn     = 0x80
)

func readInterface(dir string) (Interface, Protocol, bool, error) {
	var attrs [4]uint8
	for i, name := range []string{"bInterfaceClass", "bInterfaceSubClass", "bInterfaceProtocol", "bInterfaceNumber"} {
		v, err := readUint[uint8](dir, name, 16)
		if err != nil {
			return Interface{}, 0, false, err
		}
		attrs[i] = v
	}
	label, err := readOptional(dir, "interface")
	if err != nil {
		return Interface{}, 0, false, err
	}
	iface, err := readEndpoints(dir)
	if err != nil {
		return Interface{}, 0, false, err
	}
	iface.Number = attrs[3]
	protocol, ok := classify(attrs[0], attrs[1], attrs[2], label, iface)
	return iface, protocol, ok, nil
}

func classify(class, subclass, protocol uint8, label string, iface Interface) (Protocol, bool) {
	if iface.BulkIn == 0 || iface.BulkOut == 0 || iface.Interrupt == 0 {
		return 0, false
	}
	mtpLabel := strings.Contains(strings.ToUpper(label), "MTP")
	switch {
	case class == classStillImage && subclass == 1 && protocol == 1 && mtpLabel:
		return MTP, true
	case class == classStillImage && subclass == 1 && protocol == 1:
		return PTP, true
	case class == classVendor && mtpLabel:
		return MTP, true
	case class == classVendor && subclass == classVendor && protocol == 0:
		return Vendor, true
	}
	return 0, false
}

func readEndpoints(dir string) (Interface, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Interface{}, fmt.Errorf("usb: list %s: %w", dir, err)
	}
	var iface Interface
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "ep_") {
			continue
		}
		ep := filepath.Join(dir, e.Name())
		addr, err := readUint[uint8](ep, "bEndpointAddress", 16)
		if err != nil {
			return Interface{}, err
		}
		attr, err := readUint[uint8](ep, "bmAttributes", 16)
		if err != nil {
			return Interface{}, err
		}
		in := addr&directionIn != 0
		switch {
		case attr&attrTransfer == transferBulk && in && iface.BulkIn == 0:
			iface.BulkIn = addr
		case attr&attrTransfer == transferBulk && !in && iface.BulkOut == 0:
			iface.BulkOut = addr
		case attr&attrTransfer == transferIntr && in && iface.Interrupt == 0:
			iface.Interrupt = addr
		}
	}
	return iface, nil
}

func readDevice(dir string) (Device, error) {
	var d Device
	var err error
	if d.Bus, err = readUint[int](dir, "busnum", 10); err != nil {
		return Device{}, err
	}
	if d.Address, err = readUint[int](dir, "devnum", 10); err != nil {
		return Device{}, err
	}
	if d.VendorID, err = readUint[uint16](dir, "idVendor", 16); err != nil {
		return Device{}, err
	}
	if d.ProductID, err = readUint[uint16](dir, "idProduct", 16); err != nil {
		return Device{}, err
	}
	for _, f := range []struct {
		name  string
		field *string
	}{{"manufacturer", &d.Manufacturer}, {"product", &d.Product}, {"serial", &d.Serial}} {
		if *f.field, err = readOptional(dir, f.name); err != nil {
			return Device{}, err
		}
	}
	return d, nil
}

func readString(dir, name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // G304: a sysfs attribute of the device being read
	if err != nil {
		return "", fmt.Errorf("usb: read %s: %w", filepath.Join(dir, name), err)
	}
	return strings.TrimSpace(string(b)), nil
}

func readOptional(dir, name string) (string, error) {
	s, err := readString(dir, name)
	if errors.Is(err, fs.ErrNotExist) {
		if _, statErr := os.Stat(dir); statErr != nil {
			return "", err
		}
		return "", nil
	}
	return s, err
}

func readUint[T intconv.Integer](dir, name string, base int) (T, error) {
	s, err := readString(dir, name)
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseUint(s, base, 64)
	if err != nil {
		return 0, fmt.Errorf("usb: %s: %w", filepath.Join(dir, name), err)
	}
	n, err := intconv.Exact[T](v)
	if err != nil {
		return 0, fmt.Errorf("usb: %s: %w", filepath.Join(dir, name), err)
	}
	return n, nil
}
