package locale

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/i18n/i18ntest"
)

func TestEveryLanguageHasEveryMessageEnglishHas(t *testing.T) {
	gaps, err := Assets().Gaps()
	if err != nil {
		t.Fatal(err)
	}
	for _, gap := range gaps {
		t.Error(gap)
	}
}

func TestEveryLanguageIsShippedAndResolves(t *testing.T) {
	available, err := Assets().Available()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, lang := range available {
		got = append(got, lang.String())
		l, err := i18n.Select(Assets(), []i18n.LangID{lang})
		if err != nil {
			t.Fatal(err)
		}
		if !l.Languages()[0].Equal(lang) {
			t.Errorf("%s: negotiated %v", lang, l.Languages())
		}
		if msg := l.Get("files-rename-failed", i18n.Str("name", "a.png")); !strings.Contains(msg, "a.png") {
			t.Errorf("%s: files-rename-failed reads %q", lang, msg)
		}
	}
	if want := "de en-US es fr it nl pt"; strings.Join(got, " ") != want {
		t.Fatalf("languages %q, want %q", strings.Join(got, " "), want)
	}
}

func TestAMessageNoneHasIsNotFound(t *testing.T) {
	if T().Has("no-such-message") {
		t.Fatal("found")
	}
}

func TestNoMessageTakesAnErrorsOwnText(t *testing.T) {
	available, err := Assets().Available()
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range available {
		src, err := Assets().Source(lang.String())
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range i18ntest.RawReasons(src) {
			t.Errorf("%s: %s takes an error's text", lang, id)
		}
	}
}

// The status line's counts take their language's plural forms: none,
// one and many items read apart, and a selection names its count and
// the folder's.
func TestTheStatusLineCountsInEveryLanguage(t *testing.T) {
	available, err := Assets().Available()
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range available {
		l, err := i18n.Select(Assets(), []i18n.LangID{lang})
		if err != nil {
			t.Fatal(err)
		}
		items := func(n int64) string { return l.Get("files-status-items", i18n.Int("count", n)) }
		selected := func(n, total int64) string {
			return l.Get("files-status-selected", i18n.Int("count", n), i18n.Int("total", total))
		}
		if none, one, many := items(0), items(1), items(7); none == one || one == many || strings.Contains(none, "0") || !strings.Contains(many, "7") {
			t.Errorf("%s: 0, 1 and 7 items read %q, %q, %q", lang, none, one, many)
		}
		if all, one, some := selected(1, 1), selected(1, 9), selected(3, 9); all == one || !strings.Contains(one, "9") || !strings.Contains(some, "3") || !strings.Contains(some, "9") {
			t.Errorf("%s: 1 of 1, 1 of 9 and 3 of 9 selected read %q, %q, %q", lang, all, one, some)
		}
		if sized := l.Get("files-status-size", i18n.Str("selection", selected(3, 9)), i18n.Str("size", "4 KiB")); !strings.Contains(sized, "4 KiB") || !strings.Contains(sized, selected(3, 9)) {
			t.Errorf("%s: a selection's size reads %q", lang, sized)
		}
	}
}
