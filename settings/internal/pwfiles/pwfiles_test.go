package pwfiles

import (
	"errors"
	"strings"
	"testing"
)

const (
	passwdText = "root:x:0:0:root:/root:/bin/bash\n" +
		"alice:x:1000:1000:Alice,,,:/home/alice:/bin/bash\n" +
		"\n" +
		"+::::::\n" +
		"bob:x:1001:100:Bob:/home/bob:/bin/sh\n"
	shadowText = "root:!:19000:0:99999:7:::\n" +
		"alice:$6$a$b:19000:0:99999:7:::\n" +
		"bob:!:19000::::::\n"
	groupText = "root:x:0:\n" +
		"wheel:x:10:alice,bob\n" +
		"users:x:100:\n" +
		"alice:x:1000:\n"
	gshadowText = "root:::\n" +
		"wheel:::alice,bob\n" +
		"users:::\n" +
		"alice:!::\n"
	subuidText = "alice:100000:65536\n" +
		"bob:165536:65536\n" +
		"1000:280000:10\n"
	subgidText = "alice:100000:65536\n" +
		"bob:165536:65536\n"
)

func fixture(t *testing.T) DB {
	t.Helper()
	db, err := parse([numFiles][]byte{Passwd: []byte(passwdText), Shadow: []byte(shadowText), Group: []byte(groupText), Gshadow: []byte(gshadowText), Subuid: []byte(subuidText), Subgid: []byte(subgidText)})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func none(uint32) (bool, error) { return false, nil }

func TestAnUnchangedFileIsWrittenAsItWasRead(t *testing.T) {
	db := fixture(t)
	for f, want := range map[File]string{Passwd: passwdText, Shadow: shadowText, Group: groupText, Gshadow: gshadowText, Subuid: subuidText, Subgid: subgidText} {
		if got := db.Text(f); got != want || db.Changed(f) {
			t.Errorf("%s: %q (changed %v), want %q", f.Name(), got, db.Changed(f), want)
		}
	}
}

func TestAMalformedLineRefusesTheWholeFile(t *testing.T) {
	for name, texts := range map[string][numFiles][]byte{
		"a short passwd line":  {Passwd: []byte("root:x:0:0:root:/root\n"), Shadow: []byte(shadowText), Group: []byte(groupText)},
		"a long group line":    {Passwd: []byte(passwdText), Shadow: []byte(shadowText), Group: []byte("wheel:x:10:alice:bob\n")},
		"a nameless shadow":    {Passwd: []byte(passwdText), Shadow: []byte(":x:1:2:3:4:5:6:7\n"), Group: []byte(groupText)},
		"a long subuid line":   {Passwd: []byte(passwdText), Shadow: []byte(shadowText), Group: []byte(groupText), Subuid: []byte("alice:1:2:3\n")},
		"a subuid start":       {Passwd: []byte(passwdText), Shadow: []byte(shadowText), Group: []byte(groupText), Subuid: []byte("alice:x:2\n")},
		"a negative count":     {Passwd: []byte(passwdText), Shadow: []byte(shadowText), Group: []byte(groupText), Subgid: []byte("alice:1:-2\n")},
		"a count past 32 bits": {Passwd: []byte(passwdText), Shadow: []byte(shadowText), Group: []byte(groupText), Subgid: []byte("alice:1:4294967296\n")},
	} {
		if _, err := parse(texts); !errors.Is(err, ErrMalformed) {
			t.Errorf("%s: %v, want %v", name, err, ErrMalformed)
		}
	}
	if _, err := parse([numFiles][]byte{Passwd: []byte(passwdText), Group: []byte(groupText)}); err == nil || !strings.Contains(err.Error(), "/etc/shadow does not exist") {
		t.Errorf("no shadow: %v", err)
	}
	db, err := parse([numFiles][]byte{Passwd: []byte(passwdText), Shadow: []byte(shadowText), Group: []byte(groupText)})
	if err != nil || db.Text(Gshadow) != "" {
		t.Errorf("no gshadow, which may be missing: %v", err)
	}
	for _, f := range []File{Gshadow, Subuid, Subgid} {
		if db.Exists(f) {
			t.Errorf("%s exists without a text", f.Name())
		}
	}
	if !db.Exists(Passwd) {
		t.Errorf("passwd does not exist")
	}
}

func TestAnAddedAccountIsInPasswdAndShadowAndItsGroupInGroupAndGshadow(t *testing.T) {
	db := fixture(t)
	if err := db.AddGroup("zoe", 1002); err != nil {
		t.Fatal(err)
	}
	if err := db.AddUser(NewUser{Name: "zoe", UID: 1002, GID: 1002, GECOS: "Zoë", Home: "/home/zoe", Shell: "/bin/bash", Hash: "$6$s$h", LastChange: 20000, Max: "99999", Warn: "7"}); err != nil {
		t.Fatal(err)
	}
	if err := db.AddMember("wheel", "zoe"); err != nil {
		t.Fatal(err)
	}
	for f, want := range map[File]string{
		Passwd:  passwdText + "zoe:x:1002:1002:Zoë:/home/zoe:/bin/bash\n",
		Shadow:  shadowText + "zoe:$6$s$h:20000::99999:7:::\n",
		Group:   strings.Replace(groupText, "alice,bob\n", "alice,bob,zoe\n", 1) + "zoe:x:1002:\n",
		Gshadow: strings.Replace(gshadowText, "alice,bob\n", "alice,bob,zoe\n", 1) + "zoe:!::\n",
	} {
		if got := db.Text(f); got != want || !db.Changed(f) {
			t.Errorf("%s: %q, want %q", f.Name(), got, want)
		}
	}
}

func TestANameOrIDAlreadyHeldIsTaken(t *testing.T) {
	db := fixture(t)
	for name, err := range map[string]error{
		"user name":  db.AddUser(NewUser{Name: "bob", UID: 5000}),
		"user ID":    db.AddUser(NewUser{Name: "zoe", UID: 1001}),
		"group name": db.AddGroup("users", 5000),
		"group ID":   db.AddGroup("zoe", 10),
	} {
		if !errors.Is(err, ErrTaken) {
			t.Errorf("%s: %v, want %v", name, err, ErrTaken)
		}
	}
	shadowOnly, err := parse([numFiles][]byte{Passwd: []byte(passwdText), Shadow: []byte(shadowText + "ghost:!:1::::::\n"), Group: []byte(groupText)})
	if err != nil {
		t.Fatal(err)
	}
	if err := shadowOnly.AddUser(NewUser{Name: "ghost", UID: 5000}); !errors.Is(err, ErrTaken) {
		t.Errorf("a name only shadow holds: %v", err)
	}
	for _, f := range []File{Passwd, Shadow, Group, Gshadow} {
		if db.Changed(f) {
			t.Errorf("a refused edit changed %s", f.Name())
		}
	}
}

func TestANewIDIsOneAboveTheHighestThenTheLowestFree(t *testing.T) {
	db := fixture(t)
	for _, c := range []struct {
		r     Range
		taken map[uint32]bool
		want  uint32
	}{
		{Range{1000, 60000}, nil, 1002},
		{Range{1000, 60000}, map[uint32]bool{1002: true, 1003: true}, 1004},
		{Range{1000, 1001}, nil, 0},
		{Range{999, 1001}, nil, 999},
		{Range{2000, 3000}, nil, 2000},
	} {
		got, err := db.FreeUID(c.r, func(id uint32) (bool, error) { return c.taken[id], nil })
		if c.want == 0 {
			if !errors.Is(err, ErrExhausted) {
				t.Errorf("%v: %d, %v, want %v", c.r, got, err, ErrExhausted)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%v with %v taken: %d, %v, want %d", c.r, c.taken, got, err, c.want)
		}
	}
	if got, err := db.FreeGID(Range{1000, 60000}, 1002, none); err != nil || got != 1002 {
		t.Errorf("a free preferred group ID: %d, %v", got, err)
	}
	if got, err := db.FreeGID(Range{1000, 60000}, 100, none); err != nil || got != 1001 {
		t.Errorf("a taken preferred group ID: %d, %v, want 1001", got, err)
	}
	broken := errors.New("userdb is down")
	if _, err := db.FreeUID(Range{1000, 60000}, func(uint32) (bool, error) { return false, broken }); !errors.Is(err, broken) {
		t.Errorf("a failed lookup: %v", err)
	}
}

func TestGECOSAndPasswordAreSetInTheirFields(t *testing.T) {
	db := fixture(t)
	if err := db.SetGECOS("alice", "Alice Liddell"); err != nil {
		t.Fatal(err)
	}
	if err := db.SetPassword("bob", "$6$n$h", 20001); err != nil {
		t.Fatal(err)
	}
	if got, want := db.Text(Passwd), strings.Replace(passwdText, "Alice,,,", "Alice Liddell", 1); got != want {
		t.Errorf("passwd %q, want %q", got, want)
	}
	if got, want := db.Text(Shadow), strings.Replace(shadowText, "bob:!:19000:", "bob:$6$n$h:20001:", 1); got != want {
		t.Errorf("shadow %q, want %q", got, want)
	}
	if db.Changed(Group) || db.Changed(Gshadow) {
		t.Error("the groups changed")
	}
	for name, err := range map[string]error{
		"gecos":    db.SetGECOS("zoe", "x"),
		"password": db.SetPassword("zoe", "$6$n$h", 1),
	} {
		if !errors.Is(err, ErrNoEntry) {
			t.Errorf("%s of nobody: %v", name, err)
		}
	}
}

func TestAMemberIsRemovedFromGroupAndGshadowAndAMembershipIsNeverDoubled(t *testing.T) {
	db := fixture(t)
	if err := db.RemoveMember("wheel", "bob"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(db.Text(Group), "wheel:x:10:alice\n") || !strings.Contains(db.Text(Gshadow), "wheel:::alice\n") {
		t.Fatalf("group %q gshadow %q", db.Text(Group), db.Text(Gshadow))
	}
	for name, err := range map[string]error{
		"remove a non-member": db.RemoveMember("wheel", "bob"),
		"add a member":        db.AddMember("wheel", "alice"),
	} {
		if !errors.Is(err, ErrMember) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := db.AddMember("nogroup", "bob"); !errors.Is(err, ErrNoEntry) {
		t.Errorf("no such group: %v", err)
	}
}

func TestADeletedAccountLeavesEveryFileWithItsOwnGroup(t *testing.T) {
	db := fixture(t)
	if err := db.DeleteUser("alice"); err != nil {
		t.Fatal(err)
	}
	for f, want := range map[File]string{
		Passwd:  strings.Replace(passwdText, "alice:x:1000:1000:Alice,,,:/home/alice:/bin/bash\n", "", 1),
		Shadow:  strings.Replace(shadowText, "alice:$6$a$b:19000:0:99999:7:::\n", "", 1),
		Group:   "root:x:0:\nwheel:x:10:bob\nusers:x:100:\n",
		Gshadow: "root:::\nwheel:::bob\nusers:::\n",
		Subuid:  "bob:165536:65536\n",
		Subgid:  "bob:165536:65536\n",
	} {
		if got := db.Text(f); got != want {
			t.Errorf("%s: %q, want %q", f.Name(), got, want)
		}
	}
	shared := fixture(t)
	if err := shared.DeleteUser("bob"); err != nil {
		t.Fatal(err)
	}
	if shared.Text(Subuid) != "alice:100000:65536\n1000:280000:10\n" || shared.Text(Subgid) != "alice:100000:65536\n" {
		t.Errorf("bob's deletion took another's ranges: %q, %q", shared.Text(Subuid), shared.Text(Subgid))
	}
	if !strings.Contains(shared.Text(Group), "users:x:100:\n") {
		t.Errorf("bob's primary group, not his own, went: %q", shared.Text(Group))
	}
	if err := shared.DeleteUser("bob"); !errors.Is(err, ErrNoEntry) {
		t.Errorf("deleting bob twice: %v", err)
	}
}

func TestANewAccountsSubordinateIDsAreTheLowestFreeRangeOfTheCount(t *testing.T) {
	db := fixture(t)
	for _, c := range []struct {
		name  string
		s     SubIDs
		r     Range
		count uint32
		want  uint32
	}{
		{name: "after every range", s: SubUIDs, r: Range{Min: 100000, Max: 600100000}, count: 65536, want: 280010},
		{name: "the hole before the last", s: SubUIDs, r: Range{Min: 100000, Max: 600100000}, count: 10, want: 231072},
		{name: "below every range", s: SubUIDs, r: Range{Min: 1000, Max: 600100000}, count: 99000, want: 1000},
		{name: "one ID too many for below", s: SubUIDs, r: Range{Min: 1000, Max: 600100000}, count: 99001, want: 280010},
		{name: "in subgid alone", s: SubGIDs, r: Range{Min: 100000, Max: 600100000}, count: 65536, want: 231072},
		{name: "the range's whole tail", s: SubGIDs, r: Range{Min: 100000, Max: 231072 + 9}, count: 10, want: 231072},
		{name: "a start inside a range", s: SubGIDs, r: Range{Min: 150000, Max: 600100000}, count: 1, want: 231072},
		{name: "the top of 32 bits", s: SubGIDs, r: Range{Min: 4294967295, Max: 4294967295}, count: 1, want: 4294967295},
	} {
		if got, err := db.FreeSubIDs(c.s, c.r, c.count); err != nil || got != c.want {
			t.Errorf("%s: %d, %v; want %d", c.name, got, err, c.want)
		}
	}
	if _, err := db.FreeSubIDs(SubUIDs, Range{Min: 100000, Max: 231071 + 9}, 10); !errors.Is(err, ErrExhausted) {
		t.Errorf("no hole of 10: %v, want %v", err, ErrExhausted)
	}
	for name, c := range map[string]struct {
		r     Range
		count uint32
	}{
		"no IDs":             {r: Range{Min: 1, Max: 10}, count: 0},
		"an empty range":     {r: Range{Min: 10, Max: 1}, count: 1},
		"more than it holds": {r: Range{Min: 1, Max: 10}, count: 11},
	} {
		if _, err := db.FreeSubIDs(SubUIDs, c.r, c.count); err == nil || errors.Is(err, ErrExhausted) {
			t.Errorf("%s: %v, want a refused request", name, err)
		}
	}
}

func TestSubordinateIDsAreAnAccountsByNameOrUserID(t *testing.T) {
	db := fixture(t)
	for _, c := range []struct {
		s    SubIDs
		name string
		uid  uint32
		want bool
	}{
		{SubUIDs, "bob", 1001, true},
		{SubUIDs, "nobody-here", 1000, true},
		{SubGIDs, "nobody-here", 1000, false},
		{SubGIDs, "zoe", 1002, false},
	} {
		if got := db.HasSubIDs(c.s, c.name, c.uid); got != c.want {
			t.Errorf("HasSubIDs(%d, %s, %d) = %v, want %v", c.s, c.name, c.uid, got, c.want)
		}
	}
	if err := db.AddSubIDs(SubGIDs, "zoe", 231072, 65536); err != nil {
		t.Fatal(err)
	}
	if got, want := db.Text(Subgid), subgidText+"zoe:231072:65536\n"; got != want || !db.Changed(Subgid) || db.Changed(Subuid) {
		t.Errorf("subgid: %q, want %q", got, want)
	}
	absent, err := parse([numFiles][]byte{Passwd: []byte(passwdText), Shadow: []byte(shadowText), Group: []byte(groupText)})
	if err != nil {
		t.Fatal(err)
	}
	if err := absent.AddSubIDs(SubUIDs, "zoe", 100000, 65536); !errors.Is(err, ErrNoEntry) || absent.Changed(Subuid) {
		t.Errorf("no subuid: %v, want %v", err, ErrNoEntry)
	}
}
