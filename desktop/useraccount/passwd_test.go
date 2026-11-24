package useraccount

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/logtest"
)

func TestParseUIDRange(t *testing.T) {
	for _, c := range []struct {
		name, text string
		want       UIDRange
	}{
		{"unset", "# nothing here\nUMASK 022\n", DefaultUIDRange},
		{"set", "UID_MIN\t\t 500\nUID_MAX 29999 # trailing\n", UIDRange{500, 29999}},
		{"commented", "#UID_MIN 2000\nUID_MAX 3000\n", UIDRange{1000, 3000}},
		{"hex", "UID_MIN 0x3e8\nUID_MAX 0x7530\n", UIDRange{1000, 30000}},
	} {
		got, err := ParseUIDRange(c.text)
		if err != nil || got != c.want {
			t.Errorf("%s: %+v, %v; want %+v", c.name, got, err, c.want)
		}
	}
}

func TestAMalformedUIDRangeIsRefused(t *testing.T) {
	for _, text := range []string{"UID_MIN lots\n", "UID_MAX -1\n", "UID_MIN 5000\nUID_MAX 4000\n", "UID_MAX 99999999999\n"} {
		got, err := ParseUIDRange(text)
		if err == nil || got != DefaultUIDRange {
			t.Errorf("%q: %+v, %v; want the default and an error", text, got, err)
		}
	}
}

func TestAMissingOrUnreadableLoginDefsGivesTheDefaults(t *testing.T) {
	dir := t.TempDir()
	if got := ReadUIDRange(filepath.Join(dir, "absent")); got != DefaultUIDRange {
		t.Errorf("missing: %+v", got)
	}
	logs := logtest.Capture(t)
	if got := ReadUIDRange(dir); got != DefaultUIDRange {
		t.Errorf("a directory: %+v", got)
	}
	bad := filepath.Join(dir, "login.defs")
	if err := os.WriteFile(bad, []byte("UID_MIN x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ReadUIDRange(bad); got != DefaultUIDRange {
		t.Errorf("malformed: %+v", got)
	}
	if n := strings.Count(logs.String(), "shadow-utils' default"); n != 2 {
		t.Errorf("log = %q, want both failures said", logs.String())
	}
	good := filepath.Join(dir, "good.defs")
	if err := os.WriteFile(good, []byte("UID_MAX 2000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ReadUIDRange(good); got != (UIDRange{1000, 2000}) {
		t.Errorf("good: %+v", got)
	}
}

const passwd = `root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
alice:x:1000:100:Alice Wonder,,,:/home/alice:/bin/zsh
bob:x:1001:1001::/home/bob:/bin/bash
svc:x:998:998:service:/var/lib/svc:/usr/sbin/nologin
nobody:x:65534:65534:nobody:/nonexistent:/usr/sbin/nologin
carol:x:1002:1002:Carol:/home/carol:/bin/false
dave:x:5000:5000:Dave:/home/dave:/bin/sh
eve:x:1003:1003:Eve:/home/eve:
garbage
mallory:x:lots:1:Mallory:/home/mallory:/bin/sh
:x:1004:1004::/:/bin/sh
`

func TestParsePasswd(t *testing.T) {
	got := ParsePasswd(passwd)
	if len(got) != 9 {
		t.Fatalf("entries = %+v, want the nine well-formed ones", got)
	}
	if got[2] != (Entry{Name: "alice", UID: 1000, GID: 100, FullName: "Alice Wonder", Home: "/home/alice", Shell: "/bin/zsh"}) || got[2].DisplayName() != "Alice Wonder" {
		t.Errorf("alice = %+v", got[2])
	}
	if got[3].FullName != "" || got[3].DisplayName() != "bob" {
		t.Errorf("bob's names = %q, %q; want none and the login name", got[3].FullName, got[3].DisplayName())
	}
}

func TestPeopleAreInTheRangeWithALoginShell(t *testing.T) {
	names := func(es []Entry) []string {
		out := make([]string, len(es))
		for i, e := range es {
			out[i] = e.Name
		}
		return out
	}
	if got, want := names(People(passwd, DefaultUIDRange)), []string{"alice", "bob", "dave"}; !slices.Equal(got, want) {
		t.Errorf("people = %v, want %v", got, want)
	}
	if got, want := names(People(passwd, UIDRange{1000, 4999})), []string{"alice", "bob"}; !slices.Equal(got, want) {
		t.Errorf("people under 5000 = %v, want %v", got, want)
	}
}
