package bar

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/mpris"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

// unnamedControls lists the buttons and switches under root that
// assistive technology would read as nothing but their role.
func unnamedControls(root widget.Widget) []string {
	var out []string
	treetest.Walk(root, func(w widget.Widget) {
		switch widget.RoleOf(w) {
		case widget.RoleButton, widget.RoleToggleButton, widget.RoleSwitch:
		default:
			return
		}
		if widget.Describe(w).Name != "" {
			return
		}
		classes := ""
		if c, ok := w.(interface{ Classes() []string }); ok {
			classes = strings.Join(c.Classes(), ".")
		}
		out = append(out, widget.RoleOf(w).String()+"."+classes)
	})
	return out
}

// buildDropdownWithoutServices builds a dropdown as the bar does with
// no service behind it; ok is false for one that reaches for a service.
func buildDropdownWithoutServices(t *testing.T, build func(ModuleContext) widget.Widget) (root widget.Widget, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	ctx := newTestContext(t, config.Defaults())
	return build(ctx), true
}

// The checker itself: a bare icon button and a bare switch are
// unnamed, the same controls through the dropdown helpers are not.
func TestUnnamedControlsFindsBareIconsAndSwitches(t *testing.T) {
	box := widget.NewBox(widget.Column, 0, 0)
	box.Append(widget.NewButton(widget.NewThemeIcon("ld-x-symbolic", 16), 0, 0), false)
	box.Append(widget.NewSwitch(false), false)
	if got := unnamedControls(box); len(got) != 2 {
		t.Fatalf("unnamed controls = %v, want the bare button and switch", got)
	}
	named := widget.NewBox(widget.Column, 0, 0)
	named.Append(iconButton(widget.NewThemeIcon("ld-x-symbolic", 16), "Close", nil), false)
	named.Append(ghostIconButton("ld-x-symbolic", 16, "Close", nil), false)
	named.Append(namedSwitch("WiFi", false), false)
	named.Append(dropdownButton(widget.NewLabel(treetest.Font(t), 12, "Retry", 0), nil), false)
	if got := unnamedControls(named); len(got) != 0 {
		t.Fatalf("unnamed controls = %v, want none", got)
	}
}

// Every button and switch every dropdown builds is named, so a screen
// reader says what it does instead of "button".
func TestEveryDropdownControlIsNamed(t *testing.T) {
	builders := dropdownBuilders()
	var built []string
	for _, name := range slices.Sorted(func(yield func(string) bool) {
		for k := range builders {
			if !yield(k) {
				return
			}
		}
	}) {
		root, ok := buildDropdownWithoutServices(t, builders[name])
		if !ok {
			continue
		}
		built = append(built, name)
		if got := onHeadlessLoop(func() []string { return unnamedControls(root) }); len(got) != 0 {
			t.Errorf("%s dropdown: unnamed controls %v", name, got)
		}
	}
	for _, want := range []string{"network", "bluetooth", "media", "dashboard", "notification"} {
		if !slices.Contains(built, want) {
			t.Errorf("the %s dropdown did not build without its services (built %v)", want, built)
		}
	}
}

// The media transport buttons are named, and the play, shuffle and
// repeat names follow the player's state.
func TestMediaTransportButtonsAreNamedByTheirState(t *testing.T) {
	source := newFakeMedia(mpris.Player{BusName: "org.mpris.MediaPlayer2.vlc", Identity: "VLC", CanShuffle: true, CanLoop: true, CanGoNext: true, CanGoPrevious: true})
	ctx := newTestContext(t, config.Defaults())
	ctx.Media = source
	v := opened(mediaDropdown(ctx)).(*mediaView)
	defer v.dropdownClosed()
	name := func(b *widget.Button) string { return onHeadlessLoop(func() string { return widget.Describe(b).Name }) }
	refresh := v.refresh
	refresh()
	if got := onHeadlessLoop(func() []string { return unnamedControls(v) }); len(got) != 0 {
		t.Errorf("unnamed controls with a player: %v", got)
	}
	want := map[*widget.Button]string{
		v.previous:  i18n.T("dropdown-media-previous"),
		v.next:      i18n.T("dropdown-media-next"),
		v.playPause: i18n.T("dropdown-media-play"),
		v.shuffle:   i18n.T("dropdown-media-shuffle-off"),
		v.loop:      i18n.T("dropdown-media-repeat-off"),
	}
	for b, w := range want {
		if got := name(b); got != w {
			t.Errorf("stopped player: a transport button is named %q, want %q", got, w)
		}
	}
	source.players[0].State = mpris.StatePlaying
	source.players[0].Shuffle = true
	source.players[0].Loop = mpris.LoopTrack
	refresh()
	for b, w := range map[*widget.Button]string{
		v.playPause: i18n.T("dropdown-media-pause"),
		v.shuffle:   i18n.T("dropdown-media-shuffle-on"),
		v.loop:      i18n.T("dropdown-media-repeat-track"),
	} {
		if got := name(b); got != w {
			t.Errorf("playing, shuffled, repeating the track: a button is named %q, want %q", got, w)
		}
	}
	source.players[0].Loop = mpris.LoopPlaylist
	refresh()
	if got := name(v.loop); got != i18n.T("dropdown-media-repeat-playlist") {
		t.Errorf("repeating the playlist: the repeat button is named %q", got)
	}
}

// A mute button says what a click does: mute while sound plays, unmute
// while it is muted.
func TestAudioMuteLabelSaysWhatAClickDoes(t *testing.T) {
	if got, want := audioMuteLabel(false), i18n.T("dropdown-audio-mute"); got != want {
		t.Errorf("unmuted: %q, want %q", got, want)
	}
	if got, want := audioMuteLabel(true), i18n.T("dropdown-audio-unmute"); got != want {
		t.Errorf("muted: %q, want %q", got, want)
	}
}

// The network and bluetooth headers name their scan buttons and power
// switches.
func TestRadioHeadersNameTheirScanAndSwitch(t *testing.T) {
	n := opened(newNetworkView(newTestContext(t, config.Defaults()), netDeps{}))
	defer n.dropdownClosed()
	if got := onHeadlessLoop(func() string { return widget.Describe(n.scanBtn).Name }); got != i18n.T("dropdown-network-scan") {
		t.Errorf("network scan button named %q", got)
	}
	if got := onHeadlessLoop(func() string { return widget.Describe(n.wifiSwitch).Name }); got != i18n.T("dropdown-network-wifi") {
		t.Errorf("WiFi switch named %q", got)
	}
	d, _ := newTestBtDropdown(t, btWorld())
	if got := onHeadlessLoop(func() string { return widget.Describe(d.scanBtn).Name }); got != btText("dropdown-bluetooth-scan") {
		t.Errorf("bluetooth scan button named %q", got)
	}
	if got := onHeadlessLoop(func() string { return widget.Describe(d.toggle).Name }); got != btText("dropdown-bluetooth-title") {
		t.Errorf("bluetooth switch named %q", got)
	}
}
