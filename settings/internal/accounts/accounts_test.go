package accounts

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/useraccount"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const passwd = `root:x:0:0:root:/root:/bin/bash
alice:x:1000:1000:Alice Liddell,,,:/home/alice:/bin/zsh
bob:x:2000:2000::/home/bob:/bin/bash
carol:x:1002:10:Carol:/home/carol:/bin/bash
svc:x:998:998:service:/var/lib/svc:/usr/sbin/nologin
`

const group = `root:x:0:
wheel:x:10:alice
sudo:x:27:bob
alice:x:1000:
`

func TestTheDatabaseKnowsPeopleAndAdministrators(t *testing.T) {
	db := ParseDatabase(passwd, group, useraccount.DefaultUIDRange)
	var got []string
	for _, a := range db.People() {
		got = append(got, a.Name+"="+a.Kind.String())
	}
	if want := []string{"alice=admin", "bob=standard", "carol=admin"}; !slices.Equal(got, want) {
		t.Errorf("people = %v, want %v (wheel before sudo, primary group counts)", got, want)
	}
	if a, ok := db.ByUID(1000); !ok || a.FullName != "Alice Liddell" || a.Kind != Admin {
		t.Errorf("uid 1000 = %+v, %v", a, ok)
	}
	if a, ok := db.ByName("svc"); !ok || db.IsPerson(a) {
		t.Errorf("svc = %+v, %v; want a system account", a, ok)
	}
	if _, ok := db.ByUID(4242); ok {
		t.Error("an unknown uid was found")
	}
	if g, err := db.AdminGroup(); err != nil || g.Name != "wheel" {
		t.Errorf("admin group = %+v, %v", g, err)
	}
}

func TestWithoutAnAdminGroupNobodyIsAnAdministrator(t *testing.T) {
	db := ParseDatabase(passwd, "root:x:0:\nusers:x:100:alice,bob\n", useraccount.DefaultUIDRange)
	if _, err := db.AdminGroup(); !errors.Is(err, ErrNoAdminGroup) {
		t.Errorf("admin group: %v, want %v", err, ErrNoAdminGroup)
	}
	for _, a := range db.People() {
		if a.Kind != Standard {
			t.Errorf("%s is %s", a.Name, a.Kind)
		}
	}
	db = ParseDatabase(passwd, "sudo:x:27:alice,bob\n", useraccount.DefaultUIDRange)
	if a, _ := db.ByName("bob"); a.Kind != Admin {
		t.Errorf("bob in sudo is %s", a.Kind)
	}
}

func TestParseGroupsSkipsMalformedLines(t *testing.T) {
	got := ParseGroups("wheel:x:10:a,b\nbroken\nnogid:x:z:\n:x:1:\nempty:x:5:\n")
	if len(got) != 2 || !slices.Equal(got[0].Members, []string{"a", "b"}) || got[1].Name != "empty" || got[1].Members != nil {
		t.Errorf("groups = %+v", got)
	}
}

func TestALoginNameIsWhatUseraddTakes(t *testing.T) {
	for _, in := range []string{"zoe", "_build", "a-b_c9", strings.Repeat("a", MaxLogin)} {
		if got, err := ParseLogin(in); err != nil || string(got) != in {
			t.Errorf("%q: %q %v", in, got, err)
		}
	}
	for in, want := range map[string]error{
		"":                              ErrLoginEmpty,
		strings.Repeat("a", MaxLogin+1): ErrLoginLong,
		"9lives":                        ErrLoginStart,
		"-x":                            ErrLoginStart,
		"Zoe":                           ErrLoginStart,
		"zo e":                          ErrLoginChars,
		"zoë":                           ErrLoginChars,
		"zoe.smith":                     ErrLoginChars,
		"zoe:x":                         ErrLoginChars,
	} {
		if _, err := ParseLogin(in); !errors.Is(err, want) {
			t.Errorf("%q: %v, want %v", in, err, want)
		}
	}
}

func TestALoginNameIsMadeFromTheFullName(t *testing.T) {
	for in, want := range map[string]string{
		"Zoë Smith":              "zoesmith",
		"  Ana-María  López":     "ana-marialopez",
		"42 Douglas":             "douglas",
		"Ølaf":                   "laf",
		"日本":                     "",
		strings.Repeat("Ab", 40): strings.Repeat("ab", MaxLogin/2),
	} {
		got := DeriveLogin(in)
		if got != want {
			t.Errorf("%q made %q, want %q", in, got, want)
		}
		if got != "" {
			if _, err := ParseLogin(got); err != nil {
				t.Errorf("%q made %q, which is no login name: %v", in, got, err)
			}
		}
	}
}

func TestAnAccountsNameNeverReadsAsAFlagAPathOrAField(t *testing.T) {
	for _, in := range []string{"bob", "john.doe", "Admin", "zoë", "user@realm"} {
		if got, err := ParseName(in); err != nil || string(got) != in {
			t.Errorf("%q: %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "-r", "../etc", "a:b", "a b", "a\nroot", strings.Repeat("a", MaxName+1), "\xff"} {
		if _, err := ParseName(in); !errors.Is(err, ErrName) {
			t.Errorf("%q: %v, want %v", in, err, ErrName)
		}
	}
}

func TestAFullNameIsWhatTheGECOSFieldHolds(t *testing.T) {
	if got, err := ParseRealName("  Zoë Smith-Jones "); err != nil || got != "Zoë Smith-Jones" {
		t.Errorf("%q, %v", got, err)
	}
	for _, in := range []string{"a:b", "Smith, Zoe", "line\nbreak", "tab\there", strings.Repeat("x", MaxRealName+1), "\xff"} {
		if _, err := ParseRealName(in); !errors.Is(err, ErrRealName) {
			t.Errorf("%q: %v, want %v", in, err, ErrRealName)
		}
	}
}

func png8(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestAPictureIsABoundedPNGEncodedAfresh(t *testing.T) {
	good := png8(t, 8, 8)
	trailing := append(slices.Clone(good), "#!/bin/sh"...)
	p, err := ParsePicture(trailing)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(p, []byte("#!/bin/sh")) {
		t.Error("what followed the PNG was kept")
	}
	for name, data := range map[string][]byte{
		"not a picture": []byte("GIF89a"),
		"too wide":      png8(t, MaxPictureSide+1, 1),
		"too big":       append(slices.Clone(good), make([]byte, MaxPictureBytes)...),
	} {
		if _, err := ParsePicture(data); !errors.Is(err, ErrPicture) {
			t.Errorf("%s: %v, want %v", name, err, ErrPicture)
		}
	}
}

func roundTrip(t *testing.T, r Request) Request {
	t.Helper()
	data, err := Encode(r)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(data)
	if err != nil {
		t.Fatalf("%T: %v", r, err)
	}
	return got
}

func TestEveryRequestDecodesToWhatWasEncoded(t *testing.T) {
	hash := HashPassword([]byte("pw"))
	name, admin := RealName("Zoë"), Admin
	pic, err := ParsePicture(png8(t, 4, 4))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []Request{
		Rename{RealName: "Alice Liddell"},
		Rename{},
		SetPicture{Picture: pic},
		Create{Login: "zoe", RealName: "Zoë", Kind: Admin, Hash: hash},
		Edit{Login: "bob", RealName: &name, Kind: &admin, Hash: &hash},
		Edit{Login: "bob"},
		Delete{Login: "bob", RemoveHome: true},
	} {
		got := roundTrip(t, r)
		if !sameRequest(got, r) {
			t.Errorf("%#v came back as %#v", r, got)
		}
	}
}

func sameRequest(a, b Request) bool {
	x, _ := Encode(a)
	y, _ := Encode(b)
	return bytes.Equal(x, y)
}

func TestARequestWithAnInvalidValueIsRefused(t *testing.T) {
	for name, raw := range map[string]string{
		"unknown op":       `{"op":"chown","request":{}}`,
		"unknown field":    `{"op":"delete","request":{"login":"bob","uid":0}}`,
		"bad login":        `{"op":"delete","request":{"login":"../etc"}}`,
		"flag login":       `{"op":"edit","request":{"login":"-r"}}`,
		"strict new login": `{"op":"create","request":{"login":"Zoe","realName":"","kind":"standard","hash":"x"}}`,
		"no login":         `{"op":"delete","request":{}}`,
		"colon in name":    `{"op":"edit","request":{"login":"bob","realName":"x:0:0"}}`,
		"untrimmed name":   `{"op":"edit","request":{"login":"bob","realName":" Bob"}}`,
		"bad kind":         `{"op":"edit","request":{"login":"bob","kind":"root"}}`,
		"bad hash":         `{"op":"edit","request":{"login":"bob","hash":"bob:$6$x\nroot:"}}`,
		"no hash":          `{"op":"create","request":{"login":"zoe","realName":"","kind":"standard"}}`,
		"trailing value":   `{"op":"delete","request":{"login":"bob"}}{}`,
		"no picture":       `{"op":"picture","request":{}}`,
		"picture not PNG":  `{"op":"picture","request":{"picture":"R0lGODlh"}}`,
		"envelope field":   `{"op":"delete","request":{"login":"bob"},"as":"root"}`,
		"not JSON":         `delete bob`,
		"a trailing NUL":   "{\"op\":\"delete\",\"request\":{\"login\":\"bob\"}}\x00",
	} {
		if _, err := Decode([]byte(raw)); err == nil {
			t.Errorf("%s: %s was taken", name, raw)
		}
	}
}

func TestEachClassHasItsOwnAction(t *testing.T) {
	if Self.Action() == Others.Action() || !strings.HasPrefix(Self.Action(), "dev.stubbe.lys.settings.") {
		t.Errorf("actions %q %q", Self.Action(), Others.Action())
	}
}

func TestAHashNeverPrints(t *testing.T) {
	h := HashPassword([]byte("pw"))
	if s := Errorf(ErrFailed, "%v", h).Error(); strings.Contains(s, string(h)) {
		t.Errorf("the hash reached %q", s)
	}
}

func TestEveryErrorKindTravelsAsItsOwnBusError(t *testing.T) {
	seen := map[string]ErrorKind{}
	for _, k := range ErrorKinds {
		name := BusErrorName(k)
		if other, dup := seen[name]; dup {
			t.Errorf("%s and %s are both %s", k, other, name)
		}
		seen[name] = k
		if strings.ContainsAny(strings.TrimPrefix(name, BusName+".Error."), ".-") {
			t.Errorf("%s travels as %s, not one D-Bus name element", k, name)
		}
		e := Errorf(k, "said %d", 1)
		reply := *e.BusError()
		got, ok := fromBusError(reply)
		if !ok || *got != *e {
			t.Errorf("%s came back as %v, %v", k, got, ok)
		}
	}
	if BusErrorName(ErrUnknownUser) != "dev.stubbe.lys.Users1.Error.UnknownUser" {
		t.Errorf("unknown-user is %s", BusErrorName(ErrUnknownUser))
	}
	for name, reply := range map[string]dbus.Error{
		"another service's error":  {Name: "org.freedesktop.DBus.Error.AccessDenied", Body: []any{"no"}},
		"no detail":                {Name: BusErrorName(ErrTaken)},
		"a detail of another type": {Name: BusErrorName(ErrTaken), Body: []any{uint32(1)}},
	} {
		if got, ok := fromBusError(reply); ok {
			t.Errorf("%s read as %v", name, got)
		}
	}
}
