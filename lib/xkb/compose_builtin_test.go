package xkb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func composeEnv(t *testing.T, vars map[string]string) *Context {
	t.Helper()
	locpath := t.TempDir()
	c := NewContext(func(name string) string {
		if name == "LOCPATH" {
			return locpath
		}
		return vars[name]
	})
	c.SetLogLevel(LogCritical)
	return c
}

func composeText(t *testing.T, table *ComposeTable, syms ...Keysym) string {
	t.Helper()
	st := table.NewState()
	for _, sym := range syms {
		st.Feed(sym)
	}
	if st.Status() != ComposeComposed {
		t.Fatalf("status %d after %v, want composed", st.Status(), syms)
	}
	return st.UTF8()
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestBuiltinComposeTableIsSelfContained(t *testing.T) {
	if strings.Contains(string(builtinCompose), "\ninclude") {
		t.Fatal("the built-in Compose table includes another file")
	}
	table, err := composeEnv(t, nil).NewComposeTableFromBuffer(builtinCompose, composeFallbackLocale)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for range table.Entries() {
		n++
	}
	if n < 1000 {
		t.Fatalf("the built-in Compose table holds %d sequences", n)
	}
}

func TestComposeWithoutLocaleDataUsesTheBuiltinTable(t *testing.T) {
	for _, locale := range []string{"C", "en_US.UTF-8"} {
		c := composeEnv(t, map[string]string{"XLOCALEDIR": t.TempDir(), "HOME": t.TempDir()})
		table, err := c.NewComposeTableFromLocale(locale)
		if err != nil {
			t.Fatalf("%s without X locale data: %v", locale, err)
		}
		if got := composeText(t, table, KeyDeadAcute, Keye); got != "é" {
			t.Errorf("%s: dead_acute e = %q, want é", locale, got)
		}
	}
}

func TestComposeSystemFileWinsOverTheBuiltinTable(t *testing.T) {
	xlocale := t.TempDir()
	writeFile(t, filepath.Join(xlocale, "compose.dir"), "en_US.UTF-8/Compose:\ten_US.UTF-8\n")
	writeFile(t, filepath.Join(xlocale, "en_US.UTF-8", "Compose"), "<dead_acute> <e> : \"system\"\n")
	c := composeEnv(t, map[string]string{"XLOCALEDIR": xlocale, "HOME": t.TempDir()})
	table, err := c.NewComposeTableFromLocale("en_US.UTF-8")
	if err != nil {
		t.Fatal(err)
	}
	if got := composeText(t, table, KeyDeadAcute, Keye); got != "system" {
		t.Errorf("dead_acute e = %q, want the system file's \"system\"", got)
	}
}

func TestComposeIncludeOfTheLocaleNamesTheBuiltinTable(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".XCompose"), "include \"%L\"\n<Multi_key> <x> <x> : \"mine\"\n")
	c := composeEnv(t, map[string]string{"XLOCALEDIR": t.TempDir(), "HOME": home})
	table, err := c.NewComposeTableFromLocale("C")
	if err != nil {
		t.Fatal(err)
	}
	if got := composeText(t, table, KeyDeadAcute, Keye); got != "é" {
		t.Errorf("dead_acute e through include \"%%L\" = %q, want é", got)
	}
	if got := composeText(t, table, KeyMultiKey, Keyx, Keyx); got != "mine" {
		t.Errorf("the user's own sequence = %q, want mine", got)
	}
}

func TestComposeIncludeCannotExtendTheBuiltinTablePath(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".XCompose"), "include \"%L.extra\"\n<Multi_key> <x> <x> : \"mine\"\n")
	c := composeEnv(t, map[string]string{"XLOCALEDIR": t.TempDir(), "HOME": home})
	table, err := c.NewComposeTableFromLocale("C")
	if err != nil {
		t.Fatal(err)
	}
	st := table.NewState()
	st.Feed(KeyDeadAcute)
	if st.Status() != ComposeNothing {
		t.Errorf("dead_acute after include \"%%L.extra\": status %d, want the include skipped", st.Status())
	}
	if got := composeText(t, table, KeyMultiKey, Keyx, Keyx); got != "mine" {
		t.Errorf("the line after the skipped include = %q, want mine", got)
	}
}

func TestComposeWithoutLocaleDataRejectsAnUninstalledLocale(t *testing.T) {
	c := composeEnv(t, map[string]string{"XLOCALEDIR": t.TempDir(), "HOME": t.TempDir()})
	if _, err := c.NewComposeTableFromLocale("zz_ZZ.UTF-8"); err == nil || !strings.Contains(err.Error(), "zz_ZZ.UTF-8") {
		t.Fatalf("an uninstalled locale without X locale data: %v, want no Compose file", err)
	}
}
