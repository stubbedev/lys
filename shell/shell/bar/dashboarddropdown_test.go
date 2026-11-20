package bar

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/bluetooth"
	"github.com/stubbedev/lys/desktop/service/idleinhibit/inhibitduration"
	"github.com/stubbedev/lys/desktop/service/mpris"
	"github.com/stubbedev/lys/desktop/service/powerprofiles"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/desktop/service/upower"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/notifytest"
	"github.com/stubbedev/lys/shell/service/idleinhibit"
)

func TestDashboardBatteryHelpers(t *testing.T) {
	for _, tc := range []struct {
		pct      float64
		charging bool
		want     string
	}{
		{90, false, "ld-battery-full-symbolic"},
		{75, false, "ld-battery-medium-symbolic"},
		{51, false, "ld-battery-medium-symbolic"},
		{50, false, "ld-battery-low-symbolic"},
		{25, false, "ld-battery-warning-symbolic"},
		{10, true, "ld-battery-charging-symbolic"},
	} {
		if got := dashboardBatteryIcon(tc.pct, tc.charging); got != tc.want {
			t.Errorf("%v%% charging=%v = %q, want %q", tc.pct, tc.charging, got, tc.want)
		}
	}
	held := upower.Device{ChargeThresholdSupported: true, ChargeThresholdEnabled: true, ChargeEndThreshold: 80}
	if got, want := dashboardChargeLimit(held), i18n.T("dropdown-battery-limit-to", i18n.Str("threshold", "80")); got != want {
		t.Errorf("a limit in force reads %q, want %q", got, want)
	}
	for _, dev := range []upower.Device{
		{ChargeThresholdSupported: true, ChargeEndThreshold: 80},
		{ChargeThresholdEnabled: true, ChargeEndThreshold: 80},
	} {
		if got := dashboardChargeLimit(dev); got != "" {
			t.Errorf("no limit in force (supported %v, enabled %v) reads %q", dev.ChargeThresholdSupported, dev.ChargeThresholdEnabled, got)
		}
	}
}

func TestNextAfterSwitchesToTheFollowingItem(t *testing.T) {
	is := func(want string) func(string) bool { return func(s string) bool { return s == want } }
	for _, c := range []struct {
		items   []string
		current string
		want    string
		ok      bool
	}{
		{[]string{"a", "b", "c"}, "a", "b", true},
		{[]string{"a", "b", "c"}, "c", "a", true},
		{[]string{"a", "b"}, "gone", "a", true},
		{[]string{"a"}, "a", "", false},
		{nil, "a", "", false},
	} {
		if got, ok := nextAfter(c.items, is(c.current)); got != c.want || ok != c.ok {
			t.Errorf("nextAfter(%v, %q) = %q, %v; want %q, %v", c.items, c.current, got, ok, c.want, c.ok)
		}
	}
}

func TestDashboardSpeedAndThresholds(t *testing.T) {
	if v, mega := dashboardSpeed(512); v != "0.5" || mega {
		t.Errorf("512 B/s = %q %v", v, mega)
	}
	if v, mega := dashboardSpeed(1536 * 1024); v != "1.5" || !mega {
		t.Errorf("1.5 MiB/s = %q %v", v, mega)
	}
	cfg := config.DashboardConfig{TempWarning: 70, TempError: 90}
	token := func(c *config.ColorValue) config.CssToken {
		if c == nil {
			return ""
		}
		return c.Token
	}
	if token(dashboardTempColor(95, cfg)) != config.TokenStatusError ||
		token(dashboardTempColor(70, cfg)) != config.TokenStatusWarning ||
		dashboardTempColor(69.9, cfg) != nil {
		t.Error("temperature colors")
	}
}

func TestTheDashboardBatteryAndTheBarModuleShareOneThresholdSet(t *testing.T) {
	cfg := config.Defaults()
	blue := config.ColorValue{Kind: config.ColorToken, Token: config.TokenBlue}
	cfg.Thresholds.Battery = []config.ThresholdEntry{{Below: ptrF(50), IconColor: &blue}}
	device := upower.Device{Percentage: 40, State: upower.StateDischarging, IsPresent: true}
	ctx := newTestContext(t, cfg)
	ctx.Battery = newFakeBattery(device)
	v := noRadiosDashboard(ctx).(*dashboardView)
	var icon *widget.Icon
	var percent *widget.Label
	walkTree(v, func(w widget.Widget) bool {
		switch w := w.(type) {
		case *widget.Icon:
			if w.HasClass("battery-icon") {
				icon = w
			}
		case *widget.Label:
			if w.HasClass("battery-percent") {
				percent = w
			}
		}
		return true
	})
	if icon == nil || percent == nil {
		t.Fatalf("battery card icon %v, percent %v", icon, percent)
	}
	if got := icon.InlineStyle(); got != "color: var(--blue);" {
		t.Errorf("the dashboard's battery icon = %q, want the shared threshold's blue", got)
	}
	if got := percent.InlineStyle(); got != "" {
		t.Errorf("an entry without label-color colored the percent %q", got)
	}
	m := &battery{ctx: ctx, source: ctx.Battery}
	m.icon = barview.Icon(cfg.Battery.Icon())
	m.btn = newBarButton(ctx, m.icon, "")
	m.apply(device)
	if got := m.btn.Thresholds().IconColor; got == nil || *got != blue {
		t.Errorf("the bar module's icon override = %+v, want the shared threshold's blue", got)
	}
	m.apply(upower.Device{Percentage: 60, State: upower.StateDischarging, IsPresent: true})
	if !m.btn.Thresholds().IsEmpty() {
		t.Errorf("above the shared threshold the bar module still overrides %+v", m.btn.Thresholds())
	}
}

// The dashboard paints from the stylesheet: the battery gauge and its
// status take the battery thresholds' colors as inline declarations
// instead of fills and tints, and the quick-action icons follow the
// cascade.
func TestDashboardPaintsFromTheStylesheet(t *testing.T) {
	cfg := config.Defaults()
	ctx := styledContext(t, cfg)
	ctx.Media = newFakeMedia(mpris.Player{BusName: "org.mpris.MediaPlayer2.x", Title: "Song", Artist: "Band", State: mpris.StatePlaying})
	ctx.Battery = newFakeBattery(upower.Device{Percentage: 8, State: upower.StateDischarging, IsPresent: true, TimeToEmpty: 20 * time.Minute})
	v := noRadiosDashboard(ctx).(*dashboardView)

	var gauge *widget.ProgressBar
	var percent *widget.Label
	walkTree(v, func(w widget.Widget) bool {
		switch w := w.(type) {
		case *widget.ProgressBar:
			if w.HasClass("progress-bar") {
				gauge = w
			}
		case *widget.Label:
			if w.HasClass("battery-percent") {
				percent = w
			}
		}
		return true
	})
	if gauge == nil || percent == nil {
		t.Fatalf("the battery card = gauge %v, percent %v; want a classed gauge and percent", gauge, percent)
	}
	if gauge.Fill != 0 {
		t.Errorf("the gauge carries a fill %#08x", uint32(gauge.Fill))
	}
	if got := gauge.InlineStyle(); got != "accent-color: var(--status-error);" || gauge.HasClass("success") {
		t.Errorf("a critical battery's gauge = %q, success %v", got, gauge.HasClass("success"))
	}
	if got := percent.Color(); got != 0 {
		t.Errorf("the percent carries a programmatic color %#08x", uint32(got))
	}
	if got := percent.InlineStyle(); got != "color: var(--status-error);" {
		t.Errorf("a critical battery's percent = %q", got)
	}

	// The quick-action icons have no tint: .quick-action-icon image
	// inks them, .active inverts to fg-on-accent.
	walkTree(v, func(w widget.Widget) bool {
		if b, ok := w.(*widget.Button); ok && b.HasClass("quick-action") {
			walkTree(b, func(c widget.Widget) bool {
				if icon, ok := c.(*widget.Icon); ok && icon.Tint() != 0 {
					t.Errorf("a quick-action icon carries a tint %#08x", uint32(icon.Tint()))
				}
				return true
			})
		}
		return true
	})

	// No network: the speeds read muted through the class, uncolored.
	walkTree(v, func(w widget.Widget) bool {
		if l, ok := w.(*widget.Label); ok && l.HasClass("speed-value") {
			if got := l.Color(); got != 0 || !l.HasClass("muted") {
				t.Errorf("a disconnected speed = color %#08x muted %v", uint32(l.Color()), l.HasClass("muted"))
			}
		}
		return true
	})
}

func TestDashboardQuickActionsDriveTheServices(t *testing.T) {
	cfg := config.Defaults()
	ctx := newTestContext(t, cfg)
	ctx.Notifications = notifytest.NewService(t)
	ctx.Notifications.SetDND(false)
	ctx.IdleInhibit = idleinhibit.NewState(inhibitduration.Duration{})
	ctx.Media = newFakeMedia(mpris.Player{BusName: "org.mpris.MediaPlayer2.x", Title: "Song", Artist: "Band", State: mpris.StatePlaying, CanGoNext: true})
	v := noRadiosDashboard(ctx).(*dashboardView)
	if len(v.refreshers) == 0 {
		t.Fatal("no sections refresh")
	}
	// The quick actions in grid order: wifi, bluetooth, airplane, dnd,
	// idle inhibit, power saver.
	var actions []*widget.Button
	walkTree(v, func(w widget.Widget) bool {
		if b, ok := w.(*widget.Button); ok && b.HasClass("quick-action") {
			actions = append(actions, b)
		}
		return true
	})
	if len(actions) != 6 {
		t.Fatalf("quick actions = %d, want 6", len(actions))
	}
	wifi, dnd, idle := actions[0], actions[3], actions[4]
	dnd.OnClick()
	if !ctx.Notifications.DND() {
		t.Error("the DND toggle did not reach the service")
	}
	dnd.OnClick()
	if ctx.Notifications.DND() {
		t.Error("a second press before the dashboard refreshed left DND on")
	}
	dnd.OnClick()
	if !ctx.Notifications.DND() {
		t.Error("a third press before the dashboard refreshed left DND off")
	}
	v.refreshAll()
	if !dnd.HasClass("active") {
		t.Error("DND on did not mark its button active")
	}
	idle.OnClick()
	if !ctx.IdleInhibit.Active() {
		t.Error("the idle toggle did not enable inhibition")
	}
	// No wifi device: the wifi toggle is insensitive.
	if wifi.Enabled() {
		t.Error("wifi without a device is clickable")
	}
}

// walkTree visits w and its descendants depth-first while visit
// returns true.
func walkTree(w widget.Widget, visit func(widget.Widget) bool) {
	if w == nil || !visit(w) {
		return
	}
	if p, ok := w.(interface{ Children() []widget.Widget }); ok {
		for _, c := range p.Children() {
			walkTree(c, visit)
		}
	}
}

func TestDashboardMediaAndBatterySections(t *testing.T) {
	cfg := config.Defaults()
	ctx := newTestContext(t, cfg)
	ctx.Media = newFakeMedia(mpris.Player{BusName: "org.mpris.MediaPlayer2.x", Title: "Song", Artist: "Band", State: mpris.StatePlaying})
	ctx.Battery = newFakeBattery(upower.Device{Percentage: 8, State: upower.StateDischarging, IsPresent: true, TimeToEmpty: 20 * time.Minute})
	v := noRadiosDashboard(ctx).(*dashboardView)
	texts := map[string]bool{}
	walkTree(v, func(w widget.Widget) bool {
		if l, ok := w.(*widget.Label); ok {
			texts[l.Text()] = true
		}
		return true
	})
	dev := upower.Device{Percentage: 8, State: upower.StateDischarging, TimeToEmpty: 20 * time.Minute}
	for _, want := range []string{"Song", "Band", "8%", batteryTimeDisplay(dev), batteryStateLabel(dev), i18n.T("dropdown-dashboard-now-playing")} {
		if !texts[want] {
			t.Errorf("dashboard is missing %q", want)
		}
	}
}

func mediaCard(t *testing.T, v *dashboardView) *widget.Box {
	t.Helper()
	return dashboardCardHeaded(t, v, i18n.T("dropdown-dashboard-now-playing"))
}

func dashboardCardHeaded(t *testing.T, v *dashboardView, heading string) *widget.Box {
	t.Helper()
	var found *widget.Box
	walkTree(v, func(w widget.Widget) bool {
		b, ok := w.(*widget.Box)
		if !ok || !b.HasClass("dashboard-card") {
			return true
		}
		walkTree(b, func(c widget.Widget) bool {
			if l, ok := c.(*widget.Label); ok && l.Text() == heading {
				found = b
			}
			return true
		})
		return found == nil
	})
	if found == nil {
		t.Fatalf("no dashboard card headed %q", heading)
	}
	return found
}

func TestTheMediaCardShowsOnlyWhileAPlayerIsActive(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	media := newFakeMedia()
	ctx.Media = media
	v := noRadiosDashboard(ctx).(*dashboardView)
	if mediaCard(t, v).Visible() {
		t.Error("the media card takes room with nothing playing")
	}
	set := func(players ...mpris.Player) {
		fresh := newFakeMedia(players...)
		media.mu.Lock()
		media.players, media.active = fresh.players, fresh.active
		media.mu.Unlock()
	}
	set(mpris.Player{BusName: "org.mpris.MediaPlayer2.x", Title: "Song", State: mpris.StatePlaying})
	v.refreshAll()
	if !mediaCard(t, v).Visible() {
		t.Error("the media card stays hidden while a player is active")
	}
	set()
	v.refreshAll()
	if mediaCard(t, v).Visible() {
		t.Error("the media card stays once the player is gone")
	}
	ctx.Media = nil
	if mediaCard(t, noRadiosDashboard(ctx).(*dashboardView)).Visible() {
		t.Error("the media card shows without a media service")
	}
}

// The content box pads outside the scroll: .dropdown-content >
// .dashboard-scroll > .dashboard-content-box, with the cards in the
// inner box.
func TestDashboardContentPadsOutsideTheScroll(t *testing.T) {
	cfg := config.Defaults()
	ctx := newTestContext(t, cfg)
	ctx.Media = newFakeMedia(mpris.Player{BusName: "org.mpris.MediaPlayer2.x", Title: "Song", Artist: "Band", State: mpris.StatePlaying})
	v := noRadiosDashboard(ctx).(*dashboardView)
	content, ok := findByClass(v, "dropdown-content").(*widget.Box)
	if !ok {
		t.Fatal("no dropdown-content")
	}
	// Negative: the padded box carries nothing but the scroll — the
	// cards ride inside it.
	if got := len(content.Children()); got != 1 {
		t.Fatalf("dropdown-content children = %d, want only the scroll", got)
	}
	scroll, isScroll := content.Children()[0].(*widget.Scroll)
	if !isScroll || !scroll.HasClass("dashboard-scroll") {
		t.Fatalf("dropdown-content's child = %T, want the dashboard-scroll", content.Children()[0])
	}
	kids := scroll.Children()
	inner, isBox := kids[0].(*widget.Box)
	if len(kids) != 1 || !isBox || !inner.HasClass("dashboard-content-box") {
		t.Fatalf("the scroll wraps %v, want the dashboard-content-box", kids)
	}
	// Negative: the inner box is not the padding box itself.
	if inner.HasClass("dropdown-content") {
		t.Error("the content box rode inside the scroll")
	}
	if findByClass(inner, "quick-actions") == nil {
		t.Error("the quick actions did not land in the inner content box")
	}
	// The cards sit the theme's spacing apart.
	if got := inner.Spacing(); got != widget.Current().Spacing {
		t.Errorf("dashboard-content-box spacing = %d, want the theme's %d", got, widget.Current().Spacing)
	}
}

// The volume card is always in the tree with its no-device state
// inside; the media card is there to show once a player is.
func TestDashboardAlwaysHoldsTheVolumeAndMediaCards(t *testing.T) {
	cfg := config.Defaults()
	ctx := newTestContext(t, cfg) // no Pulse, no Media
	v := noRadiosDashboard(ctx).(*dashboardView)
	var volumeCard, mediaCard *widget.Box
	walkTree(v, func(w widget.Widget) bool {
		if b, ok := w.(*widget.Box); ok && b.HasClass("dashboard-card") {
			var volume, media bool
			walkTree(b, func(c widget.Widget) bool {
				if l, ok := c.(*widget.Label); ok {
					switch l.Text() {
					case i18n.T("dropdown-dashboard-volume"):
						volume = true
					case i18n.T("dropdown-dashboard-now-playing"):
						media = true
					}
				}
				return true
			})
			if volume {
				volumeCard = b
			}
			if media {
				mediaCard = b
			}
		}
		return true
	})
	if volumeCard == nil || mediaCard == nil {
		t.Fatalf("without services the cards are missing: volume %v, media %v", volumeCard, mediaCard)
	}
	// No device: the slider row is disabled over the no-device label.
	row := classedBox(t, volumeCard, "dashboard-slider-row")
	if row.Enabled() {
		t.Error("the volume row is live without a device")
	}
	if l := classedBox(t, volumeCard, "dashboard-controls").Children(); len(l) != 2 {
		t.Errorf("dashboard-controls children = %d, want the row and the device row", len(l))
	}
	// Negative: the slider row is wrapped, not a bare card child.
	for _, c := range volumeCard.Children() {
		if b, ok := c.(*widget.Box); ok && b.HasClass("dashboard-slider-row") {
			t.Error("the slider row hangs off the bare card")
		}
	}
	if mediaCard.Visible() {
		t.Error("no player: the media card shows")
	}
	// Positive control: with services the cards drive them.
	ctx.Pulse = &fakePulseSource{dev: pulse.Device{Name: "out", Description: "Speakers", Volume: pct(42)}, ticks: make(chan struct{}, 2)}
	ctx.Media = newFakeMedia(mpris.Player{BusName: "org.mpris.MediaPlayer2.x", Title: "Song", Artist: "Band", State: mpris.StatePlaying})
	v = noRadiosDashboard(ctx).(*dashboardView)
	v.refreshAll()
	walkTree(v, func(w widget.Widget) bool {
		if b, ok := w.(*widget.Box); ok && b.HasClass("dashboard-slider-row") && b.Enabled() {
			volumeCard = b
		}
		return true
	})
	if volumeCard == nil {
		t.Error("with a sink the volume row stayed disabled")
	}
}

// battery-detail takes the thresholds' label color like the percent.
func TestDashboardBatteryDetailTakesTheThresholdColor(t *testing.T) {
	build := func(t *testing.T, pct float64) *widget.Label {
		t.Helper()
		cfg := config.Defaults()
		ctx := newTestContext(t, cfg)
		ctx.Battery = newFakeBattery(upower.Device{Percentage: pct, State: upower.StateDischarging, IsPresent: true, TimeToEmpty: 20 * time.Minute})
		v := noRadiosDashboard(ctx).(*dashboardView)
		var detail *widget.Label
		walkTree(v, func(w widget.Widget) bool {
			if l, ok := w.(*widget.Label); ok && l.HasClass("battery-detail") {
				detail = l
			}
			return true
		})
		if detail == nil {
			t.Fatal("no battery-detail label")
		}
		return detail
	}
	if got := build(t, 8).InlineStyle(); got != "color: var(--status-error);" {
		t.Errorf("a critical battery's detail = %q", got)
	}
	if got := build(t, 25).InlineStyle(); got != "color: var(--status-warning);" {
		t.Errorf("a low battery's detail = %q", got)
	}
	if got := build(t, 80).InlineStyle(); got != "" {
		t.Errorf("a healthy battery's detail = %q", got)
	}
}

// The art carries the per-instance class, and the track labels cap
// their natural width.
func TestDashboardMediaArtAndLabels(t *testing.T) {
	cfg := config.Defaults()
	ctx := newTestContext(t, cfg)
	ctx.Media = newFakeMedia(mpris.Player{BusName: "org.mpris.MediaPlayer2.x", Title: "A Very Long Song Title That Should Cap", Artist: "Band", State: mpris.StatePlaying})
	v := noRadiosDashboard(ctx).(*dashboardView)
	var artClasses []string
	walkTree(v, func(w widget.Widget) bool {
		if h, ok := w.(interface {
			HasClass(string) bool
			Classes() []string
		}); ok && h.HasClass("dashboard-media-art") {
			artClasses = h.Classes()
		}
		return true
	})
	if artClasses == nil {
		t.Fatal("no dashboard-media-art box")
	}
	instanced := false
	for _, c := range artClasses {
		if strings.HasPrefix(c, "dashboard-media-art-instance-") {
			instanced = true
		}
	}
	if !instanced {
		t.Errorf("the art carries classes %v, want a dashboard-media-art-instance-N", artClasses)
	}
	var track, artist, aTime *widget.Label
	walkTree(v, func(w widget.Widget) bool {
		if l, ok := w.(*widget.Label); ok {
			switch {
			case l.HasClass("media-track"):
				track = l
			case l.HasClass("media-artist"):
				artist = l
			case l.HasClass("media-time"):
				aTime = l
			}
		}
		return true
	})
	for _, tc := range []struct {
		what  string
		label *widget.Label
	}{
		{"media-track", track},
		{"media-artist", artist},
	} {
		if tc.label == nil {
			t.Fatalf("no %s label", tc.what)
		}
		if tc.label.MaxWidthChars() != 1 {
			t.Errorf("%s max width chars = %d, want the 1-char cap", tc.what, tc.label.MaxWidthChars())
		}
		if tc.label.Alignment() != render.AlignStart {
			t.Errorf("%s alignment = %v, want xalign 0", tc.what, tc.label.Alignment())
		}
	}
	// Negative: only the track labels cap — the times keep natural
	// width.
	if aTime != nil && aTime.MaxWidthChars() != 0 {
		t.Error("a media-time label carries the width cap")
	}
}

// A quick action's tile is its icon on a square tile over the label,
// and the grid sits its tiles the theme's spacing apart.
func TestDashboardQuickActionTiles(t *testing.T) {
	cfg := config.Defaults()
	ctx := newTestContext(t, cfg)
	v := noRadiosDashboard(ctx).(*dashboardView)
	tile := quickActionTile(widget.NewThemeIcon("ld-wifi-symbolic", 16), widget.NewLabel(ctx.Font, 12, "Wifi", 0))
	if got := len(tile.Children()); got != 2 {
		t.Fatalf("the tile children = %d, want the icon tile over the label", got)
	}
	square, ok := tile.Children()[0].(*widget.Box)
	if !ok || !square.HasClass("quick-action-icon") || square.SizeRequest() != (widget.Size{W: quickActionTilePx, H: quickActionTilePx}) {
		t.Errorf("the tile's first child = %T, want the square quick-action-icon tile", tile.Children()[0])
	}
	th := widget.Current()
	grid, _ := findByClass(v, "quick-actions").(*widget.Grid)
	if grid == nil || grid.ColumnSpacing() != th.Spacing || grid.RowSpacing() != th.Spacing {
		t.Errorf("quick-actions grid spacing = %v, want the theme's %d", grid, th.Spacing)
	}
}

// The battery card shows only for a present battery: a desktop's UPower display device
// reports none, and the card follows a battery arriving or leaving.
func TestDashboardBatteryCardFollowsPresence(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	bat := newFakeBattery(upower.Device{Percentage: 50, IsPresent: false})
	ctx.Battery = bat
	v := noRadiosDashboard(ctx).(*dashboardView)
	card := func() *widget.Box {
		for _, c := range findByClass(v, "dashboard-info-row").(*widget.Box).Children() {
			if b, ok := c.(*widget.Box); ok && len(treetest.WithClass(b, "battery-status")) > 0 {
				return b
			}
		}
		t.Fatal("no battery card in the info row")
		return nil
	}
	if card().Visible() {
		t.Error("the battery card shows with no battery present")
	}
	bat.update(func(d *upower.Device) { d.IsPresent = true })
	v.refreshAll()
	if !card().Visible() {
		t.Error("the battery card stays hidden once a battery is present")
	}
}

// A quick action's tile and label center in the cell at their natural
// size: the colored icon tile stays a square, it never stretches.
func TestDashboardQuickActionTileCenters(t *testing.T) {
	ctx := styledContext(t, config.Defaults())
	icon := widget.NewThemeIcon("ld-wifi-symbolic", 16)
	label := widget.NewLabel(ctx.Font, 12, "WiFi", 0)
	col := quickActionTile(icon, label)
	b := dropdownButton(col, func() {}, "quick-action")
	ctx.Theme.Attach(b)
	widget.SizeWithin(b, availSize(200, 120))
	b.Arrange(render.Rect{W: 200, H: 120})
	tile := col.Children()[0]
	nat := widget.SizeWithinAsChild(tile, availSize(200, 120))
	tb := boundsOf(tile)
	if tb.W != nat.W {
		t.Errorf("the icon tile spans %dpx, want its natural %dpx", tb.W, nat.W)
	}
	if d := midX(tb) - 100; d < -1 || d > 1 {
		t.Errorf("the icon tile sits %dpx off the cell's center", d)
	}
	if d := midX(boundsOf(label)) - 100; d < -1 || d > 1 {
		t.Errorf("the label sits %dpx off the cell's center", d)
	}
}

// The dashboard grows to fit its sections: the scroll over them asks
// for their height, so a dropdown without a configured height does not
// collapse to its header.
func TestDashboardGrowsToItsSections(t *testing.T) {
	ctx := styledContext(t, config.Defaults())
	v := noRadiosDashboard(ctx).(*dashboardView)
	w, h, _ := dropdownDims("dashboard", ctx.Config)
	card, _ := dropdownTree(dropdownPanel("dashboard", ctx.Config, v), ctx.Config)
	ctx.Theme.Attach(card)
	sz := widget.SizeWithinAsChild(card, widget.Size{W: 2000, H: 4000})
	sections := widget.SizeWithinAsChild(treetest.WithClass(v, "dashboard-scroll")[0].(*widget.Scroll).Children()[0], widget.Size{W: w, H: 4000})
	if h > 0 || sz.H < sections.H {
		t.Errorf("the dashboard (height %d) measures %dpx over %dpx of sections, want it grown to fit them", h, sz.H, sections.H)
	}
}

// quickActionButtons are the dashboard's quick actions in grid order:
// wifi, bluetooth, airplane, dnd, idle inhibit, power saver.
func quickActionButtons(t *testing.T, v *dashboardView) []*widget.Button {
	t.Helper()
	var actions []*widget.Button
	walkTree(v, func(w widget.Widget) bool {
		if b, ok := w.(*widget.Button); ok && b.HasClass("quick-action") {
			actions = append(actions, b)
		}
		return true
	})
	if len(actions) != 6 {
		t.Fatalf("quick actions = %d, want 6", len(actions))
	}
	return actions
}

func quickActionAlert(t *testing.T, v *dashboardView) *widget.Alert {
	t.Helper()
	found := treetest.WithClass(v, "quick-action-failed")
	if len(found) != 1 {
		t.Fatalf("quick-action alerts = %d, want 1", len(found))
	}
	return found[0].(*widget.Alert)
}

func TestTheSaverToggleHoldsWhileItsWriteRunsAndSaysWhyItFailed(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	pp := &fakePPSource{snap: powerprofiles.Snapshot{
		Available: true, Active: powerprofiles.ProfileBalanced,
		Profiles: []string{powerprofiles.ProfilePowerSaver, powerprofiles.ProfileBalanced},
	}}
	pp.setGate = make(chan struct{})
	pp.setErr = errors.New("net.hadess.PowerProfiles: denied")
	ctx.PowerProfiles = pp
	v := noRadiosDashboard(ctx).(*dashboardView)
	defer v.dropdownClosed()
	saver := quickActionButtons(t, v)[5]
	alert := quickActionAlert(t, v)
	if !saver.Enabled() || alert.Visible() {
		t.Fatalf("before the write: saver enabled %v, alert %v", saver.Enabled(), alert.Visible())
	}
	onHeadlessLoop(func() bool { saver.OnClick(); return true })
	onHeadlessLoop(func() bool {
		if saver.Enabled() {
			t.Error("the saver toggle is sensitive while its write runs")
		}
		v.apply(v.read())
		if saver.Enabled() {
			t.Error("a refresh made the saver toggle sensitive while its write runs")
		}
		saver.OnClick()
		return true
	})
	close(pp.setGate)
	waitHeadless(t, "the failure shown", func() bool { return alert.Visible() })
	onHeadlessLoop(func() bool {
		if got := pp.sets(); len(got) != 1 {
			t.Errorf("writes = %v, want one: a click while busy writes nothing", got)
		}
		if !saver.Enabled() || saver.HasClass("active") {
			t.Errorf("after the failed write: enabled %v, active %v; want sensitive and the old value", saver.Enabled(), saver.HasClass("active"))
		}
		if alert.Title() != i18n.T("power-profiles-switch-failed") || alert.Description() != pp.setErr.Error() {
			t.Errorf("the alert reads %q / %q", alert.Title(), alert.Description())
		}
		return true
	})
}

func TestAQuickActionWriteThatGoesThroughClearsTheAlert(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	bt := newFakeBluetooth(bluetooth.State{Available: true})
	bt.fail["enable"] = errors.New("org.bluez.Error.Blocked")
	ctx.Bluetooth = bt
	v := noRadiosDashboard(ctx).(*dashboardView)
	defer v.dropdownClosed()
	v.refreshAll()
	btn := quickActionButtons(t, v)[1]
	alert := quickActionAlert(t, v)
	onHeadlessLoop(func() bool { btn.OnClick(); return true })
	waitHeadless(t, "the failure shown", func() bool { return alert.Visible() })
	if got := onHeadlessLoop(alert.Title); got != i18n.T("dropdown-dashboard-bluetooth-failed") {
		t.Errorf("the alert reads %q", got)
	}
	bt.mu.Lock()
	delete(bt.fail, "enable")
	bt.mu.Unlock()
	onHeadlessLoop(func() bool { btn.OnClick(); return true })
	waitHeadless(t, "the alert cleared", func() bool { return !alert.Visible() && btn.Enabled() })
}

// writeOutcome answers every write a fake makes with err and records
// whether the write ran under a deadline.
type writeOutcome struct {
	err      error
	deadline chan bool
}

func (w *writeOutcome) write(c context.Context) error {
	_, ok := c.Deadline()
	w.deadline <- ok
	return w.err
}

type failingPulse struct {
	*fakePulseSource
	*writeOutcome
}

func (f failingPulse) SetMuted(c context.Context, _ bool) error { return f.write(c) }

func (f failingPulse) SetVolume(c context.Context, _ float64) error { return f.write(c) }

type failingMedia struct {
	*fakeMediaSource
	*writeOutcome
}

func (f failingMedia) PlayPause(c context.Context, _ string) error { return f.write(c) }

func (f failingMedia) SetPosition(c context.Context, _ string, _ time.Duration) error {
	return f.write(c)
}

func classedButton(t *testing.T, root widget.Widget, class string) *widget.Button {
	t.Helper()
	found := treetest.WithClass(root, class)
	if len(found) != 1 {
		t.Fatalf("%s = %d widgets, want 1", class, len(found))
	}
	return found[0].(*widget.Button)
}

func classedAlert(t *testing.T, root widget.Widget, class string) *widget.Alert {
	t.Helper()
	found := treetest.WithClass(root, class)
	if len(found) != 1 {
		t.Fatalf("%s = %d widgets, want 1", class, len(found))
	}
	return found[0].(*widget.Alert)
}

func TestTheVolumeAndMediaCardsSayWhyAWriteFailed(t *testing.T) {
	for _, c := range []struct {
		button, alert, title string
	}{
		{"dashboard-volume-icon", "volume-failed", "dropdown-dashboard-volume-failed"},
		{"play", "media-failed", "dropdown-dashboard-media-failed"},
	} {
		ctx := newTestContext(t, config.Defaults())
		src := &writeOutcome{errors.New("org.freedesktop.DBus.Error.NoReply"), make(chan bool, 4)}
		ctx.Pulse = failingPulse{&fakePulseSource{dev: pulse.Device{Name: "out", Description: "Speakers", Volume: pct(42)}, ticks: make(chan struct{}, 2)}, src}
		ctx.Media = failingMedia{newFakeMedia(mpris.Player{BusName: "org.mpris.MediaPlayer2.x", Title: "Song", State: mpris.StatePlaying}), src}
		v := noRadiosDashboard(ctx).(*dashboardView)
		v.refreshAll()
		alert := classedAlert(t, v, c.alert)
		if onHeadlessLoop(alert.Visible) {
			t.Fatalf("%s shows before any write", c.alert)
		}
		onHeadlessLoop(func() bool { classedButton(t, v, c.button).OnClick(); return true })
		if !<-src.deadline {
			t.Errorf("%s: the write ran without a deadline", c.button)
		}
		waitHeadless(t, c.alert+" shown", alert.Visible)
		onHeadlessLoop(func() bool {
			if alert.Title() != i18n.T(c.title) || alert.Description() != src.err.Error() {
				t.Errorf("%s reads %q / %q", c.alert, alert.Title(), alert.Description())
			}
			return true
		})
		src.err = nil
		onHeadlessLoop(func() bool { classedButton(t, v, c.button).OnClick(); return true })
		<-src.deadline
		waitHeadless(t, c.alert+" cleared", func() bool { return !alert.Visible() })
		v.dropdownClosed()
	}
}

type unswitchableMedia struct{ *fakeMediaSource }

func (unswitchableMedia) SetActive(bus string) error {
	return fmt.Errorf("activate %s: %w", bus, mpris.ErrNoPlayer)
}

func TestSwitchingThePlayerSaysWhyItFailed(t *testing.T) {
	players := []mpris.Player{
		{BusName: "org.mpris.MediaPlayer2.a", Title: "A", State: mpris.StatePlaying},
		{BusName: "org.mpris.MediaPlayer2.b", Title: "B", State: mpris.StatePaused},
	}
	for _, c := range []struct {
		name   string
		src    mpris.Source
		failed bool
	}{
		{"switchable", newFakeMedia(players...), false},
		{"unswitchable", unswitchableMedia{newFakeMedia(players...)}, true},
	} {
		ctx := newTestContext(t, config.Defaults())
		ctx.Media = c.src
		v := noRadiosDashboard(ctx).(*dashboardView)
		v.refreshAll()
		alert := classedAlert(t, v, "media-failed")
		onHeadlessLoop(func() bool { classedButton(t, v, "media-switch-btn").OnClick(); return true })
		onHeadlessLoop(func() bool {
			if alert.Visible() != c.failed {
				t.Errorf("%s: alert visible %v, want %v", c.name, alert.Visible(), c.failed)
			}
			if c.failed && (alert.Title() != i18n.T("dropdown-dashboard-media-failed") || !strings.Contains(alert.Description(), mpris.ErrNoPlayer.Error())) {
				t.Errorf("%s: alert reads %q / %q", c.name, alert.Title(), alert.Description())
			}
			if active, _ := c.src.Active(); !c.failed && active.BusName != players[1].BusName {
				t.Errorf("%s: active player %s, want the next one", c.name, active.BusName)
			}
			return true
		})
		v.dropdownClosed()
	}
}
