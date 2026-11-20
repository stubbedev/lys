package notifyview

import (
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/notifications/notifymodel"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

func TestUrgency(t *testing.T) {
	low, normal, crit := notifymodel.UrgencyLow, notifymodel.UrgencyNormal, notifymodel.UrgencyCritical
	if UrgencyClass(low) != "low" || UrgencyClass(normal) != "normal" || UrgencyClass(crit) != "critical" {
		t.Error("urgency classes")
	}
	for _, tc := range []struct {
		th   config.UrgencyBarThreshold
		want [3]bool
	}{
		{config.UrgencyBarThresholdNone, [3]bool{false, false, false}},
		{config.UrgencyBarThresholdLow, [3]bool{true, true, true}},
		{config.UrgencyBarThresholdNormal, [3]bool{false, true, true}},
		{config.UrgencyBarThresholdCritical, [3]bool{false, false, true}},
	} {
		for i, u := range []notifymodel.Urgency{low, normal, crit} {
			if got := UrgencyBarVisible(u, tc.th); got != tc.want[i] {
				t.Errorf("UrgencyBarVisible(%v, %s) = %v", u, tc.th, got)
			}
		}
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Unix(100000, 0)
	for _, c := range []struct {
		name string
		ago  time.Duration
		want Age
	}{
		{"30s", 30 * time.Second, Age{Unit: AgeJustNow}},
		{"30m", 30 * time.Minute, Age{Unit: AgeMinutes, Count: 30}},
		{"2.5h", 150 * time.Minute, Age{Unit: AgeHours, Count: 2}},
		{"23h59m", 24*time.Hour - time.Minute, Age{Unit: AgeHours, Count: 23}},
		{"24h", 24 * time.Hour, Age{Unit: AgeDays, Count: 1}},
		{"300h", 300 * time.Hour, Age{Unit: AgeDays, Count: 12}},
		{"future", -time.Hour, Age{Unit: AgeJustNow}},
	} {
		if got := RelativeTime(now, now.Add(-c.ago)); got != c.want {
			t.Errorf("%s = %+v, want %+v", c.name, got, c.want)
		}
	}
}

// Every age reads in the surface's own strings, days included, so a
// notification from last week never shows as hundreds of hours.
func TestTimeLabelReadsEveryUnit(t *testing.T) {
	for _, domain := range []string{"notification-popup", "notification-dropdown"} {
		for _, c := range []struct {
			age  Age
			want string
		}{
			{Age{Unit: AgeJustNow}, i18n.T(domain + "-time-just-now")},
			{Age{Unit: AgeMinutes, Count: 5}, i18n.T(domain+"-time-minutes-ago", i18n.Str("minutes", "5"))},
			{Age{Unit: AgeHours, Count: 3}, i18n.T(domain+"-time-hours-ago", i18n.Str("hours", "3"))},
			{Age{Unit: AgeDays, Count: 12}, i18n.T(domain+"-time-days-ago", i18n.Str("days", "12"))},
		} {
			got := TimeLabel(c.age, domain)
			if got != c.want {
				t.Errorf("%s %+v = %q, want %q", domain, c.age, got, c.want)
			}
			if strings.Contains(got, domain) {
				t.Errorf("%s %+v reads the missing message id %q", domain, c.age, got)
			}
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("an age with no unit label rendered instead of panicking")
		}
	}()
	TimeLabel(Age{Unit: AgeDays + 1}, "notification-popup")
}

func TestBodyMarkup(t *testing.T) {
	for in, want := range map[string]string{
		// Well-formed markup passes through with its tags, the label
		// rendering it; anything else escapes to the literal characters.
		"<b>bold</b> and <i>italic</i>": "<b>bold</b> and <i>italic</i>",
		"a &amp; b &lt;c&gt;":           "a &amp; b &lt;c&gt;",
		"NixOS Package & Module":        "NixOS Package &amp; Module",
		"<b>unclosed":                   "&lt;b&gt;unclosed",
		"plain":                         "plain",
		"":                              "",
	} {
		if got := BodyMarkup(in); got != want {
			t.Errorf("BodyMarkup(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewIcon(t *testing.T) {
	tint := render.RGB(1, 2, 3)
	if w := NewIcon(Icon{Name: "ld-bell-symbolic"}, 16, tint, func(fn func()) { fn() }, nil); w.Name() != "ld-bell-symbolic" || w.Tint() != tint {
		t.Errorf("symbolic: name %q tint %v", w.Name(), w.Tint())
	}
	if w := NewIcon(Icon{Name: "firefox"}, 16, tint, func(fn func()) { fn() }, nil); w.Tint() != 0 {
		t.Error("a full-color theme icon was tinted")
	}
	if w := NewIcon(Icon{File: "/no/such.png"}, 16, tint, func(fn func()) { fn() }, nil); w.Name() != FallbackIcon || w.Tint() != 0 {
		t.Errorf("file icon: name %q tint %v", w.Name(), w.Tint())
	}
}

// An app icon resolves through GTK's dash fallbacks, as a notification
// daemon's GThemedIcon does.
func TestNewIconFallsBackAlongTheName(t *testing.T) {
	treetest.IconTheme(t, "org.example")
	if ink := treetest.Ink(NewIcon(Icon{Name: "org.example-app"}, 24, 0, func(fn func()) { fn() }, nil)); ink == 0 {
		t.Error("an app icon with a present dash-shortened form drew nothing")
	}
	if ink := treetest.Ink(NewIcon(Icon{Name: "missing"}, 24, 0, func(fn func()) { fn() }, nil)); ink != 0 {
		t.Error("an absent icon drew something")
	}
}

// An app's own name is its heading, and no name stays no name for the
// surfaces to call unknown.
func TestOtherAppsKeepTheirNames(t *testing.T) {
	for _, name := range []string{"firefox", "Notify-Send2", "lys-shell-extra", ""} {
		if got := AppLabel(name); got != name {
			t.Errorf("AppLabel(%q) = %q", name, got)
		}
	}
}
