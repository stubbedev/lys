package settings

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/keys"
	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
	"github.com/stubbedev/lys/settings/internal/wmtest"
)

func TestEveryWMRowIsAKeyOfTheInstalledLysWM(t *testing.T) {
	wmtest.Need(t)
	f, err := wmconfig.Open(os.Getenv, wmconfig.Exec)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := f.Schema()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range allWMRows() {
		if _, ok := schema.Field(wmPath(r.path)); !ok {
			t.Errorf("lys-wm has no key %s", r.path)
		}
	}
}

func TestNoWMRowFallsBackToPlainText(t *testing.T) {
	wmtest.Need(t)
	k := testKit(t, "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	w, err := openWM(k, wmconfig.Exec)
	if err != nil {
		t.Fatal(err)
	}
	k.wm = w
	if got := plainTextRows(t, k, w, allWMRows()); len(got) > 0 {
		t.Fatalf("lys-wm rows on the plain text editor: %v", got)
	}
}

func TestEveryWMRowBuildsWithLabelsAgainstTheInstalledLysWM(t *testing.T) {
	wmtest.Need(t)
	k := testKit(t, "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	w, err := openWM(k, wmconfig.Exec)
	if err != nil {
		t.Fatal(err)
	}
	k.wm = w
	for _, r := range allWMRows() {
		treetest.Walk(fieldControl(k, w, r), func(x widget.Widget) {
			c, ok := x.(*cardList)
			if !ok {
				return
			}
			for _, label := range missingLabels(k, c.spec.fields) {
				t.Errorf("%s: no message %s", r.path, label)
			}
		})
	}
}

func TestEveryDisplayFieldHasItsLabelAgainstTheInstalledLysWM(t *testing.T) {
	wmtest.Need(t)
	k := testKit(t, "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	w, err := openWM(k, wmconfig.Exec)
	if err != nil {
		t.Fatal(err)
	}
	k.wm = w
	fields := wmItemFields(w, []string{outputsPath, wmconfig.Each}, nil, outputsPath)
	for _, label := range missingLabels(k, fields) {
		t.Errorf("no message %s", label)
	}
	for _, f := range fields {
		if !arranged(f) && !locale.T().HasAttr(f.label, "description") {
			t.Errorf("%s has no description for its row", f.label)
		}
	}
}

// missingLabels are the labels of fields, and of the fields inside each
// off-or-object switch among them, that have no message.
func missingLabels(k *kit, fields []cardField) []string {
	tr := locale.T()
	var stored any
	field := func(string) slot {
		return slot{get: func() any { return stored }, set: func(v any) error { stored = v; return nil }, unset: func() { stored = nil }}
	}
	var missing []string
	for _, f := range fields {
		labels := []string{f.label}
		if f.item != nil {
			treetest.Walk(f.item(k, field), func(x widget.Widget) {
				if sw, ok := x.(*wmSwitch); ok {
					labels = append(labels, sw.labels...)
				}
			})
		}
		for _, label := range labels {
			if !tr.Has(label) {
				missing = append(missing, label)
			}
		}
	}
	return missing
}

// systemDefaulted are the lys-wm rows whose value, while unset, lys-wm
// leaves to the system or the device (libinput, xkb, the font and icon
// lookups, the GPU it finds), or which start as an empty list: they read
// "Inherited" or empty, or which are on/off groups, off while unset: every
// other row shows the value lys-wm uses.
var systemDefaulted = []string{
	"decoration.blur.tint", "decoration.blur.mica.tint", "tabs.font", "tabs.icon_theme", "gpu",
	"input.keyboard.layout", "input.keyboard.variant", "input.keyboard.options", "input.keyboard.model", "input.keyboard.rules",
	"input.pointer.natural_scroll", "input.pointer.middle_emulation", "input.pointer.accel_speed", "input.pointer.scroll_factor",
	"input.pointer.tap", "input.pointer.drag_lock", "input.pointer.disable_while_typing", "input.pointer.left_handed", "input.pointer.click_method",
	"input.pointer.scroll_method", "input.pointer.scroll_button", "input.touchpad.scroll_button", "input.trackpoint.scroll_button", "input.pointer.scroll_button_lock", "input.pointer.tap_button_map", "input.pointer.disabled_on_external_mouse",
	"input.trackpoint.tap", "input.trackpoint.drag_lock", "input.trackpoint.disable_while_typing", "input.trackpoint.left_handed", "input.trackpoint.click_method",
	"input.trackpoint.scroll_button_lock", "input.trackpoint.tap_button_map", "input.trackpoint.disabled_on_external_mouse",
	"input.touchpad.left_handed", "input.touchpad.scroll_button_lock", "input.touchpad.tap_button_map", "input.touchpad.disabled_on_external_mouse",
	"input.trackpoint.natural_scroll", "input.trackpoint.middle_emulation", "input.trackpoint.scroll_method", "input.trackpoint.accel_speed", "input.trackpoint.scroll_factor",
	"input.touchpad.tap", "input.touchpad.drag_lock", "input.touchpad.natural_scroll", "input.touchpad.disable_while_typing", "input.touchpad.middle_emulation",
	"input.touchpad.click_method", "input.touchpad.scroll_method", "input.touchpad.accel_speed", "input.touchpad.scroll_factor",
	"input.pointer.accel_points", "input.touchpad.accel_points", "input.trackpoint.accel_points", swallowTerminalsPath, swallowExceptPath,
	"gestures.hold", "gestures.touch", "outputs", "rules", "layer_rules", "autostart", "workspaces", "scratchpads", "animations.curves", "animations.events",
	"decoration.blur.refraction", "decoration.blur.specular", "input.devices",
}

func TestEveryWMRowShowsTheValueLysWMUsesUnlessTheSystemDecides(t *testing.T) {
	wmtest.Need(t)
	k := testKit(t, "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	w, err := openWM(k, wmconfig.Exec)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range allWMRows() {
		inherited := w.value(r.path) == nil
		if want := slices.Contains(systemDefaulted, r.path); inherited != want {
			t.Errorf("%s: reads inherited %v, want %v", r.path, inherited, want)
		}
	}
}

func TestARefusedListItemNamesItsRuntimeLine(t *testing.T) {
	wmtest.Need(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	f, err := wmconfig.Open(os.Getenv, wmconfig.Exec)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		edit wmconfig.Edit
		want string
	}{
		"bind":    {wmconfig.Edit{Path: []string{"binds"}, Value: []any{map[string]any{"chord": "SUPER + Q"}}}, ":2: binds[0].action: is missing"},
		"output":  {wmconfig.Edit{Path: []string{"outputs"}, Value: []any{map[string]any{"scale": 2}}}, ":2: outputs[0].match: is missing"},
		"gesture": {wmconfig.Edit{Path: []string{"gestures", "hold"}, Value: []any{map[string]any{"action": "kill"}}}, ":3: gestures.hold[0].fingers: is missing"},
	}
	for name, c := range cases {
		var rejected *wmconfig.RejectedError
		if err := f.Check([]wmconfig.Edit{c.edit}); !errors.As(err, &rejected) || !strings.Contains(rejected.Reason, f.Runtime()+c.want) {
			t.Errorf("%s: %v; want a refusal naming %s%s", name, err, f.Runtime(), c.want)
		}
	}
}

func TestEveryChordOfTheInstalledLysWMsDefaultsIsReadAndHasKeycaps(t *testing.T) {
	wmtest.Need(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	f, err := wmconfig.Open(os.Getenv, wmconfig.Exec)
	if err != nil {
		t.Fatal(err)
	}
	data, err := f.BuiltIn()
	if err != nil {
		t.Fatal(err)
	}
	tree, err := wmconfig.Tree(data)
	if err != nil {
		t.Fatal(err)
	}
	binds, _ := tree[bindsKey].([]any)
	submaps, _ := tree[submapsKey].(map[string]any)
	for _, mode := range submaps {
		more, _ := mode.([]any)
		binds = append(binds, more...)
	}
	if len(binds) == 0 {
		t.Fatal("lys-wm's defaults bind nothing")
	}
	for _, b := range binds {
		item, _ := b.(map[string]any)
		chord, _ := item[chordKey].(string)
		c, err := keys.WM.Parse(chord)
		if err != nil {
			t.Errorf("a default bind: %v", err)
			continue
		}
		if back, err := keys.WM.Parse(c.WM()); err != nil || back != c {
			t.Errorf("%q prints back as %q, which reads %+v, %v", chord, c.WM(), back, err)
		}
		k, ok := c.Trigger.(keys.Key)
		if !ok {
			continue
		}
		if label, ok := widget.KeyLabel(k.Sym); !ok {
			t.Errorf("%q: its key has no keycap, it reads %q", chord, label)
		}
	}
}
