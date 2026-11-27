package pwfiles

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/atomicfile"
)

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
	if err != nil || len(entries) != 5 {
		t.Fatalf("the directory holds %v: %v, want the four files and shadow's backup", entries, err)
	}
}

func TestACommitBacksUpEachChangedFileAsItWasRead(t *testing.T) {
	dir := etc(t)
	then := time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC)
	shadow := filepath.Join(dir, "shadow")
	if err := os.Chtimes(shadow, then, then); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shadow+"-", []byte("an older backup\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.SetPassword("bob", "$6$n$h", 20001); err != nil {
		t.Fatal(err)
	}
	if err := f.Commit(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(shadow + "-")
	if err != nil || string(data) != shadowText {
		t.Fatalf("shadow- is %q, %v, want shadow as it was read", data, err)
	}
	backup, err := os.Stat(shadow + "-")
	if err != nil {
		t.Fatal(err)
	}
	st, ok := backup.Sys().(*syscall.Stat_t)
	if !ok || backup.Mode().Perm() != 0o640 || int(st.Uid) != os.Getuid() || int(st.Gid) != os.Getgid() {
		t.Errorf("shadow- is %v, want shadow's 0640 and owner", backup)
	}
	if !backup.ModTime().Equal(then) || time.Unix(st.Atim.Unix()).UTC() != then {
		t.Errorf("shadow- has times %v and %v, want shadow's %v", backup.ModTime(), time.Unix(st.Atim.Unix()), then)
	}
	for _, name := range []string{"passwd-", "group-", "gshadow-"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("an unchanged file was backed up to %s: %v", name, err)
		}
	}
}

func TestAFailedBackupLeavesItsFileAndPutsBackTheOthers(t *testing.T) {
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
	var replaced []string
	err = f.commit(func(path string, data []byte, own atomicfile.Ownership) error {
		if filepath.Base(path) == "group-" {
			return broken
		}
		replaced = append(replaced, filepath.Base(path))
		return atomicfile.WriteFileOwned(path, data, own)
	})
	if !errors.Is(err, broken) || !strings.Contains(err.Error(), "back up "+filepath.Join(dir, "group")) {
		t.Fatalf("got %v, want the backup's error", err)
	}
	if want := []string{"shadow-", "shadow", "gshadow-", "gshadow", "shadow", "gshadow"}; !slices.Equal(replaced, want) {
		t.Errorf("wrote %v, want %v: shadow and gshadow backed up, replaced and put back, group and passwd never replaced", replaced, want)
	}
	for name, want := range map[string]string{"shadow": shadowText, "shadow-": shadowText, "gshadow": gshadowText, "gshadow-": gshadowText, "group": groupText, "passwd": passwdText} {
		if data, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(data) != want {
			t.Errorf("%s after the failed commit: %q, %v", name, data, err)
		}
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
	err = f.commit(func(path string, data []byte, own atomicfile.Ownership) error {
		if filepath.Base(path) == "passwd" {
			return broken
		}
		return atomicfile.WriteFileOwned(path, data, own)
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
