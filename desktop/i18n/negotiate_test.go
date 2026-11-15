package i18n

import (
	"strings"
	"testing"
)

func langs(ids []LangID) string {
	var out []string
	for _, id := range ids {
		out = append(out, id.String())
	}
	return strings.Join(out, ",")
}

func TestParseLangIDNormalizes(t *testing.T) {
	for in, want := range map[string]string{
		"en-US":           "en-US",
		"en_us":           "en-US",
		"EN-latn-us":      "en-Latn-US",
		"und":             "und",
		"de-DE-1996-1996": "de-DE-1996",
		"sl-rozaj-biske":  "sl-biske-rozaj",
		"es-419":          "es-419",
	} {
		id, err := ParseLangID(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if id.String() != want {
			t.Errorf("%q -> %q, want %q", in, id, want)
		}
	}
	for _, bad := range []string{"", "C", "POSIX-x", "en--US", "e", "en-US-u-ca-gregory", "abcd"} {
		if _, err := ParseLangID(bad); err == nil {
			t.Errorf("%q should not parse", bad)
		}
	}
}

func TestRequestedLanguagesFromEnvironment(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{}, ""},
		{map[string]string{"LANG": "fr_FR.UTF-8"}, "fr-FR"},
		{map[string]string{"LANG": "de_DE.UTF-8@euro"}, "de-DE"},
		{map[string]string{"LANG": "C.UTF-8"}, ""},
		{map[string]string{"LANG": "POSIX"}, "posix"}, // five letters is a valid language subtag
		{map[string]string{"LC_ALL": "fr_FR", "LC_MESSAGES": "de_DE", "LANG": "en_US"}, "fr-FR,de-DE,en-US"},
		{map[string]string{"LANGUAGE": "fr:de_DE:fr", "LANG": "en_US.UTF-8"}, "fr,de-DE,en-US"},
		{map[string]string{"LC_ALL": "en_US", "LANG": "en_US.UTF-8"}, "en-US"},
		{map[string]string{"LANGUAGE": "fr::de", "LANG": ""}, "fr,de"},
	}
	for _, c := range cases {
		got := langs(requestedLanguages(func(k string) string { return c.env[k] }))
		if got != c.want {
			t.Errorf("%v -> %q, want %q", c.env, got, c.want)
		}
	}
}

func parseLangs(t *testing.T, list string) []LangID {
	t.Helper()
	var ids []LangID
	if list == "" {
		return nil
	}
	for tag := range strings.SplitSeq(list, ",") {
		ids = append(ids, MustLangID(tag))
	}
	return ids
}

func TestNegotiateOrdersCataloguesByRequest(t *testing.T) {
	available := parseLangs(t, "de,de-AT,en-US,fr,pt")
	cases := map[string]string{
		"de-AT":       "de-AT,de,en-US",
		"de-CH":       "de,de-AT,en-US",
		"de":          "de,de-AT,en-US",
		"fr-CA":       "fr,en-US",
		"pt-BR":       "pt,en-US",
		"en-GB":       "en-US",
		"ja-JP":       "en-US",
		"und":         "en-US",
		"":            "en-US",
		"ja,pt-PT,fr": "pt,fr,en-US",
		"en-US,fr":    "en-US,fr",
		"de-1996":     "de,de-AT,en-US",
	}
	for req, want := range cases {
		if got := langs(negotiate(parseLangs(t, req), available)); got != want {
			t.Errorf("%q -> %q, want %q", req, got, want)
		}
	}
}

func TestSystemLocalesResolveToTheirCatalogue(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"LANG": "de_DE.UTF-8"}, "de,en-US"},
		{map[string]string{"LANG": "de_AT.UTF-8"}, "de,en-US"},
		{map[string]string{"LANG": "de_CH"}, "de,en-US"},
		{map[string]string{"LANG": "es_ES.UTF-8"}, "es,en-US"},
		{map[string]string{"LANG": "es_MX.UTF-8"}, "es,en-US"},
		{map[string]string{"LANG": "it_IT.UTF-8"}, "it,en-US"},
		{map[string]string{"LANG": "pt_PT.UTF-8"}, "pt,en-US"},
		{map[string]string{"LANG": "pt_BR.UTF-8"}, "pt,en-US"},
		{map[string]string{"LANG": "nl_NL.UTF-8"}, "nl,en-US"},
		{map[string]string{"LANG": "nl_BE.UTF-8@euro"}, "nl,en-US"},
		{map[string]string{"LANG": "fr_CA.UTF-8"}, "fr,en-US"},
		{map[string]string{"LANG": "en_GB.UTF-8"}, "en-US"},
		{map[string]string{"LANG": "ja_JP.UTF-8"}, "en-US"},
		{map[string]string{"LANG": "C.UTF-8"}, "en-US"},
		{map[string]string{}, "en-US"},
		{map[string]string{"LANGUAGE": "ja:es_MX:it", "LANG": "de_DE.UTF-8"}, "es,it,de,en-US"},
	}
	english, err := Select(domains["shell"], nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		requested := requestedLanguages(func(k string) string { return c.env[k] })
		for name, assets := range domains {
			l, err := Select(assets, requested)
			if err != nil {
				t.Fatalf("%s %v: %v", name, c.env, err)
			}
			if got := langs(l.Languages()); got != c.want {
				t.Errorf("%s %v -> %q, want %q", name, c.env, got, c.want)
			}
		}
		shell, err := Select(domains["shell"], requested)
		if err != nil {
			t.Fatal(err)
		}
		translated := shell.Get("osd-brightness") != english.Get("osd-brightness")
		if wantTranslated := !strings.HasPrefix(c.want, "en-US"); translated != wantTranslated {
			t.Errorf("%v: osd-brightness = %q", c.env, shell.Get("osd-brightness"))
		}
	}
}

func TestSelectOrdersBundlesByPreference(t *testing.T) {
	l := loaderFor(t, map[string]string{
		"en-US/_a.ftl": "x = en\n",
		"fr/_a.ftl":    "x = fr\n",
	}, "de-DE", "fr-FR")
	if got := langs(l.Languages()); got != "fr,en-US" {
		t.Errorf("languages = %q", got)
	}
	if l.Get("x") != "fr" {
		t.Errorf("x = %q", l.Get("x"))
	}
	en := loaderFor(t, map[string]string{
		"en-US/_a.ftl": "x = en\n",
		"fr/_a.ftl":    "x = fr\n",
	}, "de-DE")
	if en.Get("x") != "en" {
		t.Errorf("unsupported locale must fall back: %q", en.Get("x"))
	}
}
