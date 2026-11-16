package lyswmtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/lib/buildtest"
)

func TestBinariesBuildsLysWMAndLysWMCtlOncePerProcess(t *testing.T) {
	first := Binaries(t)
	before := map[string]os.FileInfo{}
	for _, name := range commands {
		info, err := os.Stat(filepath.Join(first, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("%s in %s is not an executable file (mode %s)", name, first, info.Mode())
		}
		before[name] = info
	}
	again := Binaries(t)
	if again != first {
		t.Fatalf("the second Binaries = %s, want the first build's %s", again, first)
	}
	for _, name := range commands {
		after, err := os.Stat(filepath.Join(again, name))
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(before[name], after) || !after.ModTime().Equal(before[name].ModTime()) {
			t.Errorf("the second Binaries built %s again: %v after %v", name, after.ModTime(), before[name].ModTime())
		}
	}
}

func TestTheBuiltDirectoryOutlivesTheNextHoldOfItsFamily(t *testing.T) {
	built := Binaries(t)
	if _, err := buildtest.Hold(family); err != nil {
		t.Fatalf("Hold(%s): %v", family, err)
	}
	for _, name := range commands {
		if _, err := os.Stat(filepath.Join(built, name)); err != nil {
			t.Errorf("the next Hold of %s removed %s of this running process: %v", family, name, err)
		}
	}
}

func TestOnPathRunsTheBuiltLysWMByName(t *testing.T) {
	t.Setenv("PATH", t.TempDir()+string(os.PathListSeparator)+os.Getenv("PATH"))
	OnPath(t)
	for _, name := range commands {
		got, err := exec.LookPath(name)
		if err != nil {
			t.Fatalf("LookPath(%s) after OnPath: %v", name, err)
		}
		if want := filepath.Join(Binaries(t), name); got != want {
			t.Errorf("LookPath(%s) = %s, want the built %s", name, got, want)
		}
	}
}
