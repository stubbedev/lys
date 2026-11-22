package settings

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

const someDevices = `[
	{"name":"AT Translated Set 2 keyboard","capabilities":["keyboard"],"class":"none"},
	{"name":"Synaptics TM3276","capabilities":["pointer"],"class":"touchpad"},
	{"name":"Synaptics TM3276","capabilities":["pointer"],"class":"touchpad"},
	{"name":"virtual pointer","capabilities":["pointer"],"class":"none"}]`

const deviceConfig = `input:
  devices:
    "Logitech M720 2.4GHz":
      natural_scroll: true
    "Das Keyboard":
      keyboard:
        layout: de
`

func deviceKit(t *testing.T, configYAML, devices string) *kit {
	t.Helper()
	k := lysWMKit(t, configYAML)
	k.wm.run = func(ctx context.Context, argv ...string) ([]byte, error) {
		if argv[0] != wmconfig.Control || argv[1] != "devices" {
			return wmconfig.Exec(ctx, argv...)
		}
		if devices == "" {
			return nil, &wmconfig.ToolError{Argv: argv, Exit: 3, Stderr: "lys-wmctl: ipc: no lys-wm socket", Err: errors.New("exit status 3")}
		}
		return []byte(devices), nil
	}
	return k
}

func devicesOf(t *testing.T, k *kit, page pageSpec) *deviceCards {
	t.Helper()
	all := treetest.All[*deviceCards](buildPage(k, page))
	if len(all) != 1 {
		t.Fatalf("%s holds %d device lists, want one", page.id, len(all))
	}
	return all[0]
}

func cardNames(c *deviceCards) []string {
	out := make([]string, len(c.cards))
	for i, cd := range c.cards {
		out[i] = cd.name
		if !cd.connected {
			out[i] += " (absent)"
		}
	}
	return out
}

func cardNamed(t *testing.T, c *deviceCards, name string) *deviceCard {
	t.Helper()
	i := slices.IndexFunc(c.cards, func(cd *deviceCard) bool { return cd.name == name })
	if i < 0 {
		t.Fatalf("no card for %s among %v", name, cardNames(c))
	}
	return c.cards[i]
}

func specPaths(specs []rowSpec) []string {
	out := make([]string, len(specs))
	for i, s := range specs {
		out[i] = s.path
	}
	return out
}

func TestEachPageListsItsConnectedDevicesAndTheConfiguredOnesThatAreNot(t *testing.T) {
	k := deviceKit(t, deviceConfig, someDevices)
	if got, want := cardNames(devicesOf(t, k, wmPointerPage())), []string{"Synaptics TM3276", "Logitech M720 2.4GHz (absent)"}; !slices.Equal(got, want) {
		t.Errorf("Mouse & Touchpad lists %v, want %v", got, want)
	}
	if got, want := cardNames(devicesOf(t, k, wmKeyboardPage())), []string{"AT Translated Set 2 keyboard", "Das Keyboard (absent)"}; !slices.Equal(got, want) {
		t.Errorf("Keyboard lists %v, want %v", got, want)
	}
}

func TestADevicesCardHoldsTheRowsOfItsClass(t *testing.T) {
	k := deviceKit(t, deviceConfig, someDevices)
	pointer := devicesOf(t, k, wmPointerPage())
	if got, want := specPaths(cardNamed(t, pointer, "Synaptics TM3276").specs), specPaths(wmPointerRows("touchpad")); !slices.Equal(got, want) {
		t.Errorf("the touchpad's card holds %v, want the touchpad section's %v", got, want)
	}
	if got, want := specPaths(cardNamed(t, pointer, "Logitech M720 2.4GHz").specs), specPaths(wmPointerRows("pointer")); !slices.Equal(got, want) {
		t.Errorf("an absent device's card holds %v, want the mouse section's %v", got, want)
	}
	keyboard := cardNamed(t, devicesOf(t, k, wmKeyboardPage()), "AT Translated Set 2 keyboard")
	if got, want := specPaths(keyboard.specs), []string{wmLayoutPath, "input.keyboard.options", "input.keyboard.model", "input.keyboard.rules"}; !slices.Equal(got, want) {
		t.Errorf("a keyboard's card holds %v, want the xkb keys a device takes", got)
	}
}

func TestADevicesCardOverridesItsClassForItAlone(t *testing.T) {
	k := deviceKit(t, deviceConfig, someDevices)
	c := devicesOf(t, k, wmPointerPage())
	cd := cardNamed(t, c, "Synaptics TM3276")
	c.fill(cd)
	tap := cd.scope.slot("input.touchpad.tap")
	if tap.get() != nil {
		t.Fatalf("a device without its own tap reads %v, want it inherited", tap.get())
	}
	if err := tap.set(wmToggleOff); err != nil {
		t.Fatal(err)
	}
	acceptedByLysWM(t, k)
	if got := k.wm.valueAt([]string{"input", "devices", "Synaptics TM3276", "tap"}); got != false {
		t.Errorf("the device's tap reads %v, want false", got)
	}
	if got := k.wm.value("input.touchpad.tap"); got != nil {
		t.Errorf("the device's tap wrote the touchpad section's: %v", got)
	}
	cd.scope.reset("input.touchpad.tap")
	acceptedByLysWM(t, k)
	if _, set := asMap(k.wm.value(devicesPath))["Synaptics TM3276"]; set {
		t.Errorf("a reset left the device's entry: %v", k.wm.value(devicesPath))
	}
}

func TestADottedDeviceNameIsOneKey(t *testing.T) {
	k := deviceKit(t, deviceConfig, someDevices)
	cd := cardNamed(t, devicesOf(t, k, wmPointerPage()), "Logitech M720 2.4GHz")
	if got := cd.scope.value("input.pointer.natural_scroll"); got != true {
		t.Fatalf("the device's natural_scroll reads %v, want true", got)
	}
	if err := cd.scope.slot("input.pointer.accel_speed").set(0.5); err != nil {
		t.Fatal(err)
	}
	acceptedByLysWM(t, k)
	if got := k.wm.valueAt([]string{"input", "devices", "Logitech M720 2.4GHz", "accel_speed"}); got != 0.5 {
		t.Errorf("the dotted name's accel_speed reads %v, want 0.5", got)
	}
}

func TestAnAbsentDeviceIsForgottenAndLeavesTheOtherPagesKeys(t *testing.T) {
	k := deviceKit(t, deviceConfig+`    "Combo":
      natural_scroll: true
      keyboard:
        layout: fr
`, someDevices)
	c := devicesOf(t, k, wmPointerPage())
	c.forget(cardNamed(t, c, "Combo"))
	acceptedByLysWM(t, k)
	if slices.Contains(cardNames(c), "Combo (absent)") {
		t.Errorf("a forgotten device is still listed: %v", cardNames(c))
	}
	if got := k.wm.valueAt([]string{"input", "devices", "Combo", "keyboard", "layout"}); got != "fr" {
		t.Errorf("forgetting the pointer settings took the keyboard's layout too: %v", got)
	}
	if !slices.Contains(cardNames(devicesOf(t, k, wmKeyboardPage())), "Combo (absent)") {
		t.Error("the keyboard page lost the device the pointer page forgot")
	}
}

func TestADeviceCardsOwnCurveLeavesTheClassProfile(t *testing.T) {
	k := deviceKit(t, "input:\n  pointer:\n    accel_profile: adaptive\n", `[{"name":"G305","capabilities":["pointer"],"class":"pointer"}]`)
	c := devicesOf(t, k, wmPointerPage())
	cd := cardNamed(t, c, "G305")
	c.fill(cd)
	i := slices.IndexFunc(cd.rows, func(r *fieldRow) bool { return r.path == accelPointsPath("input.pointer") })
	curve, ok := cd.rows[i].ctl.(*accelCurveEditor)
	if !ok {
		t.Fatalf("the device's curve row holds %T", cd.rows[i].ctl)
	}
	curve.begin()
	acceptedByLysWM(t, k)
	if got := k.wm.value("input.pointer.accel_profile"); got != "adaptive" {
		t.Errorf("drawing the device's curve changed the mouse section's profile to %v", got)
	}
	if _, ok := parseAccelCurve(k.wm.valueAt([]string{"input", "devices", "G305", "accel_points"})); !ok {
		t.Errorf("the device holds no curve: %v", k.wm.value(devicesPath))
	}
	c.refresh()
	profile := slices.IndexFunc(cd.rows, func(r *fieldRow) bool { return r.path == "input.pointer.accel_profile" })
	if cd.rows[profile].Visible() {
		t.Error("the device's drawn curve still shows its profile")
	}
}

func TestLookingAgainListsADeviceConnectedSince(t *testing.T) {
	devices := `[]`
	k := lysWMKit(t, "")
	k.wm.run = func(ctx context.Context, argv ...string) ([]byte, error) {
		if argv[0] == wmconfig.Control && argv[1] == "devices" {
			return []byte(devices), nil
		}
		return wmconfig.Exec(ctx, argv...)
	}
	c := devicesOf(t, k, wmPointerPage())
	if len(c.cards) != 0 {
		t.Fatalf("with nothing connected the list holds %v", cardNames(c))
	}
	devices = `[{"name":"G305","capabilities":["pointer"],"class":"pointer"}]`
	c.refresh()
	if len(c.cards) != 0 {
		t.Errorf("a config change listed a device before looking again: %v", cardNames(c))
	}
	c.reload.OnClick()
	if got := cardNames(c); !slices.Equal(got, []string{"G305"}) || c.state.Visible() || !widget.IsEnabled(c.reload) {
		t.Errorf("looking again lists %v with the state shown %v", got, c.state.Visible())
	}
}

func TestTheDeviceListSaysWhyItListsOnlyConfiguredDevices(t *testing.T) {
	tr := locale.T()
	for _, c := range []struct {
		name, devices, want string
	}{
		{"no lys-wm", "", tr.Get("wm-devices-not-running")},
		{"an older lys-wm", `[{"name":"G305","capabilities":["pointer"]}]`, tr.Get("wm-devices-outdated")},
		{"nothing connected", `[]`, ""},
	} {
		k := deviceKit(t, deviceConfig, c.devices)
		d := devicesOf(t, k, wmPointerPage())
		if got := d.state.Title(); d.state.Visible() != (c.want != "") || c.want != "" && got != c.want {
			t.Errorf("%s: the state says %q (shown %v), want %q", c.name, got, d.state.Visible(), c.want)
		}
		if got := cardNames(d); !slices.Equal(got, []string{"Logitech M720 2.4GHz (absent)"}) {
			t.Errorf("%s: lists %v, want the configured mouse", c.name, got)
		}
	}
	d := devicesOf(t, deviceKit(t, "", `[]`), wmPointerPage())
	if got := d.state.Title(); d.state.kind != stateEmpty || got != tr.Get("wm-devices-none") || len(d.cards) != 0 {
		t.Errorf("with nothing to list the state says %q over %d cards", got, len(d.cards))
	}
}
