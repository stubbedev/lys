package xkb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestARelativeConfigHomeFallsBackToHome(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	for _, sub := range []string{"home/.config/xkb", "config/xkb"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	for name, c := range map[string]struct {
		env  map[string]string
		want string
	}{
		"relative config home": {map[string]string{"XDG_CONFIG_HOME": "config", "HOME": home}, filepath.Join(home, ".config", "xkb")},
		"relative home":        {map[string]string{"HOME": "home"}, ""},
	} {
		ctx := NewContextFlags(func(n string) string { return c.env[n] }, ContextNoDefaultIncludes)
		ctx.SetLogLevel(LogCritical)
		ctx.AppendDefaultIncludePaths()
		first, _ := ctx.IncludePath(0)
		if c.want != "" && first != c.want {
			t.Errorf("%s: first include path %q, want %q", name, first, c.want)
		}
		for _, p := range ctx.IncludePaths() {
			if !filepath.IsAbs(p) {
				t.Errorf("%s: relative include path %q", name, p)
			}
		}
	}
}
