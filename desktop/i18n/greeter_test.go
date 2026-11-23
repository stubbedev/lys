package i18n

import (
	"testing"

	greeterlocales "github.com/stubbedev/lys/desktop/i18n/locales/greeter"
)

func TestGreeterDomain(t *testing.T) {
	assets := NewFileAssets(greeterlocales.FS, "lys-greeter.ftl")
	available, err := assets.Available()
	if err != nil || langs(available) != shipped {
		t.Fatalf("available = %v, %v; want %s", available, err, shipped)
	}
	fr, _ := ParseLangID("fr-FR")
	l, err := Select(assets, []LangID{fr})
	if err != nil {
		t.Fatal(err)
	}
	if got := l.Get("greeter-login-incorrect"); got != "Nom d'utilisateur ou mot de passe incorrect" {
		t.Errorf("fr incorrect = %q", got)
	}
	if got := l.Get("greeter-username"); got != "Nom d'utilisateur" {
		t.Errorf("fr username = %q", got)
	}
	en, _ := ParseLangID("ja-JP")
	l, _ = Select(assets, []LangID{en})
	if got := l.Get("greeter-caps-lock"); got != "Caps Lock is on" {
		t.Errorf("fallback caps lock = %q", got)
	}
	if got := l.Get("greeter-greetd-unavailable"); got != "greetd unavailable" {
		t.Errorf("unavailable = %q", got)
	}
	if got := l.Get("greeter-login-incorrect"); got != "Incorrect username or password" {
		t.Errorf("fallback incorrect = %q", got)
	}
	if got := l.Get("lock-nothing"); got == "" {
		t.Error("a missing id came back empty, want the no-localization marker")
	}
	if Greeter() == nil {
		t.Error("no greeter loader")
	}
	// A tree without the file has no fallback to load.
	if _, err := Select(NewFileAssets(greeterlocales.FS, "nope.ftl"), []LangID{en}); err == nil {
		t.Error("a missing file loaded")
	}
}
