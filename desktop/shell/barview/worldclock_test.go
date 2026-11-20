package barview

import (
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/strftime"
)

func renderWorldClock(format string, now time.Time) (string, error) {
	return NewWorldClock(format, ClockNames(i18n.Shell()), time.LoadLocation).Render(now)
}

func TestWorldClockRender(t *testing.T) {
	now := time.Date(2026, 9, 29, 14, 5, 0, 0, time.UTC)
	render := func(format string) string {
		t.Helper()
		got, err := renderWorldClock(format, now)
		if err != nil {
			t.Errorf("%q: %v", format, err)
		}
		return got
	}
	if got := render("{{ tz('UTC', '%H:%M %Z') }}"); got != "14:05 UTC" {
		t.Errorf("= %q, want 14:05 UTC", got)
	}
	// Two zones in one format.
	got := render("{{ tz('UTC', '%H:%M') }} | {{ tz('Asia/Tokyo', '%H:%M') }}")
	if !strings.Contains(got, "14:05") || !strings.Contains(got, "23:05") {
		t.Errorf("= %q, want UTC 14:05 and Tokyo 23:05", got)
	}
	// Plain text passes through untouched.
	if got := render("plain"); got != "plain" {
		t.Errorf("= %q, want plain", got)
	}
	// Any failure fails the whole render with its error: an unknown or
	// empty zone, a bad layout, a bad expression, a syntax error, wrong
	// arguments.
	for _, bad := range []string{"x{{ tz('Nope/Zone', '%H') }}y", "{{ tz('', '%H') }}", "{{ tz('UTC', '%Q') }}", "x{{ not-a-call }}y", "a{{ b", "{{ tz('UTC') }}", "{{ tz(1, '%H') }}"} {
		if got, err := renderWorldClock(bad, now); err == nil || got != "" {
			t.Errorf("%q = %q (%v), want an error and nothing", bad, got, err)
		}
	}
	// The format is a full template: filters apply to tz() results.
	if got := render("{{ tz('UTC', '%a') | upper }}"); got != "TUE" {
		t.Errorf("filtered = %q, want TUE", got)
	}
}

func TestAWorldClockFailureSaysWhatIsWrong(t *testing.T) {
	now := time.Date(2026, 9, 29, 14, 5, 0, 0, time.UTC)
	for format, want := range map[string]string{
		"{{ tz('Mars/Olympus', '%H') }}": i18n.T("bar-world-clock-invalid-zone", i18n.Str("zone", "Mars/Olympus")),
		"{{ tz('', '%H') }}":             i18n.T("bar-world-clock-invalid-zone", i18n.Str("zone", "")),
		"{{ tz('UTC', '%Q') }}":          i18n.T("bar-world-clock-invalid-layout", i18n.Str("layout", "%Q")),
	} {
		_, err := renderWorldClock(format, now)
		if got := WorldClockFailure(err); got != want {
			t.Errorf("%q fails as %q, want %q", format, got, want)
		}
	}
	_, err := renderWorldClock("{{ tz('UTC') }}", now)
	if got, want := WorldClockFailure(err), i18n.T("bar-world-clock-invalid-format", i18n.Str("error", errTZArguments.Error())); got != want {
		t.Errorf("wrong arguments fail as %q, want %q", got, want)
	}
}

func TestWorldClockResolutionIsTheFinestLayoutUsed(t *testing.T) {
	for format, want := range map[string]time.Duration{
		"{{ tz('UTC', '%H:%M %Z') }}": time.Minute,
		"plain":                       time.Minute,
		"{{ tz('UTC', '%H:%M') }} {{ tz('Asia/Tokyo', '%H:%M:%S') }}": time.Second,
		"{{ tz('Not/AZone', '%S') }}":                                 time.Minute,
	} {
		if got := worldClockFinest(format); got != want {
			t.Errorf("%q: resolution %v, want %v", format, got, want)
		}
	}
}

func worldClockFinest(format string) time.Duration {
	template := NewWorldClock(format, strftime.English, time.LoadLocation)
	_, _ = template.Render(time.Now())
	return template.Finest()
}

func TestWorldClockLoadsEachZoneOnceAndRendersWithinItsBudget(t *testing.T) {
	loads := map[string]int{}
	template := NewWorldClock("{{ tz('UTC', '%H:%M') }} {{ tz('Asia/Tokyo', '%H:%M') }} {{ tz('UTC', '%H') }}", strftime.English, func(zone string) (*time.Location, error) {
		loads[zone]++
		return time.LoadLocation(zone)
	})
	now := time.Date(2026, 9, 29, 14, 5, 0, 0, time.UTC)
	if _, err := template.Render(now); err != nil {
		t.Fatal(err)
	}
	allocs := testing.AllocsPerRun(50, func() { _, _ = template.Render(now) })
	for zone, n := range loads {
		if n != 1 {
			t.Errorf("%s loaded %d times, want once", zone, n)
		}
	}
	if len(loads) != 2 {
		t.Errorf("loaded %v, want the two zones", loads)
	}
	if raceEnabled {
		return
	}
	const budget = 120
	t.Logf("a render makes %.0f allocations", allocs)
	if allocs > budget {
		t.Errorf("a render makes %.0f allocations, want at most %d", allocs, budget)
	}
}

func TestTheWorldClockLocalZoneIsTheWallClocks(t *testing.T) {
	now := time.Date(2026, 3, 5, 9, 15, 0, 0, time.FixedZone("XST", 5*3600+1800))
	got, err := renderWorldClock("{{ tz('Local', '%H:%M %Z') }} {{ tz('UTC', '%H:%M') }}", now)
	if err != nil {
		t.Fatal(err)
	}
	if want := "09:15 XST 03:45"; got != want {
		t.Errorf("the world clock renders %q, want %q", got, want)
	}
}

func TestTheWorldClockPlaceholderRendersItsSample(t *testing.T) {
	ps := config.Placeholders("modules.world-clock.format")
	if len(ps) != 1 {
		t.Fatalf("world clock placeholders = %+v", ps)
	}
	if got, err := renderWorldClock(ps[0].Token, time.Date(2026, 3, 5, 14, 30, 0, 0, time.UTC)); err != nil || got != ps[0].Sample {
		t.Errorf("%s rendered %q (%v), want the sample %q", ps[0].Token, got, err, ps[0].Sample)
	}
}
