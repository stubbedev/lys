package atomicfile

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

const writeFileAllocs = 5

func TestWriteFileReplacesContentAndMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "new" {
		t.Fatalf("read %q, %v", got, err)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", fi.Mode().Perm())
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("%d entries, want only the target", len(entries))
	}
}

func TestWriteStreamsTheProducersOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	err := Write(path, 0o640, func(w io.Writer) error {
		for _, s := range []string{"a", "b", "c"} {
			if _, err := io.WriteString(w, s); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "abc" {
		t.Errorf("read %q, want abc", got)
	}
}

func TestFailedWriteLeavesTheTargetAndNoTemporary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	err := Write(path, 0o600, func(w io.Writer) error {
		_, _ = w.Write([]byte("partial"))
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("write returned %v, want the producer's error", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "old" {
		t.Errorf("target became %q", got)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("%d entries left in the directory, want only the target", len(entries))
	}
}

func TestMissingDirectoryIsReported(t *testing.T) {
	dir := t.TempDir()
	if err := WriteFile(filepath.Join(dir, "absent", "f"), nil, 0o600); err == nil {
		t.Error("writing into a missing directory succeeded")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("%d entries created, want none", len(entries))
	}
}

func TestAWriteNeedsNoReadAccessToTheDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root opens a directory without read permission")
	}
	dir := filepath.Join(t.TempDir(), "d")
	if err := os.Mkdir(dir, 0o300); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	path := filepath.Join(dir, "f")
	if err := WriteFile(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("write into a write-only directory: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Errorf("target is %q, want the renamed content", got)
	}
}

func TestAFailedRenameLeavesTheTargetAndNoTemporary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := os.MkdirAll(filepath.Join(path, "inside"), 0o700); err != nil {
		t.Fatal(err)
	}
	err := WriteFile(path, []byte("new"), 0o600)
	if !errors.Is(err, syscall.EISDIR) || !strings.Contains(err.Error(), path) {
		t.Fatalf("replace a directory: %v, want EISDIR naming %s", err, path)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("%d entries left in the directory, want only the target", len(entries))
	}
}

func TestWriteFileAllocatesOnlyTheNamesItPassesTheKernel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	data := []byte("content")
	if allocs := testing.AllocsPerRun(5, func() {
		if err := WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}); allocs > writeFileAllocs {
		t.Fatalf("WriteFile allocates %v, want at most %d: the temporary's name and the C strings of its open and rename", allocs, writeFileAllocs)
	}
}

func TestWriteSeekablePatchesWhatItWrote(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	err := WriteSeekable(path, 0o640, func(w io.WriteSeeker) error {
		if _, err := io.WriteString(w, "xxxxbody"); err != nil {
			return err
		}
		if _, err := w.Seek(0, io.SeekStart); err != nil {
			return err
		}
		_, err := io.WriteString(w, "head")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "headbody" {
		t.Errorf("read %q, want headbody", got)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o640 {
		t.Errorf("mode %v (%v), want 0640", info.Mode().Perm(), err)
	}
}

func TestFailedWriteSeekableLeavesNoTemporary(t *testing.T) {
	dir := t.TempDir()
	boom := errors.New("boom")
	err := WriteSeekable(filepath.Join(dir, "f"), 0o600, func(w io.WriteSeeker) error {
		_, _ = io.WriteString(w, "partial")
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("write returned %v, want the producer's error", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("%d entries left in the directory, want none", len(entries))
	}
}

func TestATemporaryIsKnownForItsTargetOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.yaml")
	var seen []string
	err := Write(path, 0o600, func(io.Writer) error {
		entries, err := os.ReadDir(dir)
		for _, e := range entries {
			seen = append(seen, filepath.Join(dir, e.Name()))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || !IsTemporary(path, seen[0]) {
		t.Fatalf("the write's temporaries %v are not all known as temporaries of %s", seen, path)
	}
	for _, other := range []string{path, seen[0] + "x", filepath.Join(dir, ".runtime.yaml.tmp"), filepath.Join(dir, ".other.tmp1"), filepath.Join(t.TempDir(), filepath.Base(seen[0]))} {
		if IsTemporary(path, other) {
			t.Errorf("%s is taken for a temporary of %s", other, path)
		}
	}
}

func TestADurableWriteSyncsTheDirectoryOnceAfterTheRename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	var synced []string
	counting := func(d string) error {
		if got, err := os.ReadFile(path); err != nil || string(got) != "new" {
			t.Errorf("the directory synced before the rename: %q, %v", got, err)
		}
		synced = append(synced, d)
		return syncDir(d)
	}
	if err := replace(path, permOnly(0o600), counting, writeData([]byte("new"))); err != nil {
		t.Fatal(err)
	}
	if len(synced) != 1 || synced[0] != dir {
		t.Fatalf("a durable write synced %v, want %s once", synced, dir)
	}
}

func TestADurableWriteReportsTheDirectorySyncItCouldNotMakeAndAPlainOneOpensNoDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root opens a directory without read permission")
	}
	dir := filepath.Join(t.TempDir(), "d")
	if err := os.Mkdir(dir, 0o300); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	path := filepath.Join(dir, "f")
	err := WriteFileDurable(path, []byte("new"), 0o600)
	if !errors.Is(err, fs.ErrPermission) || !strings.Contains(err.Error(), "sync the directory") {
		t.Fatalf("a durable write into an unreadable directory returned %v, want the directory sync's error", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Errorf("target is %q, want the renamed content", got)
	}
	if err := WriteFile(path, []byte("plain"), 0o600); err != nil {
		t.Fatalf("a plain write into the same directory: %v", err)
	}
}

func ownershipFixture(t *testing.T, perm os.FileMode) (string, *os.File) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "shadow")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, perm); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return path, f
}

func statOf(t *testing.T, path string) *syscall.Stat_t {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("%s has no stat", path)
	}
	return st
}

func TestAnOwnedWriteKeepsTheOwnerGroupAndModeItRead(t *testing.T) {
	path, f := ownershipFixture(t, 0o640)
	gid := os.Getgid()
	if groups, err := os.Getgroups(); err == nil {
		for _, g := range groups {
			if g != gid {
				gid = g
				break
			}
		}
	}
	if err := f.Chown(os.Getuid(), gid); err != nil {
		t.Fatal(err)
	}
	own, err := OwnershipOf(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteFileOwned(path, []byte("new"), own); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Fatalf("read %q, want new", got)
	}
	st := statOf(t, path)
	if int(st.Uid) != os.Getuid() || int(st.Gid) != gid || os.FileMode(st.Mode).Perm() != 0o640 {
		t.Errorf("replacement is %d:%d %v, want %d:%d 0640", st.Uid, st.Gid, os.FileMode(st.Mode).Perm(), os.Getuid(), gid)
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("%d entries, want only the target", len(entries))
	}
}

func TestAnOwnedWriteGivesTheFileToAnotherUserAsRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("only root may give a file to another user")
	}
	path, f := ownershipFixture(t, 0o640)
	if err := f.Chown(65534, 65534); err != nil {
		t.Fatal(err)
	}
	own, err := OwnershipOf(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteFileOwned(path, []byte("new"), own); err != nil {
		t.Fatal(err)
	}
	if st := statOf(t, path); st.Uid != 65534 || st.Gid != 65534 {
		t.Errorf("replacement is %d:%d, want 65534:65534", st.Uid, st.Gid)
	}
}

func TestAnOwnedWriteWithoutThePrivilegeLeavesTheTargetAndNoTemporary(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root may give a file to any user")
	}
	path, f := ownershipFixture(t, 0o640)
	own, err := OwnershipOf(f)
	if err != nil {
		t.Fatal(err)
	}
	own.uid = 0
	err = WriteFileOwned(path, []byte("new"), own)
	if !errors.Is(err, fs.ErrPermission) || !strings.Contains(err.Error(), "fchown") || !strings.Contains(err.Error(), path) {
		t.Fatalf("give a file to root unprivileged: %v, want fchown's EPERM naming %s", err, path)
	}
	if got, _ := os.ReadFile(path); string(got) != "old" {
		t.Errorf("target became %q", got)
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("%d entries left in the directory, want only the target", len(entries))
	}
}

func TestAnOwnedWriteCarriesTheAttributesItRead(t *testing.T) {
	const name = "user.lys-label"
	path, f := ownershipFixture(t, 0o600)
	if err := unix.Fsetxattr(int(f.Fd()), name, []byte("system_u:object_r:shadow_t:s0"), 0); errors.Is(err, unix.ENOTSUP) {
		t.Skipf("the file system of %s holds no user attributes", path)
	} else if err != nil {
		t.Fatal(err)
	}
	own, err := ownershipOf(f, name, "user.lys-absent")
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteFileOwned(path, []byte("new"), own); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	n, err := unix.Getxattr(path, name, buf)
	if err != nil || string(buf[:n]) != "system_u:object_r:shadow_t:s0" {
		t.Errorf("the replacement's %s is %q, %v", name, buf[:max(n, 0)], err)
	}
	if _, err := unix.Getxattr(path, "user.lys-absent", buf); !errors.Is(err, unix.ENODATA) {
		t.Errorf("an attribute the original lacked: %v, want ENODATA", err)
	}
}

func TestAnAttributeTheWriterMayNotSetLeavesTheTargetAndNoTemporary(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root may set trusted attributes")
	}
	path, f := ownershipFixture(t, 0o600)
	own, err := OwnershipOf(f)
	if err != nil {
		t.Fatal(err)
	}
	own.attrs = append(own.attrs, xattr{name: "trusted.lys-label", value: []byte("x")})
	err = WriteFileOwned(path, []byte("new"), own)
	if !errors.Is(err, fs.ErrPermission) || !strings.Contains(err.Error(), "trusted.lys-label") {
		t.Fatalf("set a trusted attribute unprivileged: %v, want EPERM naming it", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "old" {
		t.Errorf("target became %q", got)
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("%d entries left in the directory, want only the target", len(entries))
	}
}

func TestAnOwnedWriteRefusesAnOwnershipItDidNotRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shadow")
	if err := WriteFileOwned(path, []byte("new"), Ownership{}); !errors.Is(err, ErrNoOwnership) || !strings.Contains(err.Error(), path) {
		t.Fatalf("the zero Ownership: %v, want %v naming %s", err, ErrNoOwnership, path)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("%d entries created, want none", len(entries))
	}
}

func TestTheOwnershipOfAFileWithoutALabelHasNone(t *testing.T) {
	_, f := ownershipFixture(t, 0o644)
	own, err := ownershipOf(f, "user.lys-absent")
	if err != nil {
		t.Fatal(err)
	}
	if len(own.attrs) != 0 || own.perm != 0o644 || !own.owned {
		t.Errorf("ownership %+v, want 0644 with no attributes", own)
	}
}
