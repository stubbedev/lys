package settings

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/bluetooth"
	"github.com/stubbedev/lys/desktop/service/network"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/desktop/service/upower"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestTheWindowOpensOnTheOverview(t *testing.T) {
	k := testKit(t, "")
	w := newWindow(k, k.store.svc.Config(), (&fakeAfter{}).after)
	w.open(launch{})
	if w.pageID != overviewID {
		t.Errorf("the window opens on %q, want the overview", w.pageID)
	}
}

func TestTheOverviewHasATileOpeningEachSection(t *testing.T) {
	k := testKit(t, "")
	w := newWindow(k, k.store.svc.Config(), (&fakeAfter{}).after)
	tiles := treetest.All[*widget.Button](overviewTiles(k))
	want := len(w.sections) - 1
	if len(tiles) != want {
		t.Fatalf("%d tiles for %d sections besides home", len(tiles), want)
	}
	for i, s := range w.sections[1:] {
		if name := tiles[i].AccessibleName(); name != locale.T().Get(s.key) {
			t.Errorf("tile %d is %q, want %q", i, name, locale.T().Get(s.key))
		}
		tiles[i].OnClick()
		if w.pageID != s.pages[0].id {
			t.Errorf("the %s tile opened %q, want %q", s.key, w.pageID, s.pages[0].id)
		}
		if !locale.T().HasAttr(s.key, "description") {
			t.Errorf("%s has no description for its tile", s.key)
		}
	}
}

func TestTheOverviewLeavesOutWhatIsNotInstalled(t *testing.T) {
	k := testKit(t, "")
	k.parts = parts{shell: true}
	newWindow(k, k.store.svc.Config(), (&fakeAfter{}).after)
	for _, b := range treetest.All[*widget.Button](overviewTiles(k)) {
		if b.AccessibleName() == locale.T().Get("settings-nav-input") {
			t.Errorf("lys-shell alone shows the %q tile", b.AccessibleName())
		}
	}
}

func descriptionOf(t *testing.T, tiles widget.Widget, key string) string {
	t.Helper()
	for _, b := range treetest.All[*widget.Button](tiles) {
		if b.AccessibleName() == locale.T().Get(key) {
			return treetest.Labels(b)[1]
		}
	}
	t.Fatalf("no tile for %s", key)
	return ""
}

func TestATileShowsItsLiveStatusAndKeepsItsDescriptionWithoutOne(t *testing.T) {
	k := testKit(t, "")
	newWindow(k, k.store.svc.Config(), (&fakeAfter{}).after)
	k.statuses = map[string]tileStatus{
		"settings-nav-sound-section": func(context.Context, *kit) (string, error) { return "Speakers · 40%", nil },
		"settings-nav-account":       func(context.Context, *kit) (string, error) { return "", errors.New("no accounts-daemon") },
	}
	tiles := overviewTiles(k)
	if got := descriptionOf(t, tiles, "settings-nav-sound-section"); got != "Speakers · 40%" {
		t.Errorf("the sound tile reads %q", got)
	}
	if got, want := descriptionOf(t, tiles, "settings-nav-account"), locale.T().Attr("settings-nav-account", "description"); got != want {
		t.Errorf("an account tile without accounts-daemon reads %q, want its description %q", got, want)
	}
}

func TestTheNetworkTileSaysWhatIsConnected(t *testing.T) {
	k := testKit(t, "")
	tr := locale.T()
	for _, c := range []struct {
		state netState
		want  string
	}{
		{netState{active: "Home"}, tr.Get("settings-overview-wifi", i18n.Str("name", "Home"))},
		{netState{snap: network.Snapshot{Wired: network.StatusConnected}}, tr.Get("settings-overview-wired")},
		{netState{}, tr.Get("settings-overview-offline")},
	} {
		f := &fakeNet{state: c.state}
		k.network = func() (netControl, error) { return f, nil }
		if got, err := networkStatus(t.Context(), k); err != nil || got != c.want || !f.closed {
			t.Errorf("%+v reads %q, %v, closed %v; want %q", c.state, got, err, f.closed, c.want)
		}
	}
	k.network = func() (netControl, error) { return nil, errors.New("no NetworkManager") }
	if _, err := networkStatus(t.Context(), k); err == nil {
		t.Error("a missing NetworkManager gave a status")
	}
}

func TestTheAppearanceTileNamesTheAppearanceAndTheProvider(t *testing.T) {
	k := testKit(t, "styling:\n  appearance: dark\n  theme-provider: matugen\n")
	got, err := appearanceStatus(t.Context(), k)
	if err != nil || !strings.Contains(got, enumLabel("styling.appearance", "dark")) || !strings.Contains(got, enumLabel("styling.theme-provider", "matugen")) {
		t.Errorf("%q, %v", got, err)
	}
}

func TestTheSoundTileNamesTheOutputAndItsVolume(t *testing.T) {
	srv := soundServer(t)
	useSoundServer(t, srv)
	got, err := soundStatus(t.Context(), testKit(t, ""))
	if err != nil || !strings.Contains(got, "%") {
		t.Fatalf("%q, %v", got, err)
	}
	openPulse = func(context.Context) (*pulse.Service, error) { return nil, errors.New("no sound server") }
	if _, err := soundStatus(t.Context(), testKit(t, "")); err == nil {
		t.Error("a missing sound server gave a status")
	}
}

func TestEveryTileStatusNamesASection(t *testing.T) {
	keys := map[string]bool{}
	for _, s := range layout(config.Defaults(), bothParts) {
		keys[s.key] = true
	}
	for key := range tileStatuses {
		if !keys[key] {
			t.Errorf("a tile status for %s, which is no section", key)
		}
	}
}

func TestThePowerTileShowsTheChargeAndNothingWithoutABattery(t *testing.T) {
	k := testKit(t, "")
	f := &fakePower{state: powerState{upower: true, battery: upower.Device{IsPresent: true, Percentage: 64, State: upower.StateCharging}}}
	k.power = func() (powerControl, error) { return f, nil }
	got, err := powerStatus(t.Context(), k)
	if err != nil || !strings.Contains(got, percentText(64)) || !f.closed {
		t.Fatalf("%q, %v, closed %v", got, err, f.closed)
	}
	f.state.battery.IsPresent = false
	if _, err := powerStatus(t.Context(), k); !errors.Is(err, errNoStatus) {
		t.Errorf("a desktop without a battery reports %v", err)
	}
}

func TestTheConnectivityTileJoinsNetworkAndBluetooth(t *testing.T) {
	k := testKit(t, "")
	tr := locale.T()
	k.network = func() (netControl, error) { return &fakeNet{state: netState{active: "Home"}}, nil }
	bt := &fakeBT{st: bluetooth.State{Available: true, Enabled: true, Devices: []bluetooth.Device{{Alias: "Buds", Connected: true}}}}
	k.bluetooth = func() (btControl, error) { return bt, nil }
	status := tileStatuses["settings-nav-connectivity"]
	want := tr.Get("settings-overview-wifi", i18n.Str("name", "Home")) + " · " + tr.Get("settings-overview-bluetooth-connected", i18n.Str("name", "Buds"))
	if got, err := status(t.Context(), k); err != nil || got != want {
		t.Fatalf("%q, %v; want %q", got, err, want)
	}
	bt.st.Enabled = false
	k.network = func() (netControl, error) { return nil, errors.New("no NetworkManager") }
	if got, err := status(t.Context(), k); err != nil || got != tr.Get("settings-overview-bluetooth-off") {
		t.Errorf("without NetworkManager and Bluetooth off: %q, %v", got, err)
	}
	k.bluetooth = func() (btControl, error) { return nil, errors.New("no BlueZ") }
	if _, err := status(t.Context(), k); err == nil {
		t.Error("with neither service the tile got a status")
	}
}
