package config

import "testing"

func TestTheScreenshotDirectoryIsTheConfiguredOneThenThePicturesFolder(t *testing.T) {
	for _, c := range []struct {
		configured string
		env        map[string]string
		want       string
	}{
		{"/custom", map[string]string{"XDG_PICTURES_DIR": "/pics", "HOME": "/home/u"}, "/custom"},
		{"", map[string]string{"XDG_PICTURES_DIR": "/pics", "HOME": "/home/u"}, "/pics"},
		{"", map[string]string{"HOME": "/home/u"}, "/home/u/Pictures"},
		{"", nil, "."},
	} {
		env := func(name string) string { return c.env[name] }
		if got := ScreenshotDirectory(c.configured, env); got != c.want {
			t.Errorf("%q with %v: %q, want %q", c.configured, c.env, got, c.want)
		}
	}
}
