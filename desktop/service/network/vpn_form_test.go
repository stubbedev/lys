package network

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func exampleKind() VPNKind {
	return VPNKind{ID: "org.example.vpn", Label: "Example", Fields: []VPNField{
		{Key: "gateway", Label: "Gateway", Required: true, Format: FormatHost},
		{Key: "protocol", Label: "Protocol", Choices: []VPNChoice{{Value: "gp", Label: "GlobalProtect"}}},
		{Key: "password", Label: "Password", Secret: true},
	}}
}

func TestTheRawEditorIsOneKeyAndValuePerLine(t *testing.T) {
	got := ParseRawValues("gateway = vpn.example.com\n  protocol=gp  \n\n# a comment = not a key\nno-equals\n = orphan\ntoken = abc=def==\n")
	want := map[string]string{"gateway": "vpn.example.com", "protocol": "gp", "token": "abc=def=="}
	if !maps.Equal(got, want) {
		t.Errorf("parse = %v, want %v (comments, junk and keyless lines ignored)", got, want)
	}
	if back := ParseRawValues(RenderRawValues(want)); !maps.Equal(back, want) {
		t.Errorf("round trip = %v", back)
	}
	if RenderRawValues(map[string]string{"b": "2", "a": "1"}) != "a = 1\nb = 2" {
		t.Error("render order is not stable")
	}
	if len(ParseRawValues("")) != 0 || RenderRawValues(nil) != "" {
		t.Error("nothing does not read and render as nothing")
	}
}

func TestTypedValuesWinOverRawOnesAndEmptiedOnesClear(t *testing.T) {
	raw := map[string]string{"gateway": "raw.example.com", "extra": "x", "blank": "  ", "cleared": "old"}
	got := MergeVPNValues(raw, []TypedValue{{"gateway", "typed.example.com"}, {"cleared", ""}})
	want := map[string]string{"gateway": "typed.example.com", "extra": "x"}
	if !maps.Equal(got, want) {
		t.Errorf("merge = %v, want %v (typed wins, emptied clears, blank dropped)", got, want)
	}
	if raw["gateway"] != "raw.example.com" {
		t.Error("merge wrote into the raw map")
	}
	if got := MergeVPNValues(nil, []TypedValue{{"gateway", "g"}}); !maps.Equal(got, map[string]string{"gateway": "g"}) {
		t.Errorf("merge without raw keys = %v", got)
	}
}

func TestLeftoversAreOnlyTheKeysTheFormHasNoFieldFor(t *testing.T) {
	kind := exampleKind()
	left := kind.Leftovers(map[string]string{"gateway": "g", "protocol": "gp", "mtu": "1400"})
	if !maps.Equal(left, map[string]string{"mtu": "1400"}) {
		t.Errorf("leftovers = %v, want only the key the form has no field for", left)
	}
	if len(kind.Leftovers(map[string]string{"gateway": "g"})) != 0 {
		t.Error("a profile that fits its form left raw keys")
	}
	if got := (VPNKind{ID: "raw"}).Leftovers(map[string]string{"a": "1"}); !maps.Equal(got, map[string]string{"a": "1"}) {
		t.Errorf("an untyped kind kept %v of its values in the raw editor", got)
	}
}

func problemKeys(problems []VPNProblem) (missing, malformed []string) {
	for _, p := range problems {
		switch p.Fault {
		case FaultMissing:
			missing = append(missing, p.Field.Key)
		case FaultMalformed:
			malformed = append(malformed, p.Field.Key)
		}
	}
	return missing, malformed
}

func TestACheckNamesEveryMissingFieldThenEveryMalformedOne(t *testing.T) {
	wg := wireGuardKind()
	missing, malformed := problemKeys(wg.Check(map[string]string{"interface": "wg0", "peer-keepalive": "soon"}))
	if !slices.Equal(missing, []string{"private-key", "address", "peer-public-key", "peer-endpoint", "peer-allowed-ips"}) || !slices.Equal(malformed, []string{"peer-keepalive"}) {
		t.Errorf("missing %v, malformed %v, want every problem at once, in drawing order", missing, malformed)
	}
	if p := wg.Check(map[string]string{"interface": "wg0", "peer-keepalive": "soon"}); p[0].Fault != FaultMissing || p[len(p)-1].Fault != FaultMalformed {
		t.Errorf("problems %v do not put the missing first", p)
	}
	key := GenerateKeyPair()
	good := map[string]string{
		"interface": "wg0", "private-key": key.Private, "address": "10.0.0.2/24",
		"peer-public-key": key.Public, "peer-endpoint": "vpn.example.com:51820", "peer-allowed-ips": "0.0.0.0/0",
	}
	if p := wg.Check(good); len(p) != 0 {
		t.Errorf("a complete, well-formed profile was refused: %v", p)
	}
	bad := maps.Clone(good)
	bad["peer-endpoint"] = "https://vpn.example.com"
	bad["private-key"] = "   "
	missing, malformed = problemKeys(wg.Check(bad))
	if !slices.Equal(missing, []string{"private-key"}) || !slices.Equal(malformed, []string{"peer-endpoint"}) {
		t.Errorf("missing %v, malformed %v: a blank value is missing, a wrong one malformed", missing, malformed)
	}
	if p := (VPNKind{ID: "raw"}).Check(map[string]string{"anything": "goes"}); len(p) != 0 {
		t.Errorf("an untyped kind refused %v", p)
	}
}

func TestKindsWithKeepsAProfilesUninstalledKind(t *testing.T) {
	kinds := []VPNKind{wireGuardKind(), exampleKind()}
	if len(KindsWith(kinds, "org.example.vpn")) != len(kinds) || len(KindsWith(kinds, "")) != len(kinds) {
		t.Error("a known kind or none was added again")
	}
	got := KindsWith(kinds, "org.gone.vpn")
	if len(got) != len(kinds)+1 || got[len(kinds)].ID != "org.gone.vpn" || got[len(kinds)].IsTyped() {
		t.Errorf("uninstalled kind = %+v, want it appended, untyped", got)
	}
	if len(kinds) != 2 {
		t.Error("KindsWith changed its input")
	}
}

func TestAWgQuickFileIsReadNamedAfterItself(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "office.conf")
	if err := os.WriteFile(conf, []byte(wgConf), 0o600); err != nil {
		t.Fatal(err)
	}
	values, err := ReadWgQuick(conf)
	if err != nil || values["interface"] != "office" || values["peer-endpoint"] != "vpn.example.com:51820" {
		t.Fatalf("read %v, %v", values, err)
	}
	junk := filepath.Join(dir, "notes.conf")
	if err := os.WriteFile(junk, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadWgQuick(junk); !errors.Is(err, ErrNotWgQuick) {
		t.Errorf("a file that is no wg-quick config read with %v", err)
	}
	if _, err := ReadWgQuick(filepath.Join(dir, "gone.conf")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a missing file read with %v", err)
	}
}
