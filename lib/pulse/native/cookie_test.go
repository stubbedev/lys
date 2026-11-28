package native

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeCookie(t *testing.T, path string, b byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte{b}, CookieLength), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTheCookieComesFromTheXDGConfigHomeThenHome(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	writeCookie(t, filepath.Join(home, ".config", "pulse", "cookie"), 1)
	writeCookie(t, filepath.Join(home, ".pulse-cookie"), 2)
	writeCookie(t, filepath.Join(dir, "xdg", "pulse", "cookie"), 3)
	for name, c := range map[string]struct {
		env  map[string]string
		want byte
	}{
		"config home":                  {map[string]string{"HOME": home, "XDG_CONFIG_HOME": filepath.Join(dir, "xdg")}, 3},
		"home's .config":               {map[string]string{"HOME": home}, 1},
		"a relative config home":       {map[string]string{"HOME": home, "XDG_CONFIG_HOME": "xdg"}, 1},
		"the legacy file":              {map[string]string{"HOME": home, "XDG_CONFIG_HOME": filepath.Join(dir, "none")}, 2},
		"no home and no config home":   {map[string]string{}, 0},
		"PULSE_COOKIE before the rest": {map[string]string{"HOME": home, "PULSE_COOKIE": filepath.Join(home, ".pulse-cookie")}, 2},
	} {
		t.Run(name, func(t *testing.T) {
			t.Chdir(dir)
			got := loadCookie(func(k string) string { return c.env[k] })
			if !bytes.Equal(got, bytes.Repeat([]byte{c.want}, CookieLength)) {
				t.Errorf("cookie starts %v, want all %d", got[:4], c.want)
			}
		})
	}
}
