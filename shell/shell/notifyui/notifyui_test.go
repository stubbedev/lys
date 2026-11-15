package notifyui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/identity"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"

	"github.com/stubbedev/lys/shell/service/notifications"
)

func TestIconString(t *testing.T) {
	for in, want := range map[string]Icon{
		"file:///usr/share/icon.png":           {File: "/usr/share/icon.png"},
		"/usr/share/icon.png":                  {File: "/usr/share/icon.png"},
		"firefox":                              {Name: "firefox"},
		"/opt/icons/si-gmail-symbolic.svg":     {Name: "si-gmail-symbolic"},
		"file:///x/tb-brand-edge-symbolic.svg": {Name: "tb-brand-edge-symbolic"},
		"/opt/icons/gmail.svg":                 {File: "/opt/icons/gmail.svg"},
		"/opt/icons/si-gmail-symbolic.png":     {File: "/opt/icons/si-gmail-symbolic.png"},
	} {
		got, ok := iconString(in)
		if !ok || got != want {
			t.Errorf("iconString(%q) = %+v %v, want %+v", in, got, ok, want)
		}
	}
	if _, ok := iconString(""); ok {
		t.Error("empty string resolved")
	}
}

func TestResolveIcon(t *testing.T) {
	full := &notifications.Notification{
		AppName: "firefox", AppIcon: "app-icon", ImagePath: "/img.png", DesktopEntry: "org.app",
	}
	mapped := Icon{Name: "si-firefox-symbolic"}
	if got := ResolveIcon(config.IconSourceMapped, full); got != mapped {
		t.Errorf("mapped = %+v", got)
	}
	if got := ResolveIcon(config.IconSourceAutomatic, full); got != (Icon{File: "/img.png"}) {
		t.Errorf("automatic = %+v", got)
	}
	if got := ResolveIcon(config.IconSourceApplication, full); got != (Icon{File: "/img.png"}) {
		t.Errorf("application = %+v", got)
	}
	noImage := *full
	noImage.ImagePath = ""
	if got := ResolveIcon(config.IconSourceAutomatic, &noImage); got != mapped {
		t.Errorf("automatic without image = %+v, want mapped", got)
	}
	if got := ResolveIcon(config.IconSourceApplication, &noImage); got != (Icon{Name: "app-icon"}) {
		t.Errorf("application without image = %+v, want the app icon", got)
	}
	noImage.AppIcon = ""
	if got := ResolveIcon(config.IconSourceApplication, &noImage); got != (Icon{Name: "org.app"}) {
		t.Errorf("application, desktop entry = %+v", got)
	}
	noImage.DesktopEntry = ""
	if got := ResolveIcon(config.IconSourceApplication, &noImage); got != mapped {
		t.Errorf("application, nothing = %+v, want mapped", got)
	}
	unknown := &notifications.Notification{AppName: "zzz-no-such-app"}
	if got := ResolveIcon(config.IconSourceMapped, unknown); got != (Icon{Name: FallbackIcon}) {
		t.Errorf("unknown app = %+v, want the bell", got)
	}
	if got := ResolveIcon(config.IconSourceMapped, &notifications.Notification{}); got != (Icon{Name: FallbackIcon}) {
		t.Errorf("no app = %+v, want the bell", got)
	}
}

func TestUrgency(t *testing.T) {
	low, normal, crit := notifications.UrgencyLow, notifications.UrgencyNormal, notifications.UrgencyCritical
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
		for i, u := range []notifications.Urgency{low, normal, crit} {
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

// fakeMinter answers a token request with token and err.
type fakeMinter struct {
	token string
	err   error
}

func (f fakeMinter) RequestActivationToken(done func(string, error)) { done(f.token, f.err) }

// The press's token reaches the action; without a minter, without
// xdg-activation, or after a failed request the action still runs,
// with no token.
func TestWithActivationToken(t *testing.T) {
	for name, tc := range map[string]struct {
		m    TokenMinter
		want string
	}{
		"minted":        {fakeMinter{token: "tok-1"}, "tok-1"},
		"no minter":     {nil, ""},
		"no activation": {fakeMinter{err: app.ErrActivationUnavailable}, ""},
		"failed":        {fakeMinter{err: errors.New("compositor said no")}, ""},
		"nil app":       {Minter(nil), ""},
	} {
		ran, got := false, "unset"
		WithActivationToken(tc.m, func(token string) { ran, got = true, token })
		if !ran || got != tc.want {
			t.Errorf("%s: ran %v with %q, want %q", name, ran, got, tc.want)
		}
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

func TestGroupBy(t *testing.T) {
	base := time.Unix(1000, 0)
	n := func(id uint32, app string, at int) *notifications.Notification {
		return &notifications.Notification{ID: id, AppName: app, Added: base.Add(time.Duration(at) * time.Second)}
	}
	groups := GroupBy([]*notifications.Notification{n(1, "mail", 1), n(2, "chat", 5), n(3, "mail", 9), n(4, "", 3)}, AppKey)
	var apps []string
	for _, g := range groups {
		apps = append(apps, g.Key)
	}
	if len(apps) != 3 || apps[0] != "mail" || apps[1] != "chat" || apps[2] != "" {
		t.Fatalf("group order = %q, want by newest notification", apps)
	}
	if ids := groups[0].Notifs; ids[0].ID != 3 || ids[1].ID != 1 {
		t.Errorf("mail group = %v, want newest first", ids)
	}
	if len(GroupBy(nil, AppKey)) != 0 {
		t.Error("no notifications made groups")
	}
}

// Who delivered a notification is no heading: notify-send and the shell's
// own name show as the system, under one group.
func TestDeliverersShowAsTheSystem(t *testing.T) {
	system := i18n.T("notification-source-system")
	if system == "" || system == "notification-source-system" {
		t.Fatalf("the system's name is not translated: %q", system)
	}
	for _, name := range []string{"notify-send", identity.Name} {
		if got := AppLabel(name); got != system {
			t.Errorf("AppLabel(%q) = %q, want %q", name, got, system)
		}
	}
	base := time.Unix(1000, 0)
	groups := GroupBy([]*notifications.Notification{
		{ID: 1, AppName: "notify-send", Added: base},
		{ID: 2, AppName: identity.Name, Added: base.Add(time.Second)},
	}, AppKey)
	if len(groups) != 1 || groups[0].Key != system || len(groups[0].Notifs) != 2 {
		t.Errorf("groups %+v, want both under %q", groups, system)
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
