package i18n

import (
	"io/fs"
	"maps"
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

var shipped = langs(Shipped())

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

func TestGapsNameACountShownWithoutAPluralSelect(t *testing.T) {
	got, err := mapAssets(map[string]string{
		"en-US/_a.ftl": "bare = { $count } items\n" +
			"selected = { $count ->\n    [one] { $count } item\n   *[other] { $count } items\n}\n" +
			"number = { NUMBER($total) ->\n    [one] { $shown } of { $total } item\n   *[other] { $shown } of { $total } items\n}\n" +
			"nested = { $new ->\n    [one] { $new } message\n   *[other] { $new } messages\n} ({ $count } unread)\n" +
			"other-var = { $count ->\n    [one] one\n   *[other] { $max } of { $count }\n}\n" +
			"unit = { $minutes }m ago\n" +
			"attr = Label\n    .description = { $words } words\n",
		"de/_a.ftl": "bare = { $count ->\n    [one] { $count } Eintrag\n   *[other] { $count } Einträge\n}\n" +
			"selected = { $count } Einträge\n" +
			"number = { NUMBER($total) ->\n    [one] { $shown } von { $total } Eintrag\n   *[other] { $shown } von { $total } Einträgen\n}\n" +
			"nested = { $new ->\n    [one] { $new } Nachricht\n   *[other] { $new } Nachrichten\n} ({ $count ->\n    [one] { $count } ungelesen\n   *[other] { $count } ungelesen\n})\n" +
			"other-var = { $count ->\n    [one] eins\n   *[other] { $max } von { $count }\n}\n" +
			"unit = vor { $minutes } min\n" +
			"attr = Bezeichnung\n    .description = { $words ->\n        [one] { $words } Wort\n       *[other] { $words } Wörter\n    }\n",
	}).Gaps()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"de: _a.ftl: other-var: count $max is shown without a plural select on it",
		"de: _a.ftl: selected: count $count is shown without a plural select on it",
		"en-US: _a.ftl: attr.description: count $words is shown without a plural select on it",
		"en-US: _a.ftl: bare: count $count is shown without a plural select on it",
		"en-US: _a.ftl: nested: count $count is shown without a plural select on it",
		"en-US: _a.ftl: other-var: count $max is shown without a plural select on it",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestNoIDIsDefinedInTwoDomains(t *testing.T) {
	names := slices.Sorted(maps.Keys(domains))
	defined := map[string]string{}
	for _, name := range names {
		entries, err := domains[name].catalogueOf(Fallback)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, id := range slices.Sorted(maps.Keys(entries)) {
			if first, dup := defined[id]; dup {
				t.Errorf("%s is defined in both the %s and the %s domain", id, first, name)
				continue
			}
			defined[id] = name
		}
	}
}

func TestEmbeddedCountsFollowEachLanguagesPluralRules(t *testing.T) {
	load := func(tree, lang string) *Loader {
		l, err := Select(trees[tree], []LangID{MustLangID(lang)})
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	en, de, es, it := load("lys-shell-core", "en-US"), load("lys-shell-core", "de"), load("lys-shell-core", "es"), load("lys-shell-core", "it")
	iso := func(n string) string { return "\u2068" + n + "\u2069" }
	cases := []struct {
		got, want string
	}{
		{en.Get("lock-failed-attempts", Int("count", 1)), "Incorrect password — " + iso("1") + " failed attempt"},
		{en.Get("lock-failed-attempts", Int("count", 3)), "Incorrect password — " + iso("3") + " failed attempts"},
		{de.Get("lock-failed-attempts", Int("count", 1)), "Falsches Passwort – " + iso("1") + " Fehlversuch"},
		{es.Get("lock-failed-attempts", Int("count", 1000000)), "Contraseña incorrecta — " + iso("1000000") + " de intentos fallidos"},
		{en.Get("dropdown-bitwarden-generator-passphrase-caption", Int("words", 1)), iso("1") + " word"},
		{en.Get("dropdown-bitwarden-generator-password-caption", Int("length", 1), Str("charsets", "a-z")), iso("1") + " character: " + iso("a-z")},
		{en.Get("dropdown-bitwarden-more", Int("shown", 50), Int("total", 51)), "Showing " + iso("50") + " of " + iso("51") + " items. Search to narrow them down."},
		{en.Get("dropdown-bitwarden-send-accesses-of", Int("opened", 0), Int("max", 1)), "opened " + iso("0") + " of " + iso("1") + " time"},
		{en.Get("dropdown-bluetooth-pairing-entering", Int("entered", 0), Int("total", 1)), iso("0") + " of " + iso("1") + " digit entered"},
		{it.Get("notification-dropdown-group-more", Int("count", 1)), "Ancora " + iso("1")},
		{it.Get("notification-dropdown-group-more", Int("count", 2)), "Altre " + iso("2")},
		{de.Get("mail-notification-arrived", Int("new", 1), Int("count", 1)), iso(iso("1")+" neue Nachricht") + " (" + iso(iso("1")+" ungelesen") + ")"},
		{load("lys-shell-i18n", "en-US").Get("settings-animations-share-picker"), "Share Picker"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
	if got, other := en.Get("lock-failed-attempts", Str("count", "1")), en.Get("lock-failed-attempts", Int("count", 1)); got == other {
		t.Errorf("a count passed as text %q selects like a number", got)
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
	if got, want := l.Attr("settings-launcher-search-web-url", "description"), `{query}`; !strings.Contains(got, want) {
		t.Errorf("the web search URL's description %q lost its literal %s", got, want)
	}
	shell, err := Select(trees["lys-shell-core"], []LangID{MustLangID("fr-FR")})
	if err != nil {
		t.Fatal(err)
	}
	failed := shell.Get("launcher-failed", Str("what", "Alacritty"))
	if strings.Contains(failed, "de Alacritty") {
		t.Errorf("launcher-failed does not elide before a vowel: %q", failed)
	}
}
