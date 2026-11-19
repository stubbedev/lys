package config

import "testing"

func TestTrayPatternsAreCaseSensitiveGlobsOverIDAndTitle(t *testing.T) {
	const id, title = "com.discord.Discord", "Discord"
	for _, c := range []struct {
		pattern string
		hidden  bool
	}{
		{"*discord*", true},
		{"Discord", true},
		{"*DISCORD*", false},
		{"slack", false},
	} {
		cfg := SystrayConfig{Blacklist: []string{c.pattern}}
		if got := cfg.Hidden(id, title); got != c.hidden {
			t.Errorf("%q hides %v, want %v", c.pattern, got, c.hidden)
		}
	}
	icon := "x"
	cfg := SystrayConfig{Overrides: []TrayItemOverride{{Name: "nope"}, {Name: "*Discord", Icon: &icon}, {Name: "Discord"}}}
	o, ok := cfg.Override(id, title)
	if !ok || o.Name != "*Discord" {
		t.Errorf("override %+v, %v; want the first match", o, ok)
	}
	if _, ok := cfg.Override("org.slack", "Slack"); ok {
		t.Error("an item no override names has one")
	}
}
