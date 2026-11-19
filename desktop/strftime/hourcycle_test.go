package strftime

import "testing"

func TestHourCycleReadsHowTheFormatWritesTheHour(t *testing.T) {
	for format, want := range map[string]HourCycle{
		"%a %b %d %H:%M": Hour24,
		"%T":             Hour24,
		"%I:%M %p":       Hour12,
		"%-I:%M":         Hour12,
		"%Y-%m-%d":       NoHour,
		"%%H %%I":        NoHour,
		"%H (%I %p)":     MixedHours,
	} {
		if got := MustCompile(format).HourCycle(); got != want {
			t.Errorf("%q: %v, want %v", format, got, want)
		}
	}
}

func TestWithHourCycleKeepsEverythingButTheHour(t *testing.T) {
	for _, c := range []struct {
		format string
		to     HourCycle
		want   string
	}{
		{"%Y-%m-%d %H:%M", Hour12, "%Y-%m-%d %I:%M %p"},
		{"%Y-%m-%d %I:%M %p", Hour24, "%Y-%m-%d %H:%M"},
		{"%a %d · %-H:%M:%S", Hour12, "%a %d · %-I:%M:%S %p"},
		{"%T on %A", Hour12, "%I:%M:%S %p on %A"},
		{"%p %I:%M", Hour24, "%H:%M"},
		{"%I:%M%p", Hour24, "%H:%M"},
		{"%I:%M %p %%p", Hour24, "%H:%M %%p"},
		{"[%H]", Hour12, "[%I %p]"},
	} {
		if got := MustCompile(c.format).WithHourCycle(c.to).String(); got != c.want {
			t.Errorf("%q to %v: %q, want %q", c.format, c.to, got, c.want)
		}
	}
}

func TestWithHourCycleRoundTripsAFormatItWrote(t *testing.T) {
	for _, format := range []string{"%Y-%m-%d %H:%M", "%a %b %d %H:%M", "%A, %-H:%M:%S"} {
		there := MustCompile(format).WithHourCycle(Hour12)
		if back := there.WithHourCycle(Hour24).String(); back != format {
			t.Errorf("%q to 12 and back: %q", format, back)
		}
	}
}

func TestWithHourCycleLeavesFormatsItCannotConvert(t *testing.T) {
	for format, to := range map[string]HourCycle{
		"%Y-%m-%d":   Hour24,
		"%H (%I %p)": Hour12,
		"%H:%M":      Hour24,
		"%I:%M %p":   NoHour,
	} {
		if got := MustCompile(format).WithHourCycle(to).String(); got != format {
			t.Errorf("%q to %v changed to %q", format, to, got)
		}
	}
}
