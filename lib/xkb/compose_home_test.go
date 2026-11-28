package xkb

import (
	"path/filepath"
	"testing"
)

func TestComposeReadsTheUserFilesOnlyUnderAbsolutePaths(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	writeFile(t, filepath.Join(cwd, "home", ".XCompose"), "<dead_acute> <e> : \"home\"\n")
	writeFile(t, filepath.Join(cwd, "configured", ".config", "XCompose"), "<dead_acute> <e> : \"config\"\n")
	writeFile(t, filepath.Join(cwd, "config", "XCompose"), "<dead_acute> <e> : \"relative config\"\n")
	for name, tc := range map[string]struct {
		vars map[string]string
		want string
	}{
		"absolute home":                     {map[string]string{"HOME": filepath.Join(cwd, "home")}, "home"},
		"absolute home with a config":       {map[string]string{"HOME": filepath.Join(cwd, "configured")}, "config"},
		"absolute config":                   {map[string]string{"HOME": filepath.Join(cwd, "home"), "XDG_CONFIG_HOME": filepath.Join(cwd, "config")}, "relative config"},
		"relative config, absolute home":    {map[string]string{"HOME": filepath.Join(cwd, "home"), "XDG_CONFIG_HOME": "config"}, "home"},
		"relative home":                     {map[string]string{"HOME": "home"}, "é"},
		"relative home and relative config": {map[string]string{"HOME": "home", "XDG_CONFIG_HOME": "config"}, "é"},
	} {
		tc.vars["XLOCALEDIR"] = t.TempDir()
		table, err := composeEnv(t, tc.vars).NewComposeTableFromLocale("C")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := composeText(t, table, KeyDeadAcute, Keye); got != tc.want {
			t.Errorf("%s: dead_acute e = %q, want %q", name, got, tc.want)
		}
	}
}

func TestComposeIncludeOfHomeNeedsAnAbsoluteHome(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	writeFile(t, filepath.Join(cwd, "home", "mine"), "<dead_acute> <e> : \"mine\"\n")
	compose := filepath.Join(cwd, "Compose")
	writeFile(t, compose, "include \"%H/mine\"\n<dead_acute> <a> : \"after\"\n")
	for home, want := range map[string]bool{filepath.Join(cwd, "home"): true, "home": false} {
		c := composeEnv(t, map[string]string{"XLOCALEDIR": t.TempDir(), "XCOMPOSEFILE": compose, "HOME": home})
		table, err := c.NewComposeTableFromLocale("C")
		if err != nil {
			t.Fatalf("HOME=%s: %v", home, err)
		}
		st := table.NewState()
		st.Feed(KeyDeadAcute)
		st.Feed(Keye)
		if got := st.Status() == ComposeComposed && st.UTF8() == "mine"; got != want {
			t.Errorf("HOME=%s: include \"%%H/mine\" read %v, want %v", home, got, want)
		}
		if got := composeText(t, table, KeyDeadAcute, Keya); got != "after" {
			t.Errorf("HOME=%s: the line after the include = %q, want after", home, got)
		}
	}
}
