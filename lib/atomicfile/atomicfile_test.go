package atomicfile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
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
