package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestARoundCountIsWrittenShortInEveryLanguage(t *testing.T) {
	const nb = " "
	for lang, want := range map[string][]string{
		"en-US": {"1K", "10K", "100K", "1M"},
		"de":    {"1.000", "10.000", "100.000", "1" + nb + "Mio."},
		"es":    {"1" + nb + "mil", "10" + nb + "mil", "100" + nb + "mil", "1" + nb + "M"},
		"fr":    {"1" + nb + "k", "10" + nb + "k", "100" + nb + "k", "1" + nb + "M"},
		"it":    {"1.000", "10.000", "100.000", "1" + nb + "Mln"},
		"nl":    {"1K", "10K", "100K", "1" + nb + "mln."},
		"pt":    {"1" + nb + "mil", "10" + nb + "mil", "100" + nb + "mil", "1" + nb + "M"},
	} {
		tr, err := i18n.Select(locale.Assets(), []i18n.LangID{i18n.MustLangID(lang)})
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, n := range []int64{1_000, 10_000, 100_000, 1_000_000} {
			got = append(got, unisolated(compactCount(tr, n)))
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s writes 1000, 10000, 100000 and 1000000 as %q, want %q", lang, got, want)
		}
	}
}

func TestACountThatIsNotRoundStaysInDigits(t *testing.T) {
	tr, err := i18n.Select(locale.Assets(), []i18n.LangID{i18n.MustLangID("de")})
	if err != nil {
		t.Fatal(err)
	}
	for n, want := range map[int64]string{0: "0", 25: "25", 999: "999", 1_500: "1500", 1_000_500: "1000500"} {
		if got := compactCount(tr, n); got != want {
			t.Errorf("%d is written %q, want %q", n, got, want)
		}
	}
}

func TestTheStoredNotificationHistoryIsPickedFromRoundCounts(t *testing.T) {
	k := testKit(t, "")
	p := pickerOf(t, k, pageRow(t, "notifications", "modules.notifications.history-stored"))
	tr, err := i18n.Select(locale.Assets(), []i18n.LangID{i18n.Fallback})
	if err != nil {
		t.Fatal(err)
	}
	if got := chipLabels(p); !slices.Equal(got, []string{compactCount(tr, 1_000), compactCount(tr, 10_000), compactCount(tr, 100_000)}) {
		t.Fatalf("the chips read %q", got)
	}
	if p.active() != 1 {
		t.Errorf("the default 10000 shows chip %d", p.active())
	}
	p.chips[2].SetActive(true)
	if v := k.store.value("modules.notifications.history-stored"); v != int64(100_000) {
		t.Errorf("choosing 100K stored %v", v)
	}
}
