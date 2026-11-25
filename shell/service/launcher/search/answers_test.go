package search

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	shelllocales "github.com/stubbedev/lys/desktop/i18n/locales/shell"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/shell/service/launcher/answer"
)

var answerClock = time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)

// loaderIn is the shell catalogue negotiated for locale alone, so a
// test reads the same whatever the host's locale is.
func loaderIn(t *testing.T, locale string) *i18n.Loader {
	t.Helper()
	l, err := i18n.Select(i18n.NewAssets(shelllocales.FS), []i18n.LangID{i18n.MustLangID(locale)})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func calculatorIn(t *testing.T, locale string, rates *answer.Rates) *Calculator {
	return NewCalculator(loaderIn(t, locale), rates, answer.NewCurrencyParser("en"), answer.NewZones(time.LoadLocation), func() time.Time { return answerClock })
}

func testCalculator(t *testing.T, rates *answer.Rates) *Calculator {
	return calculatorIn(t, "en-US", rates)
}

const ecbFixture = `<gesmes:Envelope xmlns:gesmes="http://www.gesmes.org/xml/2002-08-01" xmlns="http://www.ecb.int/vocabulary/2002-08-01/eurofxref">
<Cube><Cube time='2026-11-06'><Cube currency='USD' rate='1.0825'/><Cube currency='DKK' rate='7.4593'/></Cube></Cube>
</gesmes:Envelope>`

func ecbServer(t *testing.T, up bool) (*httptest.Server, *atomic.Int32) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		if !up {
			http.Error(w, "offline", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(ecbFixture))
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}

func stripIsolation(s string) string {
	return strings.NewReplacer("\u2068", "", "\u2069", "").Replace(s)
}

func TestCurrencyRowAppearsOnceTheRatesLoad(t *testing.T) {
	srv, requests := ecbServer(t, true)
	cache := filepath.Join(t.TempDir(), "fx.xml")
	rates := answer.NewRates(srv.URL, cache, func() time.Time { return answerClock })
	s := &sink{}
	e := start(t, s, nil, testCalculator(t, rates))
	e.SetQuery("100 usd in eur", 1)
	waittest.Until(t, func() bool {
		rows := s.last().Rows
		return len(rows) == 2 && rows[1].Entry.Title == "92.38 EUR"
	}, "the currency row")

	if sub := stripIsolation(s.last().Rows[1].Entry.Subtitle); !strings.Contains(sub, "ECB, Nov 6, 2026") || strings.Contains(sub, "old") {
		t.Errorf("subtitle = %q, want the fresh rate date", sub)
	}
	for range 20 {
		testCalculator(t, rates).Load(t.Context(), func([]Entry) { t.Error("an open with fresh rates published") })
	}
	reopened := answer.NewRates(srv.URL, cache, func() time.Time { return answerClock })
	if got := testCalculator(t, reopened).Answer("100 usd in eur"); got != nil {
		t.Errorf("answered %v before the cache was read", got)
	}
	reopened.Refresh(t.Context())
	got := testCalculator(t, reopened).Answer("1 eur in dkk")
	if len(got) != 1 || got[0].Title != "7.46 DKK" {
		t.Errorf("from the cache = %+v", got)
	}
	if n := requests.Load(); n != 1 {
		t.Errorf("feed requests = %d, want 1", n)
	}
}

func TestCurrencyWithoutNetworkOrCacheHasNoRow(t *testing.T) {
	srv, requests := ecbServer(t, false)
	rates := answer.NewRates(srv.URL, filepath.Join(t.TempDir(), "fx.xml"), func() time.Time { return answerClock })
	c := testCalculator(t, rates)
	var published atomic.Int32
	c.Load(t.Context(), func([]Entry) { published.Add(1) })
	if requests.Load() != 1 || published.Load() != 0 {
		t.Errorf("requests = %d, publishes = %d", requests.Load(), published.Load())
	}
	if got := c.Answer("100 usd in eur"); got != nil {
		t.Errorf("answered %+v with no rates", got)
	}
	if got := testCalculator(t, nil).Answer("100 usd in eur"); got != nil {
		t.Errorf("answered %+v with currencies off", got)
	}
}

func TestStaleRatesSayHowOld(t *testing.T) {
	srv, _ := ecbServer(t, true)
	fetchedAt := answerClock
	cache := filepath.Join(t.TempDir(), "fx.xml")
	answer.NewRates(srv.URL, cache, func() time.Time { return fetchedAt }).Refresh(t.Context())
	later := answerClock.Add(72 * time.Hour)
	srv.Close()
	rates := answer.NewRates(srv.URL, cache, func() time.Time { return later })
	rates.Refresh(t.Context())
	c := NewCalculator(loaderIn(t, "en-US"), rates, answer.NewCurrencyParser("en"), answer.NewZones(time.LoadLocation), func() time.Time { return later })
	got := c.Answer("$100 to €")
	if len(got) != 1 {
		t.Fatalf("answer = %+v", got)
	}
	if sub := stripIsolation(got[0].Subtitle); !strings.Contains(sub, "ECB, Nov 6, 2026") || !strings.Contains(sub, "4 days old") {
		t.Errorf("subtitle = %q", sub)
	}
}

func TestTimeAndDateRows(t *testing.T) {
	c := testCalculator(t, nil)
	for q, want := range map[string]string{
		"15:00 PST in Copenhagen": "12:00\u202fAM CET (+1 day)",
		"time in Tokyo":           "7:00\u202fPM JST",
		"today + 90 days":         "Friday Feb 5, 2027",
		"days until 2027-01-01":   "55 days",
		"days until 2026-11-08":   "1 day",
	} {
		got := c.Answer(q)
		if len(got) != 1 || stripIsolation(got[0].Title) != want {
			t.Errorf("%q = %+v, want %q", q, got, want)
		}
	}
	got := c.Answer("time in Tokyo")
	if sub := stripIsolation(got[0].Subtitle); sub != "Saturday Nov 7, 2026 in Tokyo" {
		t.Errorf("subtitle = %q", sub)
	}
	if acts := c.Actions(got[0]); len(acts) < 2 {
		t.Errorf("actions = %d", len(acts))
	}
	for _, q := range []string{"time in Atlantis", "today", "days of our lives"} {
		if got := c.Answer(q); got != nil {
			t.Errorf("%q answered %+v", q, got)
		}
	}
}

func TestATimeRowFollowsTheLocalesHourCycle(t *testing.T) {
	for _, c := range []struct {
		locale, want string
	}{
		{"en-US", "7:00\u202fPM JST"},
		{"de", "19:00 JST"},
		{"fr", "19:00 JST"},
		{"es", "19:00 JST"},
	} {
		got := calculatorIn(t, c.locale, nil).Answer("time in Tokyo")
		if len(got) != 1 || stripIsolation(got[0].Title) != c.want {
			t.Errorf("%s time in Tokyo = %+v, want %q", c.locale, got, c.want)
		}
	}
}

func TestATimeRowContinuesWithTheClockAQueryReads(t *testing.T) {
	for _, locale := range []string{"en-US", "de"} {
		c := calculatorIn(t, locale, nil)
		got := c.Answer("time in Tokyo")
		if len(got) != 1 {
			t.Fatalf("%s time in Tokyo = %+v", locale, got)
		}
		d, ok := got[0].Data.(calcData)
		if !ok || d.value != "19:00" {
			t.Errorf("%s value = %+v, want the 24-hour clock whatever the locale shows", locale, got[0].Data)
		}
	}
}

func TestADateRowShowsTheLocalesDateAndContinuesWithTheISODate(t *testing.T) {
	for _, c := range []struct {
		locale, title string
	}{
		{"en-US", "Friday Feb 5, 2027"},
		{"de", "Freitag 05.02.2027"},
		{"fr", "vendredi 5 févr. 2027"},
	} {
		calc := calculatorIn(t, c.locale, nil)
		got := calc.Answer("today + 90 days")
		if len(got) != 1 {
			t.Fatalf("%s today + 90 days = %+v", c.locale, got)
		}
		if title := stripIsolation(got[0].Title); title != c.title {
			t.Errorf("%s title = %q, want %q", c.locale, title, c.title)
		}
		d, ok := got[0].Data.(calcData)
		if !ok || d.value != "2027-02-05" {
			t.Errorf("%s value = %+v, want the ISO date whatever the locale shows", c.locale, got[0].Data)
		}
		if again := calc.Answer(d.value + " + 1 day"); len(again) != 1 {
			t.Errorf("%s: the continued value %q does not read back as a date", c.locale, d.value)
		}
		if strings.Contains(got[0].Title, "2027-02-05") {
			t.Errorf("%s title %q shows the ISO date", c.locale, got[0].Title)
		}
	}
}

func TestATimeRowsDayAndTheCurrencySourceSpeakTheLocalesDate(t *testing.T) {
	got := calculatorIn(t, "de", nil).Answer("time in Tokyo")
	if len(got) != 1 {
		t.Fatalf("answer = %+v", got)
	}
	if sub := stripIsolation(got[0].Subtitle); !strings.HasPrefix(sub, "Samstag 07.11.2026") || strings.Contains(sub, "2026-11-07") {
		t.Errorf("de subtitle = %q, want the German day and date", sub)
	}
	srv, _ := ecbServer(t, true)
	rates := answer.NewRates(srv.URL, filepath.Join(t.TempDir(), "fx.xml"), func() time.Time { return answerClock })
	rates.Refresh(t.Context())
	got = calculatorIn(t, "fr", rates).Answer("$100 to €")
	if len(got) != 1 {
		t.Fatalf("currency answer = %+v", got)
	}
	if sub := stripIsolation(got[0].Subtitle); !strings.Contains(sub, "6 nov. 2026") || strings.Contains(sub, "2026-11-06") {
		t.Errorf("fr currency source = %q, want the French date", sub)
	}
}

func TestATimeRowSpeaksTheLocalesLanguage(t *testing.T) {
	got := calculatorIn(t, "de", nil).Answer("15:00 PST in Copenhagen")
	if len(got) != 1 {
		t.Fatalf("answer = %+v", got)
	}
	if title := stripIsolation(got[0].Title); strings.Contains(title, "day") || strings.Contains(title, "AM") || !strings.HasPrefix(title, "00:00 CET") {
		t.Errorf("de title = %q, want a 24-hour time and no English", title)
	}
}

func TestACalcRowShowsTheLocalesDigitsAndContinuesWithThePlainOnes(t *testing.T) {
	for _, c := range []struct {
		locale, title string
	}{
		{"en-US", "1,234.5"},
		{"de", "1.234,5"},
		{"fr", "1\u202f234,5"},
		{"es", "1234,5"},
	} {
		calc := calculatorIn(t, c.locale, nil)
		got := calc.Answer("2469/2")
		if len(got) != 1 || got[0].Title != c.title {
			t.Fatalf("%s 2469/2 = %+v, want %q", c.locale, got, c.title)
		}
		d, ok := got[0].Data.(calcData)
		if !ok || d.value != "1234.5" {
			t.Errorf("%s value = %+v, want the plain digits whatever the locale shows", c.locale, got[0].Data)
		}
		if again := calc.Answer(d.value + "*2"); len(again) != 1 || stripIsolation(again[0].Title) != calc.l.Integer(2469) {
			t.Errorf("%s: the continued %q reads back as %+v", c.locale, d.value, again)
		}
	}
}

func TestACurrencyRowShowsTheLocalesDigitsAndContinuesWithThePlainOnes(t *testing.T) {
	srv, _ := ecbServer(t, true)
	rates := answer.NewRates(srv.URL, filepath.Join(t.TempDir(), "fx.xml"), func() time.Time { return answerClock })
	rates.Refresh(t.Context())
	for _, c := range []struct {
		locale, title string
	}{
		{"en-US", "9,237.88 EUR"},
		{"de", "9.237,88 EUR"},
		{"fr", "9\u202f237,88 EUR"},
	} {
		calc := calculatorIn(t, c.locale, rates)
		got := calc.Answer("10000 usd in eur")
		if len(got) != 1 || got[0].Title != c.title {
			t.Fatalf("%s 10000 usd in eur = %+v, want %q", c.locale, got, c.title)
		}
		d, ok := got[0].Data.(calcData)
		if !ok || d.value != "9237.88" {
			t.Errorf("%s value = %+v, want the plain amount whatever the locale shows", c.locale, got[0].Data)
		}
	}
	if got := calculatorIn(t, "de", rates).Answer("100 usd in eur"); len(got) != 1 || got[0].Title != "92,38 EUR" {
		t.Errorf("de 100 usd in eur = %+v, want its cents kept", got)
	}
}
