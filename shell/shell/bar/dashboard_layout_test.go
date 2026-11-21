package bar

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/mpris"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/desktop/service/upower"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

func speakers(index uint32, description string) pulse.OutputDevice {
	return pulse.OutputDevice{Key: pulse.DeviceKey{Index: index}, Description: description, Volume: pct(40)}
}

func volumeDashboard(t *testing.T, src *fakePulseSource) *dashboardView {
	t.Helper()
	ctx := newTestContext(t, config.Defaults())
	ctx.Pulse = src
	v := opened(noRadiosDashboard(ctx)).(*dashboardView)
	v.refreshAll()
	return v
}

func TestTheVolumeIconSaysMutedOrShowsTheLevel(t *testing.T) {
	for _, c := range []struct {
		name  string
		muted bool
		level float64
		icon  string
	}{
		{"unmuted at 40%", false, 40, "ld-volume-1-symbolic"},
		{"unmuted at 90%", false, 90, "ld-volume-2-symbolic"},
		{"muted at 40%", true, 40, "ld-volume-x-symbolic"},
	} {
		t.Run(c.name, func(t *testing.T) {
			v := volumeDashboard(t, &fakePulseSource{dev: pulse.Device{Description: "Speakers", Volume: pct(c.level), Muted: c.muted}, ticks: make(chan struct{}, 1)})
			mute := classedButton(t, v, "dashboard-volume-icon")
			icon := treetest.First[*widget.Icon](t, mute)
			if icon.Name() != c.icon {
				t.Errorf("the volume icon is %q, want %q", icon.Name(), c.icon)
			}
			row := classedBox(t, v, "dashboard-slider-row")
			if row.HasClass("muted") != c.muted {
				t.Errorf("the slider row muted %v, want %v: the level reads as held while muted", row.HasClass("muted"), c.muted)
			}
		})
	}
}

func TestTheOutputSwitchesToTheNextDevice(t *testing.T) {
	built := speakers(1, "Core Ultra 200V Series Processors HD Audio Speaker")
	hdmi := speakers(2, "HDMI")
	src := &fakePulseSource{dev: built.Device, outputs: []pulse.OutputDevice{built, hdmi}, ticks: make(chan struct{}, 1), defaulted: make(chan pulse.DeviceKey, 1)}
	v := volumeDashboard(t, src)
	switchOutput := classedButton(t, v, "output-switch-btn")
	if !switchOutput.Visible() {
		t.Fatal("two outputs and no switch between them")
	}
	if got := switchOutput.TooltipText(); got != i18n.T("dropdown-dashboard-switch-output") {
		t.Errorf("the switch reads %q", got)
	}
	onHeadlessLoop(func() bool { switchOutput.OnClick(); return true })
	select {
	case key := <-src.defaulted:
		if key != hdmi.Key {
			t.Errorf("the switch made %v the default, want the next output %v", key, hdmi.Key)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the switch never set a default output")
	}

	single := volumeDashboard(t, &fakePulseSource{dev: built.Device, outputs: []pulse.OutputDevice{built}, ticks: make(chan struct{}, 1)})
	if classedButton(t, single, "output-switch-btn").Visible() {
		t.Error("one output shows a switch with nowhere to go")
	}
}

func TestALongOutputNameNeverWidensTheDashboard(t *testing.T) {
	long := strings.Repeat("Core Ultra 200V Series Processors HD Audio Speaker ", 4)
	ctx := styledContext(t, config.Defaults())
	ctx.Pulse = &fakePulseSource{dev: pulse.Device{Description: long, Volume: pct(40)}, ticks: make(chan struct{}, 1)}
	v := opened(noRadiosDashboard(ctx)).(*dashboardView)
	v.refreshAll()
	device := classedWidget(t, v, "controls-device").(*widget.Label)
	if device.Text() != long {
		t.Fatalf("the device label reads %q", device.Text())
	}
	width, _, _ := dropdownDims("dashboard", ctx.Config)
	panel := dropdownPanel("dashboard", ctx.Config, v)
	tree, _ := dropdownTree(panel, ctx.Config)
	ctx.Theme.Attach(tree)
	widget.SizeWithinAsChild(tree, widget.Size{W: 2000, H: 4000})
	if got := panel.Measure(widget.Row, widget.Unknown).Nat; got != width {
		t.Errorf("the dashboard asks for %dpx across with a long output name, want its %dpx", got, width)
	}
}

func layoutContext(t *testing.T) ModuleContext {
	t.Helper()
	ctx := styledContext(t, config.Defaults())
	ctx.Pulse = &fakePulseSource{dev: pulse.Device{Description: "Speakers", Volume: pct(40)}, ticks: make(chan struct{}, 1)}
	ctx.Battery = newFakeBattery(upower.Device{Percentage: 64, IsPresent: true, State: upower.StateDischarging, TimeToEmpty: 3 * time.Hour})
	return ctx
}

func scrollRange(t *testing.T, ctx ModuleContext, monH int) int {
	t.Helper()
	v := opened(noRadiosDashboard(ctx)).(*dashboardView)
	v.refreshAll()
	panel := dropdownPanel("dashboard", ctx.Config, v).(*panelBox)
	panel.clamp(monH)
	tree, rev := dropdownTree(panel, ctx.Config)
	rev.SetRevealed(true)
	rev.Finish()
	ctx.Theme.Attach(tree)
	for range 2 {
		sz := widget.SizeWithinAsChild(tree, widget.Size{W: 2000, H: 4000})
		tree.Arrange(render.Rect{W: sz.W, H: sz.H})
	}
	content := v.scroll.Children()[0]
	return widget.SizeWithinAsChild(content, widget.Size{W: v.scroll.Bounds().W, H: 1 << 14}).H - v.scroll.Bounds().H
}

func TestTheCommonCardsFitWithoutScrollingOnA1200PixelOutput(t *testing.T) {
	ctx := layoutContext(t)
	if over := scrollRange(t, ctx, 1200); over > 0 {
		t.Errorf("an idle dashboard on a 1200px tall output scrolls by %dpx, want every card in view", over)
	}
	ctx.Media = newFakeMedia(mpris.Player{BusName: "org.mpris.MediaPlayer2.x", Title: "Song", Artist: "Band", State: mpris.StatePlaying})
	if over := scrollRange(t, ctx, 1200); over > 0 {
		t.Errorf("a dashboard playing media on a 1200px tall output scrolls by %dpx, want every card in view", over)
	}
	if over := scrollRange(t, ctx, 600); over <= 0 {
		t.Error("on a 600px tall output the dashboard does not scroll, so its cards spill past the panel")
	}
}

func TestTheBatteryAndNetworkCardsShareTheirRowEqually(t *testing.T) {
	ctx := layoutContext(t)
	v := opened(noRadiosDashboard(ctx)).(*dashboardView)
	v.refreshAll()
	ctx.Theme.Attach(v)
	arrangeDropdown(t, v, 380, 900)
	row := classedBox(t, v, "dashboard-info-row")
	kids := row.Children()
	if len(kids) != 2 {
		t.Fatalf("the info row holds %d cards, want battery and network", len(kids))
	}
	battery, network := boundsOf(kids[0]), boundsOf(kids[1])
	if d := battery.W - network.W; d < -1 || d > 1 {
		t.Errorf("the battery card is %dpx wide and the network card %dpx, want them equal", battery.W, network.W)
	}
	for _, stat := range treetest.WithClass(v, "speed-stat") {
		value := treetest.WithClass(stat, "speed-value")[0]
		unit := treetest.WithClass(stat, "speed-unit")[0]
		if vb, ub := boundsOf(value), boundsOf(unit); ub.Y >= vb.Y+vb.H || ub.X < vb.X+vb.W {
			t.Errorf("a speed's unit sits at %+v, off the line of its value at %+v", ub, vb)
		}
	}

	ctx.Battery = nil
	alone := opened(noRadiosDashboard(ctx)).(*dashboardView)
	if got := len(classedBox(t, alone, "dashboard-info-row").Children()); got != 1 {
		t.Errorf("without a battery the info row holds %d cards, want the network card alone", got)
	}
}

func TestTheBatteryCardSaysWhatThePowerSaverToggleCannot(t *testing.T) {
	ctx := layoutContext(t)
	dev := upower.Device{Percentage: 79, IsPresent: true, State: upower.StatePendingCharge, ChargeThresholdSupported: true, ChargeThresholdEnabled: true, ChargeEndThreshold: 80}
	ctx.Battery = newFakeBattery(dev)
	v := opened(noRadiosDashboard(ctx)).(*dashboardView)
	v.refreshAll()
	state := classedWidget(t, v, "battery-state").(*widget.Label)
	if state.Text() != batteryStateLabel(dev) {
		t.Errorf("the battery state reads %q, want %q", state.Text(), batteryStateLabel(dev))
	}
	limit := classedWidget(t, v, "battery-limit").(*widget.Label)
	if !limit.Visible() || limit.Text() != dashboardChargeLimit(dev) {
		t.Errorf("the charge limit shows %v reading %q, want %q", limit.Visible(), limit.Text(), dashboardChargeLimit(dev))
	}
	card := dashboardCardHeaded(t, v, i18n.T("dropdown-dashboard-battery"))
	walkTree(card, func(w widget.Widget) bool {
		if l, ok := w.(*widget.Label); ok {
			for _, profile := range []string{i18n.T("dropdown-battery-profile-saver"), i18n.T("dropdown-dashboard-power-saver")} {
				if l.Text() == profile {
					t.Errorf("the battery card repeats the power profile %q the toggle shows", profile)
				}
			}
		}
		return true
	})

	ctx.Battery = newFakeBattery(upower.Device{Percentage: 79, IsPresent: true, State: upower.StateDischarging})
	v = opened(noRadiosDashboard(ctx)).(*dashboardView)
	v.refreshAll()
	if classedWidget(t, v, "battery-limit").(*widget.Label).Visible() {
		t.Error("the charge limit shows with none in force")
	}
}

func TestQuickActionStatesAreEachTheirOwn(t *testing.T) {
	ctx := lookContext(t)
	v := opened(noRadiosDashboard(ctx)).(*dashboardView)
	v.refreshAll()
	ctx.Theme.Attach(v)
	arrangeDropdown(t, v, 380, 900)
	actions := quickActionButtons(t, v)
	on, off := actions[3], actions[4]
	if !on.HasClass("active") || off.HasClass("active") {
		t.Fatalf("Do Not Disturb active %v, Keep Awake active %v; want on and off", on.HasClass("active"), off.HasClass("active"))
	}
	tile := func(b *widget.Button) widget.Widget { return treetest.WithClass(b, "quick-action-icon")[0] }
	label := func(b *widget.Button) widget.Widget { return treetest.WithClass(b, "quick-action-label")[0] }
	if widget.CascadeColor(label(off)) != widget.CascadeColor(label(on)) {
		t.Errorf("an off toggle's label is %#08x and an on one's %#08x: the off label loses contrast", uint32(widget.CascadeColor(label(off))), uint32(widget.CascadeColor(label(on))))
	}
	if widget.StyleBackground(tile(on)) == widget.StyleBackground(tile(off)) {
		t.Error("on and off tiles share one background")
	}
	if bg := widget.StyleBackground(off); bg.A() != 0 {
		t.Errorf("an off toggle's button paints %#08x behind its tile, a state beside on and off", uint32(bg))
	}

	r := &widget.Router{Root: v}
	if err := r.SetFocus(off); err != nil {
		t.Fatal(err)
	}
	if bg := widget.StyleBackground(off); bg.A() != 0 {
		t.Errorf("a focused toggle paints %#08x behind its tile, a third look beside on and off", uint32(bg))
	}
}

func TestControlsOfAHiddenCardTakeNoArrowKeyFocus(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	media := newFakeMedia()
	ctx.Media = media
	v := opened(noRadiosDashboard(ctx)).(*dashboardView)
	v.refreshAll()
	play := widget.Widget(classedButton(t, v, "play"))
	if slices.Contains(focusablesUnder(v.root, nil), play) {
		t.Error("the play button of the hidden media card is in the arrow keys' focus order")
	}
	media.mu.Lock()
	media.players, media.active = []mpris.Player{{BusName: "org.mpris.MediaPlayer2.x", Title: "Song", State: mpris.StatePlaying}}, 0
	media.mu.Unlock()
	v.refreshAll()
	if !slices.Contains(focusablesUnder(v.root, nil), play) {
		t.Error("the play button of the shown media card left the arrow keys' focus order")
	}
}
