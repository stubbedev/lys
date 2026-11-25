package i18n

import (
	"strings"
	"testing"
	"time"

	shelllocales "github.com/stubbedev/lys/desktop/i18n/locales/shell"
)

func shellLoader(t *testing.T, requested ...string) *Loader {
	t.Helper()
	var ids []LangID
	for _, r := range requested {
		ids = append(ids, MustLangID(r))
	}
	l, err := Select(NewAssets(shelllocales.FS), ids)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestIntegerGroupsDigitsByLocale(t *testing.T) {
	for _, c := range []struct {
		locale string
		n      int64
		want   string
	}{
		{"en-US", 10_000, "10,000"},
		{"en-US", 1_000, "1,000"},
		{"en-US", 999, "999"},
		{"en-US", -1_234_567, "-1,234,567"},
		{"de", 10_000, "10.000"},
		{"de", 1_000, "1.000"},
		{"fr", 10_000, "10 000"},
		{"fr", 1_234_567, "1 234 567"},
		{"es", 1_000, "1000"},
		{"es", 10_000, "10.000"},
		{"pt", 1_000, "1000"},
		{"pt", 10_000, "10 000"},
		{"it", 1_000, "1000"},
		{"it", 10_000, "10.000"},
		{"nl", 1_000, "1.000"},
		{"nl", 10_000, "10.000"},
	} {
		if got := shellLoader(t, c.locale).Integer(c.n); got != c.want {
			t.Errorf("%s Integer(%d) = %q, want %q", c.locale, c.n, got, c.want)
		}
	}
}

func TestByteSizeLocalizesNumberAndUnit(t *testing.T) {
	for _, c := range []struct {
		locale string
		n      uint64
		want   string
	}{
		{"en-US", 0, "0 B"},
		{"en-US", 512, "512 B"},
		{"en-US", 1023, "1,023 B"},
		{"en-US", 1024, "1.0 KiB"},
		{"en-US", 1536, "1.5 KiB"},
		{"en-US", 3 << 30, "3.0 GiB"},
		{"en-US", 1 << 63, "8.0 EiB"},
		{"de", 1536, "1,5 KiB"},
		{"de", 1023, "1.023 B"},
		{"de", 1500 << 20, "1,5 GiB"},
		{"fr", 512, "512 o"},
		{"fr", 1536, "1,5 Kio"},
		{"fr", 5 << 20, "5,0 Mio"},
		{"es", 1023, "1023 B"},
	} {
		if got := shellLoader(t, c.locale).ByteSize(c.n); got != c.want {
			t.Errorf("%s ByteSize(%d) = %q, want %q", c.locale, c.n, got, c.want)
		}
	}
}

func TestAByteSizeAboutToRoundUpStepsToTheNextUnit(t *testing.T) {
	for _, c := range []struct {
		n    uint64
		want string
	}{
		{1<<20 - 1, "1.0 MiB"},
		{1<<20 - 60, "1,023.9 KiB"},
		{1<<30 - 1, "1.0 GiB"},
	} {
		if got := shellLoader(t, "en-US").ByteSize(c.n); got != c.want {
			t.Errorf("ByteSize(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestDecimalByteSizeCountsInThousandsAndLocalizes(t *testing.T) {
	for _, c := range []struct {
		locale string
		n      uint64
		want   string
	}{
		{"en-US", 0, "0 B"},
		{"en-US", 999, "999 B"},
		{"en-US", 1000, "1.0 kB"},
		{"en-US", 1024, "1.0 kB"},
		{"en-US", 1500, "1.5 kB"},
		{"en-US", 999_999, "1.0 MB"},
		{"en-US", 16_000_000_000, "16.0 GB"},
		{"en-US", 500_107_862_016, "500.1 GB"},
		{"en-US", 2_000_000_000_000, "2.0 TB"},
		{"en-US", 1 << 63, "9.2 EB"},
		{"de", 500_107_862_016, "500,1 GB"},
		{"de", 999, "999 B"},
		{"fr", 500_107_862_016, "500,1\u00a0Go"},
		{"fr", 1500, "1,5\u00a0ko"},
		{"fr", 512, "512\u00a0o"},
		{"nl", 4_700_000_000, "4,7 GB"},
	} {
		if got := shellLoader(t, c.locale).DecimalByteSize(c.n); got != c.want {
			t.Errorf("%s DecimalByteSize(%d) = %q, want %q", c.locale, c.n, got, c.want)
		}
	}
}

func TestBinaryAndDecimalSizesDisagreeAboveAThousand(t *testing.T) {
	l := shellLoader(t, "en-US")
	if binary, decimal := l.ByteSize(1_000_000), l.DecimalByteSize(1_000_000); binary != "976.6 KiB" || decimal != "1.0 MB" {
		t.Errorf("a million bytes = %q binary, %q decimal, want 976.6 KiB and 1.0 MB", binary, decimal)
	}
	if binary, decimal := l.ByteSize(999), l.DecimalByteSize(999); binary != decimal {
		t.Errorf("999 bytes = %q binary, %q decimal, want the same plain bytes", binary, decimal)
	}
}

func TestDateTimeFollowsTheLocalePatterns(t *testing.T) {
	morning := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	evening := time.Date(2026, 11, 24, 17, 30, 0, 0, time.UTC)
	midnight := time.Date(2026, 1, 1, 0, 5, 0, 0, time.UTC)
	for _, c := range []struct {
		locale string
		at     time.Time
		want   string
	}{
		{"en-US", morning, "3/4/2026, 5:06 AM"},
		{"en-US", evening, "11/24/2026, 5:30 PM"},
		{"en-US", midnight, "1/1/2026, 12:05 AM"},
		{"de", morning, "4.3.2026, 05:06"},
		{"de", evening, "24.11.2026, 17:30"},
		{"fr", morning, "04/03/2026 05:06"},
		{"fr", evening, "24/11/2026 17:30"},
		{"es", morning, "4/3/2026, 5:06"},
		{"it", morning, "04/03/2026, 05:06"},
		{"nl", morning, "4-3-2026, 05:06"},
		{"pt", morning, "04/03/2026, 05:06"},
	} {
		if got := shellLoader(t, c.locale).DateTime(c.at); got != c.want {
			t.Errorf("%s DateTime(%v) = %q, want %q", c.locale, c.at, got, c.want)
		}
	}
}

func TestDateIsTheLocalesMediumDate(t *testing.T) {
	early := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	late := time.Date(2026, 11, 24, 23, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		locale string
		at     time.Time
		want   string
	}{
		{"en-US", early, "Mar 4, 2026"},
		{"en-US", late, "Nov 24, 2026"},
		{"de", early, "04.03.2026"},
		{"de", late, "24.11.2026"},
		{"es", early, "4 mar 2026"},
		{"es", time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC), "4 sept 2026"},
		{"fr", early, "4 mars 2026"},
		{"fr", late, "24 nov. 2026"},
		{"it", early, "4 mar 2026"},
		{"it", time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC), "4 gen 2026"},
		{"nl", early, "4 mrt 2026"},
		{"pt", early, "4 de mar. de 2026"},
		{"pt", time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), "31 de dez. de 2026"},
	} {
		if got := shellLoader(t, c.locale).Date(c.at); got != c.want {
			t.Errorf("%s Date(%v) = %q, want %q", c.locale, c.at, got, c.want)
		}
	}
}

func TestDateNamesEveryMonthApartAndKeepsTheTimesDay(t *testing.T) {
	for _, locale := range parseLangs(t, shipped) {
		l := shellLoader(t, locale.String())
		seen := map[string]time.Month{}
		for m := time.January; m <= time.December; m++ {
			got := l.Date(time.Date(2026, m, 15, 12, 0, 0, 0, time.UTC))
			if prev, dup := seen[got]; dup {
				t.Errorf("%s: %s and %s both read %q", locale, prev, m, got)
			}
			seen[got] = m
			if !strings.Contains(got, "15") || !strings.Contains(got, "2026") {
				t.Errorf("%s: %s 15 reads %q, want its day and year", locale, m, got)
			}
		}
	}
	zone := time.FixedZone("UTC+9", 9*60*60)
	at := time.Date(2026, 3, 4, 20, 0, 0, 0, time.UTC).In(zone)
	if got := shellLoader(t, "en-US").Date(at); got != "Mar 5, 2026" {
		t.Errorf("Date in UTC+9 = %q, want the zone's own day", got)
	}
}

func TestTimeFollowsTheLocaleHourCycle(t *testing.T) {
	at := time.Date(2026, 3, 4, 17, 6, 0, 0, time.UTC)
	for locale, want := range map[string]string{"en-US": "5:06 PM", "de": "17:06", "fr": "17:06"} {
		if got := shellLoader(t, locale).Time(at); got != want {
			t.Errorf("%s Time = %q, want %q", locale, got, want)
		}
	}
}

func TestDateTimeKeepsTheTimesLocation(t *testing.T) {
	zone := time.FixedZone("UTC+9", 9*60*60)
	at := time.Date(2026, 3, 4, 20, 0, 0, 0, time.UTC).In(zone)
	if got := shellLoader(t, "de").DateTime(at); got != "5.3.2026, 05:00" {
		t.Errorf("DateTime in UTC+9 = %q, want the zone's own day and hour", got)
	}
}

func TestFormatsFollowTheMostPreferredCatalogue(t *testing.T) {
	if got := shellLoader(t, "de-AT", "fr").Integer(10_000); got != "10.000" {
		t.Errorf("de-AT before fr formats %q, want de's", got)
	}
	if got := shellLoader(t, "pt-BR").ByteSize(1536); got != "1,5 KiB" {
		t.Errorf("pt-BR formats %q, want pt's", got)
	}
}

func TestAnUnservedLocaleFormatsAsTheFallback(t *testing.T) {
	for _, requested := range [][]string{{"ja"}, nil} {
		l := shellLoader(t, requested...)
		if got := l.Integer(10_000); got != "10,000" {
			t.Errorf("%v: Integer = %q, want en-US's", requested, got)
		}
		if got := l.ByteSize(1536); got != "1.5 KiB" {
			t.Errorf("%v: ByteSize = %q, want en-US's", requested, got)
		}
		if got := l.DecimalByteSize(1500); got != "1.5 kB" {
			t.Errorf("%v: DecimalByteSize = %q, want en-US's", requested, got)
		}
	}
}

func TestEveryCatalogueHasItsFormats(t *testing.T) {
	for _, locale := range parseLangs(t, shipped) {
		if _, err := formatsFor(locale); err != nil {
			t.Error(err)
		}
	}
	for _, unknown := range []string{"pt-BR", "de-AT", "en", "ja"} {
		_, err := formatsFor(MustLangID(unknown))
		if err == nil || !strings.Contains(err.Error(), unknown) {
			t.Errorf("%s: err = %v, want one naming it", unknown, err)
		}
	}
}
