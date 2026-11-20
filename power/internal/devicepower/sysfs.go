package devicepower

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/stubbedev/lys/lib/intconv"
)

type Sysfs struct {
	Root string
}

func (s Sysfs) path(parts ...string) string {
	return filepath.Join(append([]string{s.Root}, parts...)...)
}

func (s Sysfs) Read() (Inventory, error) {
	var inv Inventory
	var err error
	if inv.PCI, err = s.pci(); err != nil {
		return Inventory{}, err
	}
	if inv.USB, err = s.usb(); err != nil {
		return Inventory{}, err
	}
	if inv.ASPM, err = s.optional("module/pcie_aspm/parameters/policy", parseASPM); err != nil {
		return Inventory{}, err
	}
	if inv.Audio, err = s.optional("module/snd_hda_intel/parameters/power_save", plain); err != nil {
		return Inventory{}, err
	}
	if inv.SATA, err = s.sata(); err != nil {
		return Inventory{}, err
	}
	if inv.NVMe, err = s.nvme(); err != nil {
		return Inventory{}, err
	}
	return inv, nil
}

func (s Sysfs) entries(dir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", dir, err)
	}
	return entries, nil
}

func (s Sysfs) pci() ([]PCIDevice, error) {
	dir := s.path("bus/pci/devices")
	entries, err := s.entries(dir)
	if err != nil {
		return nil, err
	}
	devices := make([]PCIDevice, 0, len(entries))
	for _, e := range entries {
		dev := filepath.Join(dir, e.Name())
		vendor, err := readHex[uint16](filepath.Join(dev, "vendor"))
		if err != nil {
			return nil, err
		}
		product, err := readHex[uint16](filepath.Join(dev, "device"))
		if err != nil {
			return nil, err
		}
		class, err := readHex[uint32](filepath.Join(dev, "class"))
		if err != nil {
			return nil, err
		}
		driver, err := boundDriver(filepath.Join(dev, "driver"))
		if err != nil {
			return nil, err
		}
		control, err := readAttr(filepath.Join(dev, "power/control"), plain)
		if err != nil {
			return nil, err
		}
		devices = append(devices, PCIDevice{
			Address: e.Name(),
			ID:      DeviceID{Vendor: vendor, Product: product},
			Class:   class,
			Driver:  driver,
			Control: control,
		})
	}
	return devices, nil
}

func boundDriver(link string) (string, error) {
	target, err := os.Readlink(link)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading the driver link %s: %w", link, err)
	}
	return filepath.Base(target), nil
}

func (s Sysfs) usb() ([]USBDevice, error) {
	dir := s.path("bus/usb/devices")
	entries, err := s.entries(dir)
	if err != nil {
		return nil, err
	}
	var devices []USBDevice
	interfaces := map[string][]uint8{}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if device, _, isInterface := strings.Cut(e.Name(), ":"); isInterface {
			class, err := readHex[uint8](filepath.Join(path, "bInterfaceClass"))
			if err != nil {
				return nil, err
			}
			interfaces[device] = append(interfaces[device], class)
			continue
		}
		vendor, err := readHex[uint16](filepath.Join(path, "idVendor"))
		if err != nil {
			return nil, err
		}
		product, err := readHex[uint16](filepath.Join(path, "idProduct"))
		if err != nil {
			return nil, err
		}
		class, err := readHex[uint8](filepath.Join(path, "bDeviceClass"))
		if err != nil {
			return nil, err
		}
		control, err := readAttr(filepath.Join(path, "power/control"), plain)
		if err != nil {
			return nil, err
		}
		devices = append(devices, USBDevice{
			Name:    e.Name(),
			ID:      DeviceID{Vendor: vendor, Product: product},
			Class:   class,
			Control: control,
		})
	}
	for i := range devices {
		devices[i].Interfaces = interfaces[devices[i].Name]
	}
	return devices, nil
}

func (s Sysfs) sata() ([]SATAHost, error) {
	dir := s.path("class/scsi_host")
	entries, err := s.entries(dir)
	if err != nil {
		return nil, err
	}
	var hosts []SATAHost
	for _, e := range entries {
		attrs, err := s.optional(filepath.Join("class/scsi_host", e.Name(), "link_power_management_policy"), plain)
		if err != nil {
			return nil, err
		}
		for _, a := range attrs {
			hosts = append(hosts, SATAHost{Name: e.Name(), Policy: a})
		}
	}
	return hosts, nil
}

func (s Sysfs) nvme() ([]NVMeController, error) {
	dir := s.path("class/nvme")
	entries, err := s.entries(dir)
	if err != nil {
		return nil, err
	}
	controllers := make([]NVMeController, 0, len(entries))
	for _, e := range entries {
		attrs, err := s.optional(filepath.Join("class/nvme", e.Name(), "power/pm_qos_latency_tolerance_us"), plain)
		if err != nil {
			return nil, err
		}
		apst := APSTUnsupported
		for _, a := range attrs {
			apst = APSTEnabled
			if a.Value == "0" {
				apst = APSTDisabled
			}
		}
		controllers = append(controllers, NVMeController{Name: e.Name(), APST: apst})
	}
	return controllers, nil
}

func (s Sysfs) optional(rel string, parse func(string) (Value, error)) ([]Attr, error) {
	a, err := readAttr(s.path(rel), parse)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return []Attr{a}, nil
}

func plain(raw string) (Value, error) { return Value(raw), nil }

func parseASPM(raw string) (Value, error) {
	for field := range strings.FieldsSeq(raw) {
		if inner, ok := strings.CutPrefix(field, "["); ok {
			if v, ok := strings.CutSuffix(inner, "]"); ok {
				return Value(v), nil
			}
		}
	}
	return "", fmt.Errorf("no [selected] policy in %q", raw)
}

func readAttr(path string, parse func(string) (Value, error)) (Attr, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Attr{}, fmt.Errorf("reading %s: %w", path, err)
	}
	v, err := parse(strings.TrimSpace(string(raw)))
	if err != nil {
		return Attr{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return Attr{Path: path, Value: v}, nil
}

func readHex[T uint8 | uint16 | uint32](path string) (T, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", path, err)
	}
	text := strings.TrimPrefix(strings.TrimSpace(string(raw)), "0x")
	wide, err := strconv.ParseUint(text, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing %s %q as hex: %w", path, text, err)
	}
	v, err := intconv.Exact[T](wide)
	if err != nil {
		return 0, fmt.Errorf("parsing %s %q: %w", path, text, err)
	}
	return v, nil
}

var ErrRefused = errors.New("the kernel refused the value")

func writeAttr(path string, v Value) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	_, werr := f.Write([]byte(v))
	return classify(path, v, errors.Join(werr, f.Close()))
}

func classify(path string, v Value, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, syscall.EPERM):
		return fmt.Errorf("writing %q to %s: %w: %w", v, path, ErrRefused, err)
	default:
		return fmt.Errorf("writing %q to %s: %w", v, path, err)
	}
}
