package console

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func vtConsole(t *testing.T, dir, entry, name, bind string) string {
	t.Helper()
	con := filepath.Join(dir, entry)
	if err := os.MkdirAll(con, 0o755); err != nil {
		t.Fatal(err)
	}
	for file, content := range map[string]string{"name": name + "\n", "bind": bind + "\n"} {
		if err := os.WriteFile(filepath.Join(con, file), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(con, "bind")
}

func bindOf(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

func TestUnbindingTheFramebufferConsolesLeavesTheOthersBound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dummy := vtConsole(t, dir, "vtcon0", "(S) dummy device", "1")
	fbcon := vtConsole(t, dir, "vtcon1", "(M) frame buffer device", "1")
	idle := vtConsole(t, dir, "vtcon2", "(S) frame buffer device", "0")
	got, err := UnbindFramebufferConsoles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"vtcon1"}) {
		t.Errorf("unbound %q, want only the bound frame buffer console vtcon1", got)
	}
	for path, want := range map[string]string{dummy: "1", fbcon: "0", idle: "0"} {
		if b := bindOf(t, path); b != want {
			t.Errorf("%s reads %q, want %q", path, b, want)
		}
	}
	if again, err := UnbindFramebufferConsoles(dir); err != nil || len(again) != 0 {
		t.Errorf("unbinding again unbound %q with %v, want nothing", again, err)
	}
}

func TestUnbindingFramebufferConsolesWithoutVTsDoesNothing(t *testing.T) {
	t.Parallel()
	if got, err := UnbindFramebufferConsoles(filepath.Join(t.TempDir(), "absent")); err != nil || len(got) != 0 {
		t.Errorf("unbound %q with %v on a kernel without VT consoles, want nothing", got, err)
	}
}

func TestAVTConsoleWithoutANameIsAnError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "vtcon0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := UnbindFramebufferConsoles(dir); err == nil || !strings.Contains(err.Error(), "read the name of "+filepath.Join(dir, "vtcon0")) {
		t.Errorf("got %v, want an error naming the console whose name could not be read", err)
	}
}
