package config

import (
	"regexp"
	"slices"
	"testing"
)

var codeSpan = regexp.MustCompile("`[^`]*`")

var rejectedSpellings = []struct {
	pattern *regexp.Regexp
	want    string
}{
	{regexp.MustCompile(`(?i)\bcolour\w*`), "color"},
	{regexp.MustCompile(`(?i)\bcapitalis\w*`), "capitalize"}, //nolint:misspell // the British form this check rejects
	{regexp.MustCompile(`(?i)\bcentr(e|ed|es|ing)\b`), "center"},
	{regexp.MustCompile(`(?i)\bbehaviour\w*`), "behavior"},
	{regexp.MustCompile(`(?i)\bvisualis\w*`), "visualize"},
	{regexp.MustCompile(`(?i)\blabell(ed|ing)\b`), "labeled"},
	{regexp.MustCompile(`(?i)\benrolment\w*`), "enrollment"},
	{regexp.MustCompile(`(?i)\bwi-?fi\b`), "Wi-Fi"},
}

func misspellings(doc string) []string {
	prose := codeSpan.ReplaceAllString(doc, "")
	var found []string
	for _, r := range rejectedSpellings {
		for _, m := range r.pattern.FindAllString(prose, -1) {
			if m == r.want {
				continue
			}
			found = append(found, m+" (want "+r.want+")")
		}
	}
	return found
}

func TestSchemaDocsUseTheCatalogueSpelling(t *testing.T) {
	keys := make([]string, 0, len(schemaDocs))
	for k := range schemaDocs {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if found := misspellings(schemaDocs[k]); len(found) > 0 {
			t.Errorf("%s: %v", k, found)
		}
	}
}

func TestMisspellingsRejectsTheBritishFormsAndWiFi(t *testing.T) {
	for _, doc := range []string{
		"Warms the display colours.",
		"Capitalise the first letter.",
		"Transitions are centred on dawn.",
		"The same behaviour.",
		"Bars visualising the stream.",
		"A labelled workspace.",
		"Fingerprint enrolment.",
		"The WiFi icon.",
		"The wifi/wired icon.",
	} {
		if len(misspellings(doc)) == 0 {
			t.Errorf("%q passed", doc)
		}
	}
}

func TestMisspellingsAcceptsTheCatalogueSpellingAndCode(t *testing.T) {
	for _, doc := range []string{
		"Warms the display colors, capitalize, centered, behavior.",
		"Bars visualizing the stream of a labeled workspace after enrollment.",
		"The Wi-Fi icon.",
		"Set `url` under `[wifi]` or `wifi-signal-icons`.",
	} {
		if found := misspellings(doc); len(found) > 0 {
			t.Errorf("%q: %v", doc, found)
		}
	}
}
