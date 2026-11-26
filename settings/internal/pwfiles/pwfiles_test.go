package pwfiles

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
)

func fixture(t *testing.T) DB {
	t.Helper()
	db, err := parse([numFiles][]byte{Passwd: []byte(passwdText), Shadow: []byte(shadowText), Group: []byte(groupText), Gshadow: []byte(gshadowText)})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func none(uint32) (bool, error) { return false, nil }

func TestAnUnchangedFileIsWrittenAsItWasRead(t *testing.T) {
	db := fixture(t)
	for f, want := range map[File]string{Passwd: passwdText, Shadow: shadowText, Group: groupText, Gshadow: gshadowText} {
		if got := db.Text(f); got != want || db.Changed(f) {
			t.Errorf("%s: %q (changed %v), want %q", f.Name(), got, db.Changed(f), want)
		}
	}
}

func TestAMalformedLineRefusesTheWholeFile(t *testing.T) {
	for name, texts := range map[string][numFiles][]byte{
		"a short passwd line": {Passwd: []byte("root:x:0:0:root:/root\n"), Shadow: []byte(shadowText), Group: []byte(groupText)},
		"a long group line":   {Passwd: []byte(passwdText), Shadow: []byte(shadowText), Group: []byte("wheel:x:10:alice:bob\n")},
		"a nameless shadow":   {Passwd: []byte(passwdText), Shadow: []byte(":x:1:2:3:4:5:6:7\n"), Group: []byte(groupText)},
	} {
		if _, err := parse(texts); !errors.Is(err, ErrMalformed) {
			t.Errorf("%s: %v, want %v", name, err, ErrMalformed)
		}
	}
	if _, err := parse([numFiles][]byte{Passwd: []byte(passwdText), Group: []byte(groupText)}); err == nil || !strings.Contains(err.Error(), "/etc/shadow does not exist") {
		t.Errorf("no shadow: %v", err)
	}
	if db, err := parse([numFiles][]byte{Passwd: []byte(passwdText), Shadow: []byte(shadowText), Group: []byte(groupText)}); err != nil || db.Text(Gshadow) != "" {
		t.Errorf("no gshadow, which may be missing: %v", err)
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
	} {
		if got := db.Text(f); got != want {
			t.Errorf("%s: %q, want %q", f.Name(), got, want)
		}
	}
	shared := fixture(t)
	if err := shared.DeleteUser("bob"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shared.Text(Group), "users:x:100:\n") {
		t.Errorf("bob's primary group, not his own, went: %q", shared.Text(Group))
	}
	if err := shared.DeleteUser("bob"); !errors.Is(err, ErrNoEntry) {
		t.Errorf("deleting bob twice: %v", err)
	}
}

func etc(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, c := range map[string]struct {
		text string
		mode os.FileMode
	}{"passwd": {passwdText, 0o644}, "shadow": {shadowText, 0o640}, "group": {groupText, 0o644}, "gshadow": {gshadowText, 0o600}} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(c.text), c.mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, c.mode); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestACommitReplacesOnlyTheChangedFilesKeepingTheirModes(t *testing.T) {
	dir := etc(t)
	f, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(filepath.Join(dir, "group"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.SetPassword("bob", "$6$n$h", 20001); err != nil {
		t.Fatal(err)
	}
	if err := f.Commit(); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(dir, "shadow"))
	if err != nil || st.Mode().Perm() != 0o640 {
		t.Fatalf("shadow after the commit: %v, %v", st, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "shadow"))
	if err != nil || !strings.Contains(string(data), "bob:$6$n$h:20001:") {
		t.Fatalf("shadow %q, %v", data, err)
	}
	after, err := os.Stat(filepath.Join(dir, "group"))
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("the unchanged group was replaced")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 4 {
		t.Fatalf("the directory holds %v: %v", entries, err)
	}
}

func TestAFailedCommitPutsBackWhatItHadWritten(t *testing.T) {
	dir := etc(t)
	f, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.AddGroup("zoe", 1002); err != nil {
		t.Fatal(err)
	}
	if err := f.AddUser(NewUser{Name: "zoe", UID: 1002, GID: 1002}); err != nil {
		t.Fatal(err)
	}
	broken := errors.New("disk full")
	err = f.commit(func(path string, data []byte, m meta) error {
		if filepath.Base(path) == "passwd" {
			return broken
		}
		return replace(path, data, m)
	})
	if !errors.Is(err, broken) {
		t.Fatalf("got %v", err)
	}
	for name, want := range map[string]string{"passwd": passwdText, "shadow": shadowText, "group": groupText, "gshadow": gshadowText} {
		if data, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(data) != want {
			t.Errorf("%s after the failed commit: %q, %v", name, data, err)
		}
	}
}

func TestTheLockExcludesAnotherHolderUntilItIsReleased(t *testing.T) {
	dir := t.TempDir()
	held, err := TakeLock(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*lockRetry)
	defer cancel()
	if l, err := TakeLock(ctx, dir); !errors.Is(err, ErrLocked) {
		if l != nil {
			_ = l.Unlock()
		}
		t.Fatalf("a second holder: %v, want %v", err, ErrLocked)
	}
	got := make(chan error, 1)
	go func() {
		l, err := TakeLock(t.Context(), dir)
		if err == nil {
			err = l.Unlock()
		}
		got <- err
	}()
	if err := held.Unlock(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-got:
		if err != nil {
			t.Fatalf("after the release: %v", err)
		}
	case <-time.After(time.Minute):
		t.Fatal("the lock stayed held after its release")
	}
	if st, err := os.Stat(filepath.Join(dir, LockName)); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("the lock file: %v, %v", st, err)
	}
}
