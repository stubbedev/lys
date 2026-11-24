package edid

import (
	"encoding/hex"
	"strings"
	"testing"
)

const odysseyG5 = "00ffffffffffff004c2d807038574a30182001" +
	"04b54628783b43a5ae5244b0260f5054bfef80714f810081c081809500a9c0b3000101" +
	"98e200a0a0a0295008403500ba892100001a000000fd003090d6d63b010a202020202020" +
	"000000fc004c43333247357854" + "0a20202020000000ff00484e415436303033373" + "00a202001c4"

func base(t *testing.T) []byte {
	t.Helper()
	b, err := hex.DecodeString(odysseyG5)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func withChecksum(b []byte) []byte {
	var sum byte
	for _, c := range b[:127] {
		sum += c
	}
	b[127] = -sum
	return b
}

func TestParseARealMonitor(t *testing.T) {
	info, err := Parse(base(t))
	if err != nil {
		t.Fatal(err)
	}
	if want := (Info{PNP: "SAM", Make: "Samsung Electric Company", Model: "LC32G5xT", Serial: "HNAT600370"}); info != want {
		t.Fatalf("%+v, want %+v", info, want)
	}
	if d := info.Description(); d != "Samsung Electric Company LC32G5xT HNAT600370" {
		t.Fatalf("description %q", d)
	}
}

func TestAVendorIsNamedFromTheEmbeddedRegistry(t *testing.T) {
	if name, ok := Vendor("SAM"); !ok || name != "Samsung Electric Company" {
		t.Fatalf("SAM: %q, %v", name, ok)
	}
	if name, ok := Vendor("VQ@"); !ok || name != "Vision Quest" {
		t.Fatalf("VQ@: %q, %v", name, ok)
	}
	for _, id := range []string{"XQX", "inu", "", "SAMS"} {
		if name, ok := Vendor(id); ok {
			t.Errorf("%q is named %q", id, name)
		}
	}
}

func TestFallbacksAndRejection(t *testing.T) {
	b := base(t)
	unknown := append([]byte(nil), b[:128]...)
	unknown[8], unknown[9] = 0x62, 0x38
	unknown = withChecksum(unknown)
	info, err := Parse(unknown)
	if err != nil || info.PNP != "XQX" || info.Make != "XQX" {
		t.Fatalf("%+v, %v: an unknown vendor keeps its PNP ID", info, err)
	}
	stripped := append([]byte(nil), b[:128]...)
	for i := 54; i < 126; i++ {
		stripped[i] = 1
	}
	info, err = Parse(withChecksum(stripped))
	if err != nil {
		t.Fatal(err)
	}
	if info.Model != "0x7080" || info.Serial != "0x304A5738" || info.Description() != "Samsung Electric Company 0x7080 0x304A5738" {
		t.Fatalf("without descriptors: %+v", info)
	}
	bad := append([]byte(nil), b...)
	bad[20]++
	if _, err := Parse(bad); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("a bad checksum: %v", err)
	}
	if _, err := Parse(b[:64]); err == nil {
		t.Fatal("a short EDID was accepted")
	}
}
