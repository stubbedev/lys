package usershelper

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/settings/internal/accounts"
	"github.com/stubbedev/lys/settings/internal/pwfiles"
)

func (m machine) subIDFiles(t *testing.T, subuid, subgid string) {
	t.Helper()
	write(t, filepath.Join(m.etc, "subuid"), subuid, 0o644)
	write(t, filepath.Join(m.etc, "subgid"), subgid, 0o644)
}

func (m machine) addLoginDefs(t *testing.T, lines string) {
	t.Helper()
	write(t, filepath.Join(m.etc, loginDefsName), m.read(t, loginDefsName)+lines, 0o644)
}

func TestANewAccountGetsTheLowestFreeSubordinateIDs(t *testing.T) {
	m := newMachine(t)
	m.subIDFiles(t, "alice:100000:65536\n", "alice:100000:65536\nbob:165536:65536\n")
	if err := m.change(t.Context(), 0, accounts.Create{Login: "zoe", Hash: hash(t)}); err != nil {
		t.Fatal(err)
	}
	if got, want := m.read(t, "subuid"), "alice:100000:65536\nzoe:165536:65536\n"; got != want {
		t.Errorf("subuid %q, want %q", got, want)
	}
	if got, want := m.read(t, "subgid"), "alice:100000:65536\nbob:165536:65536\nzoe:231072:65536\n"; got != want {
		t.Errorf("subgid %q, want %q", got, want)
	}
}

func TestANewAccountGetsNoSubordinateIDsWhereUseraddGivesNone(t *testing.T) {
	const held = "zoe:100000:65536\n"
	for name, setup := range map[string]func(*testing.T, *machine){
		"no subuid or subgid": func(*testing.T, *machine) {},
		"a COUNT of 0": func(t *testing.T, m *machine) {
			m.subIDFiles(t, "", "")
			m.addLoginDefs(t, "SUB_UID_COUNT 0\nSUB_GID_COUNT 0\n")
		},
		"NixOS": func(t *testing.T, m *machine) {
			m.subIDFiles(t, "", "")
			m.nixos = &nixos{configured: true, mutable: true}
		},
		"an NSS module for them": func(t *testing.T, m *machine) {
			m.subIDFiles(t, "", "")
			write(t, filepath.Join(m.etc, nsswitchName), "subid: sss\n", 0o644)
		},
		"ranges the name holds already": func(t *testing.T, m *machine) { m.subIDFiles(t, held, held) },
		"ranges the uid holds already": func(t *testing.T, m *machine) {
			m.subIDFiles(t, id(me)+":100000:65536\n", id(me)+":100000:65536\n")
		},
	} {
		m := newMachine(t)
		setup(t, &m)
		before := map[string]string{}
		for _, f := range []string{"subuid", "subgid"} {
			if data, err := os.ReadFile(filepath.Join(m.etc, f)); err == nil {
				before[f] = string(data)
			}
		}
		if err := m.change(t.Context(), 0, accounts.Create{Login: "zoe", Hash: hash(t)}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, f := range []string{"subuid", "subgid"} {
			data, err := os.ReadFile(filepath.Join(m.etc, f))
			if _, existed := before[f]; !existed {
				if !errors.Is(err, os.ErrNotExist) {
					t.Errorf("%s: /etc/%s was created: %v", name, f, err)
				}
				continue
			}
			if string(data) != before[f] {
				t.Errorf("%s: /etc/%s %q, want %q", name, f, data, before[f])
			}
		}
	}
}

func TestDeterministicSubordinateIDsAreTheUIDsSlot(t *testing.T) {
	m := newMachine(t)
	m.subIDFiles(t, "alice:1000000:1000\n", "")
	m.addLoginDefs(t, "SUB_UID_DETERMINISTIC yes\nSUB_UID_MIN 1000000\nSUB_UID_COUNT 1000\nSUB_GID_DETERMINISTIC yes\nSUB_GID_MIN 2000000\nSUB_GID_COUNT 1000\nSUB_GID_STORE_BY_UID yes\n")
	if err := m.change(t.Context(), 0, accounts.Create{Login: "zoe", Hash: hash(t)}); err != nil {
		t.Fatal(err)
	}
	if got, want := m.read(t, "subuid"), "alice:1000000:1000\nzoe:1000000:1000\n"; got != want {
		t.Errorf("subuid, whose slot is the first whoever holds it: %q, want %q", got, want)
	}
	if got, want := m.read(t, "subgid"), id(me)+":2000000:1000\n"; got != want {
		t.Errorf("subgid, which names the uid: %q, want %q", got, want)
	}
}

func TestADeterministicRangeOutsideItsSlotsIsRefused(t *testing.T) {
	rule := subIDRule{ids: pwfiles.Range{Min: 100, Max: 299}, count: 100, deterministic: true}
	for _, c := range []struct {
		name string
		rule subIDRule
		uid  uint32
		want uint32
	}{
		{name: "the first slot", rule: rule, uid: 1000, want: 100},
		{name: "the last slot", rule: rule, uid: 1001, want: 200},
		{name: "wrapped", rule: subIDRule{ids: rule.ids, count: 100, deterministic: true, wrap: true}, uid: 1002, want: 100},
	} {
		if got, err := c.rule.start(pwfiles.DB{}, pwfiles.SubUIDs, 1000, c.uid); err != nil || got != c.want {
			t.Errorf("%s: %d, %v; want %d", c.name, got, err, c.want)
		}
	}
	if _, err := rule.start(pwfiles.DB{}, pwfiles.SubUIDs, 1000, 1002); !errors.Is(err, pwfiles.ErrExhausted) {
		t.Errorf("past the slots: %v, want %v", err, pwfiles.ErrExhausted)
	}
	for name, c := range map[string]struct {
		rule subIDRule
		uid  uint32
	}{
		"below UID_MIN":       {rule: rule, uid: 999},
		"an empty range":      {rule: subIDRule{ids: pwfiles.Range{Min: 300, Max: 100}, count: 1, deterministic: true}, uid: 1000},
		"a count past it all": {rule: subIDRule{ids: rule.ids, count: 201, deterministic: true}, uid: 1000},
	} {
		if _, err := c.rule.start(pwfiles.DB{}, pwfiles.SubUIDs, 1000, c.uid); err == nil || errors.Is(err, pwfiles.ErrExhausted) {
			t.Errorf("%s: %v, want a refusal", name, err)
		}
	}
}

func TestNoFreeSubordinateIDsCreateNoAccount(t *testing.T) {
	m := newMachine(t)
	m.subIDFiles(t, "alice:100000:65536\n", "")
	m.addLoginDefs(t, "SUB_UID_MAX 165535\n")
	before := m.snapshot(t)
	if err := m.change(t.Context(), 0, accounts.Create{Login: "zoe", Hash: hash(t)}); err == nil || err.Kind != accounts.ErrExhausted {
		t.Fatalf("no free subordinate IDs: %v, want %s", err, accounts.ErrExhausted)
	}
	if m.snapshot(t) != before || m.read(t, "subuid") != "alice:100000:65536\n" || m.read(t, "subgid") != "" {
		t.Errorf("the refused account changed the files:\n%s", m.snapshot(t))
	}
	if _, err := os.Stat(filepath.Join(m.home, "zoe")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the refused account left a home: %v", err)
	}
}

func TestADeletedAccountLosesItsSubordinateIDs(t *testing.T) {
	m := newMachine(t, entry("alice", me+1), entry("carol", me))
	write(t, filepath.Join(m.etc, "shadow"), "root:!:20000::::::\ncarol:!:20000::::::\n", 0o600)
	m.subIDFiles(t, "alice:100000:65536\ncarol:165536:65536\n"+id(me)+":231072:10\n", "carol:100000:65536\nalice:165536:65536\n")
	if err := m.change(t.Context(), me+1, accounts.Delete{Login: "carol"}); err != nil {
		t.Fatal(err)
	}
	if got := m.read(t, "subuid") + m.read(t, "subgid"); got != "alice:100000:65536\nalice:165536:65536\n" {
		t.Errorf("after carol: %q", got)
	}
}

func TestTheSubidLineOfNSSwitchNamesAModuleOrTheFiles(t *testing.T) {
	for text, want := range map[string]string{
		"":                              "",
		"passwd: files\n":               "",
		"subid: files\n":                "",
		"subid:\nsubid: sss files\n":    "sss",
		"SubID:\tsss\n":                 "sss",
		"# subid: sss\nsubid: files\n":  "",
		" subid: sss\n":                 "",
		"subid: sss\nsubid: files\n":    "sss",
		"subid: freeipa\tfiles\n":       "freeipa",
		"subidx: sss\n":                 "",
		"passwd: files\nsubid:  sss \n": "sss",
	} {
		if got := parseNSSwitchSubID(text); got != want {
			t.Errorf("%q: %q, want %q", text, got, want)
		}
	}
}
