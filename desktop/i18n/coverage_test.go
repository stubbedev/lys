package i18n

import (
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	greeterlocales "github.com/stubbedev/lys/desktop/i18n/locales/greeter"
	settingslocales "github.com/stubbedev/lys/desktop/i18n/locales/settings"
	shelllocales "github.com/stubbedev/lys/desktop/i18n/locales/shell"
)

var trees = map[string]Assets{
	"lys-shell-core": NewAssets(shelllocales.FS),
	"lys-shell-i18n": NewAssets(settingslocales.FS),
}

var domains = map[string]Assets{
	"shell":    NewAssets(shelllocales.FS),
	"settings": NewAssets(settingslocales.FS),
	"greeter":  NewFileAssets(greeterlocales.FS, "lys-greeter.ftl"),
}

const shipped = "de,en-US,es,fr,it,nl,pt"

func TestEveryDomainShipsEveryLanguage(t *testing.T) {
	for name, assets := range domains {
		available, err := assets.Available()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := langs(available); got != shipped {
			t.Errorf("%s: catalogues %s, want %s", name, got, shipped)
		}
	}
}

func TestEveryCatalogueCarriesEveryEnglishMessage(t *testing.T) {
	for name, assets := range domains {
		gaps, err := assets.Gaps()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, gap := range gaps {
			t.Errorf("%s: %s", name, gap)
		}
	}
}

func mapAssets(files map[string]string) Assets {
	m := fstest.MapFS{}
	for name, body := range files {
		m[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return NewAssets(m)
}

func TestGapsNameTheLocaleFileAndID(t *testing.T) {
	assets := mapAssets(map[string]string{
		"en-US/_a.ftl": "a = one {\"{cmd}\"}\n    .description = two\nb = three\n-brand = lys\n",
		"en-US/sub/_c.ftl": "c = { $n ->\n    [one] one\n   *[other] { $n } and { $m }\n}\n" +
			"z = { $n ->\n    [one] one\n   *[other] { $n }\n}\n",
		"de/_a.ftl": "a = eins {\"{cmd}\"}\nd = vier\n-brand = lys\n",
		"de/sub/_c.ftl": "c = { $n ->\n    [one] eins\n   *[other] { $n }\n}\n" +
			"z = { $n ->\n    [one] eins\n    [few] wenige\n   *[other] { $n }\n}\n",
		"es/_a.ftl": "a = uno {\"{cmd}\"}\n    .description = dos\nb = tres\n-brand = lys\n",
		"es/sub/_c.ftl": "c = { $n ->\n    [one] uno\n   *[other] { $n } y { $m }\n}\n" +
			"z = { $n ->\n    [one] uno\n    [many] { $n }\n   *[other] { $n }\n}\n",
		"pt-BR/_a.ftl": "a = um {\"{cmd}\"}\n    .description = dois\nb = três\n-brand = lys\n",
		"pt-BR/sub/_c.ftl": "c = { $n ->\n    [one] um\n   *[other] { $n } e { $m }\n}\n" +
			"z = { $n ->\n    [one] um\n   *[other] { $n }\n}\n",
	})
	got, err := assets.Gaps()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`de: _a.ftl: a.description: missing`,
		`de: _a.ftl: b: missing`,
		`de: _a.ftl: d: not in en-US`,
		`de: sub/_c.ftl: c: placeables [$n], want [$m $n]`,
		`de: sub/_c.ftl: z: variant [few] is no plural category of de`,
		`i18n: catalogue pt-BR has no plural rule set`,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestEnglishOwnPluralKeysAreChecked(t *testing.T) {
	got, err := mapAssets(map[string]string{
		"en-US/_a.ftl": "x = { $n ->\n    [many] many\n   *[other] other\n}\n",
	}).Gaps()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"en-US: _a.ftl: x: variant [many] is no plural category of en-US"}; !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestEmbeddedPartialsParseCleanly(t *testing.T) {
	for name, assets := range domains {
		locales, err := assets.Available()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, loc := range locales {
			files, err := assets.partials(loc.String())
			if err != nil {
				t.Fatal(err)
			}
			if len(files) == 0 {
				t.Errorf("%s/%s: no partials", name, loc)
			}
			for _, f := range files {
				data, err := fs.ReadFile(f.tree, f.path)
				if err != nil {
					t.Fatal(err)
				}
				res := parseResource(strings.ReplaceAll(string(data), "\r\n", "\n"))
				for _, j := range res.junk {
					t.Errorf("%s/%s: junk (%v): %q", name, f.path, j.err, j.content)
				}
			}
			if _, err := cardinalRulesFor(loc); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}
	}
}

func TestDomainsInitialize(t *testing.T) {
	if Shell() == nil || Settings() == nil {
		t.Fatal("domain loaders are nil")
	}
	if got := Settings().Get("app-name"); got != "lys-shell" && !strings.Contains(got, "lys-shell") {
		t.Errorf("app-name = %q", got)
	}
}

func TestEmbeddedMessagesResolve(t *testing.T) {
	load := func(tree string, want ...string) *Loader {
		var req []LangID
		for _, w := range want {
			req = append(req, MustLangID(w))
		}
		l, err := Select(trees[tree], req)
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	shellEN := load("lys-shell-core", "en-US")
	shellFR := load("lys-shell-core", "fr-FR")
	settings := load("lys-shell-i18n")
	cases := []struct {
		got, want string
	}{
		{shellEN.Get("osd-brightness"), "Brightness"},
		{shellEN.Get("bar-bluetooth-connected-count", Int("count", 3)), "\u20683\u2069 Connected"},
		{shellEN.Get("osd-toggle-on", Str("label", "Caps Lock")), "\u2068Caps Lock\u2069 On"},
		{shellFR.Get("osd-toggle-on", Str("label", "Verr. Maj")), "\u2068Verr. Maj\u2069 activé"},
		{shellFR.Get("bar-bluetooth-disabled"), "Désactivé"},
		{settings.Get("settings-bar-scale"), "Scale"},
		{settings.Attr("settings-bar-scale", "description"), "Bar-specific scale multiplier for spacing, radius, and other elements"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestTheFrenchLauncherSettingsResolve(t *testing.T) {
	l, err := Select(trees["lys-shell-i18n"], []LangID{MustLangID("fr-FR")})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := l.Attr("settings-launcher-window-format", "description"), `{w}`; !strings.Contains(got, want) {
		t.Errorf("the window format's description %q lost %s", got, want)
	}
	shell, err := Select(trees["lys-shell-core"], []LangID{MustLangID("fr-FR")})
	if err != nil {
		t.Fatal(err)
	}
	failed := shell.Get("launcher-failed", Str("what", "Alacritty"), Str("error", "x"))
	if strings.Contains(failed, "de Alacritty") {
		t.Errorf("launcher-failed does not elide before a vowel: %q", failed)
	}
}
