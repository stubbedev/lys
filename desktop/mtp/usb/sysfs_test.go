package usb

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestScanFindsCamerasPhonesAndVendorCandidates(t *testing.T) {
	t.Parallel()
	got, err := Scan("testdata/devices")
	if err != nil {
		t.Fatal(err)
	}
	ep := Interface{BulkIn: 0x81, BulkOut: 0x01, Interrupt: 0x82}
	want := []Device{
		{Name: "1-2", Bus: 1, Address: 5, VendorID: 0x04a9, ProductID: 0x3218, Manufacturer: "Canon Inc.", Product: "Canon Digital Camera", Protocol: PTP, Interface: Interface{BulkIn: 0x81, BulkOut: 0x02, Interrupt: 0x83}},
		{Name: "1-3", Bus: 1, Address: 7, VendorID: 0x18d1, ProductID: 0x4ee1, Manufacturer: "Google", Product: "Pixel 7", Serial: "28131FDH2000AB", Protocol: MTP, Interface: ep},
		{Name: "1-5", Bus: 1, Address: 11, VendorID: 0x2717, ProductID: 0xff40, Protocol: Vendor, Interface: ep},
		{Name: "2-1", Bus: 2, Address: 3, VendorID: 0x04e8, ProductID: 0x6860, Manufacturer: "SAMSUNG", Product: "SAMSUNG_Android", Serial: "R58M12345", Protocol: MTP, Interface: ep},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Scan found\n%+v\nwant\n%+v", got, want)
	}
	if node := got[0].Node(); node != "/dev/bus/usb/001/005" {
		t.Fatalf("the camera's node is %s", node)
	}
}

func copyTree(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS("testdata/devices")); err != nil {
		t.Fatal(err)
	}
	return dst
}

func names(devices []Device) []string {
	out := make([]string, len(devices))
	for i, d := range devices {
		out[i] = d.Name
	}
	return out
}

func TestScanLeavesOutADeviceUnpluggedWhileRead(t *testing.T) {
	t.Parallel()
	root := copyTree(t)
	if err := os.RemoveAll(filepath.Join(root, "1-3")); err != nil {
		t.Fatal(err)
	}
	got, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if n := names(got); !slices.Equal(n, []string{"1-2", "1-5", "2-1"}) {
		t.Fatalf("Scan found %v after 1-3 went away", n)
	}
}

func TestScanRefusesAMalformedAttribute(t *testing.T) {
	t.Parallel()
	root := copyTree(t)
	if err := os.WriteFile(filepath.Join(root, "1-2:1.0", "bInterfaceClass"), []byte("zz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(root); err == nil || !strings.Contains(err.Error(), "bInterfaceClass") {
		t.Fatalf("Scan of a malformed class answered %v", err)
	}
}

func TestScanFailsOnAMissingRoot(t *testing.T) {
	t.Parallel()
	if _, err := Scan(filepath.Join(t.TempDir(), "none")); err == nil {
		t.Fatal("Scan of a missing directory succeeded")
	}
}

func TestClassifyNeedsAllThreeEndpoints(t *testing.T) {
	t.Parallel()
	full := Interface{BulkIn: 0x81, BulkOut: 0x01, Interrupt: 0x82}
	cases := []struct {
		name                      string
		class, subclass, protocol uint8
		label                     string
		iface                     Interface
		want                      Protocol
		ok                        bool
	}{
		{"still image", 6, 1, 1, "", full, PTP, true},
		{"still image named MTP", 6, 1, 1, "mtp", full, MTP, true},
		{"vendor named MTP", 0xff, 0xff, 0, "MTP", full, MTP, true},
		{"unnamed vendor", 0xff, 0xff, 0, "", full, Vendor, true},
		{"adb", 0xff, 0x42, 1, "ADB Interface", full, 0, false},
		{"no interrupt", 6, 1, 1, "", Interface{BulkIn: 0x81, BulkOut: 0x01}, 0, false},
		{"hid", 3, 1, 1, "", full, 0, false},
	}
	for _, c := range cases {
		got, ok := classify(c.class, c.subclass, c.protocol, c.label, c.iface)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: classify = %v, %t; want %v, %t", c.name, got, ok, c.want, c.ok)
		}
	}
}
