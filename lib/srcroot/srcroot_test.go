package srcroot

import (
	"os"
	"path/filepath"
	"testing"
)

func tree(t *testing.T, goMod string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "ui", "internal", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if goMod != "" {
		if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestAComponentIsTheDirectoryBelowTheModuleRoot(t *testing.T) {
	root := tree(t, "module example.com/lys\n\ngo 1.27\n")
	dir, path, err := Component(filepath.Join(root, "ui", "internal", "x"))
	if err != nil || dir != filepath.Join(root, "ui") || path != "example.com/lys/ui" {
		t.Fatalf("Component = %q, %q, %v", dir, path, err)
	}
	moduleRoot, module, err := Module(filepath.Join(root, "ui"))
	if err != nil || moduleRoot != root || module != "example.com/lys" {
		t.Fatalf("Module = %q, %q, %v", moduleRoot, module, err)
	}
}

func TestTheModuleRootIsInNoComponent(t *testing.T) {
	root := tree(t, "module example.com/lys\n")
	if _, _, err := Component(root); err == nil {
		t.Fatal("the module root was taken for a component")
	}
}

func TestAGoModWithoutAModuleLineIsAnError(t *testing.T) {
	root := tree(t, "go 1.27\n")
	if _, _, err := Module(filepath.Join(root, "ui")); err == nil {
		t.Fatal("a go.mod without a module line was accepted")
	}
}

func TestThisPackageIsInLib(t *testing.T) {
	dir, path, err := Component(".")
	if err != nil || filepath.Base(dir) != "lib" || path != "github.com/stubbedev/lys/lib" {
		t.Fatalf("Component(.) = %q, %q, %v", dir, path, err)
	}
}
