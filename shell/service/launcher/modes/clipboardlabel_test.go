package modes

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	shelllocales "github.com/stubbedev/lys/desktop/i18n/locales/shell"
	"github.com/stubbedev/lys/desktop/service/clipboard"
)

// unisolated drops the isolation marks Fluent wraps placeables in.
var unisolated = strings.NewReplacer("\u2068", "", "\u2069", "")

// labelsIn labels each selection, oldest first, in locale's language.
func labelsIn(t *testing.T, locale string, maxChars int, selections ...[2]string) []string {
	t.Helper()
	l, err := i18n.Select(i18n.NewAssets(shelllocales.FS), []i18n.LangID{i18n.MustLangID(locale)})
	if err != nil {
		t.Fatal(err)
	}
	history, _ := newFakeClipboard(t, selections...)
	snap := history.Entries()
	defer snap.Release()
	labels := make([]string, 0, len(selections))
	for _, e := range snap.Entries() {
		labels = append(labels, unisolated.Replace(EntryLabel(l, e, maxChars)))
	}
	slices.Reverse(labels)
	return labels
}

var everyKind = [][2]string{
	{"text/plain", "  first line\n  second line  "},
	{clipboard.URIListMime, "file:///tmp/notes.txt\n"},
	{clipboard.URIListMime, "file:///tmp/a.txt\nfile:///tmp/b.txt\n"},
	{clipboard.URIListMime, "https://example.com\n"},
	{"image/png", strings.Repeat("\x00", 2048)},
	{"application/pdf", "%PDF"},
}

func TestAClipboardRowIsLabelledInTheLocalesLanguageAndUnits(t *testing.T) {
	for locale, want := range map[string][]string{
		"en-US": {"first line second line", "notes.txt", "a.txt, b.txt (2 files)", "No files", "Image · image/png · 2.0 KiB", "application/pdf · 4 B"},
		"de":    {"first line second line", "notes.txt", "a.txt, b.txt (2 Dateien)", "Keine Dateien", "Bild · image/png · 2,0 KiB", "application/pdf · 4 B"},
		"fr":    {"first line second line", "notes.txt", "a.txt, b.txt (2 fichiers)", "Aucun fichier", "Image · image/png · 2,0\u00a0Kio", "application/pdf · 4\u00a0o"},
		"pt":    {"first line second line", "notes.txt", "a.txt, b.txt (2 ficheiros)", "Nenhum ficheiro", "Imagem · image/png · 2,0 KiB", "application/pdf · 4 B"},
	} {
		if got := labelsIn(t, locale, 160, everyKind...); !slices.Equal(got, want) {
			t.Errorf("%s labels = %q, want %q", locale, got, want)
		}
	}
}

func TestAClipboardRowKeepsNoEnglishInAnotherLanguage(t *testing.T) {
	for _, label := range labelsIn(t, "de", 160, everyKind[2:]...) {
		for _, english := range []string{"Image", "files", "KB"} {
			if strings.Contains(label, english) {
				t.Errorf("de label %q carries the English %q", label, english)
			}
		}
	}
}

func TestACopiedTextIsShownAsItIsNotTranslated(t *testing.T) {
	if got := labelsIn(t, "de", 160, [2]string{"text/plain", "Image · 2 files"}); !slices.Equal(got, []string{"Image · 2 files"}) {
		t.Errorf("de text label = %q, want the copied text verbatim", got)
	}
}

func TestManyCopiedFilesNameWhatFitsAndKeepTheirCount(t *testing.T) {
	uris := "file:///tmp/alpha.txt\nfile:///tmp/beta.txt\nfile:///tmp/gamma.txt\n"
	if got := labelsIn(t, "en-US", 20, [2]string{clipboard.URIListMime, uris}); !slices.Equal(got, []string{"alpha.txt, beta.txt, … (3 files)"}) {
		t.Errorf("label = %q, want the names that fit, then the count", got)
	}
	long := "file:///tmp/" + strings.Repeat("x", 30) + "\nfile:///tmp/y\n"
	if got := labelsIn(t, "en-US", 10, [2]string{clipboard.URIListMime, long}); !slices.Equal(got, []string{strings.Repeat("x", 30) + ", … (2 files)"}) {
		t.Errorf("label = %q, want the first name even past the limit", got)
	}
}

func TestALongTextIsCutToTheRow(t *testing.T) {
	got := labelsIn(t, "en-US", 5, [2]string{"text/plain", "abcdefgh"})
	if !slices.Equal(got, []string{"abcd…"}) {
		t.Errorf("label = %q, want the text cut to five characters", got)
	}
}
