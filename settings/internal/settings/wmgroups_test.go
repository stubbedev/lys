package settings

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

func groupSwitch(t *testing.T, p *settingsPage, path string) *toggle {
	t.Helper()
	c, ok := rowOf(t, p, path).ctl.(*toggle)
	if !ok {
		t.Fatalf("the %s row holds %T, want a switch", path, rowOf(t, p, path).ctl)
	}
	return c
}

func TestRefractionAndTheRimSheenAreGroupsTheirRowsFollow(t *testing.T) {
	for group, rows := range map[string][]string{
		"decoration.blur.refraction": {"decoration.blur.refraction.bezel", "decoration.blur.refraction.amount"},
		"decoration.blur.specular":   {"decoration.blur.specular.strength", "decoration.blur.specular.width"},
	} {
		k := lysWMKit(t, "")
		p := layoutPage(t, k, "wm-effects")
		sw := groupSwitch(t, p, group)
		if sw.On() {
			t.Fatalf("%s reads on with no config", group)
		}
		for _, r := range rows {
			if rowOf(t, p, r).Enabled() {
				t.Errorf("%s is enabled while %s is off", r, group)
			}
		}
		sw.OnChanged(true)
		if got := fileText(t, k.wm.file.Runtime()); got != "decoration:\n  blur:\n    "+group[len("decoration.blur."):]+": {}\n" {
			t.Errorf("turning %s on wrote %q", group, got)
		}
		for _, r := range rows {
			if !rowOf(t, p, r).Enabled() {
				t.Errorf("%s stays disabled with %s on", r, group)
			}
			if d, _ := k.wm.defaultOf(r); k.wm.value(r) != d || d == nil {
				t.Errorf("%s reads %v with the group on, want the published default %v", r, k.wm.value(r), d)
			}
		}
		sw.OnChanged(false)
		if isObject(k.wm.value(group)) || rowOf(t, p, rows[0]).Enabled() {
			t.Errorf("turning %s off left %v and its rows enabled %v", group, k.wm.value(group), rowOf(t, p, rows[0]).Enabled())
		}
	}
}

func TestTurningOffAGroupTheConfigFileSetsWritesNull(t *testing.T) {
	k := lysWMKit(t, "decoration:\n  blur:\n    refraction: {amount: 4}\n")
	p := layoutPage(t, k, "wm-effects")
	sw := groupSwitch(t, p, "decoration.blur.refraction")
	if !sw.On() || !rowOf(t, p, "decoration.blur.refraction.amount").Enabled() {
		t.Fatal("the config file's refraction reads off")
	}
	sw.OnChanged(false)
	if got := fileText(t, k.wm.file.Runtime()); got != "decoration:\n  blur:\n    refraction: null\n" {
		t.Errorf("turning the config file's refraction off wrote %q", got)
	}
	if k.wm.value("decoration.blur.refraction") != nil || rowOf(t, p, "decoration.blur.refraction.amount").Enabled() {
		t.Error("refraction stays on over the null")
	}
	sw.OnChanged(true)
	if got := k.wm.value("decoration.blur.refraction.amount"); got != int64(4) {
		t.Errorf("turning it back on reads amount %v, want the config file's 4", got)
	}
	if got := fileText(t, k.wm.file.Runtime()); got != "" {
		t.Errorf("turning it back on left the override %q", got)
	}
}

func TestResettingAGroupsLastKeyKeepsTheGroupOn(t *testing.T) {
	k := lysWMKit(t, "")
	p := layoutPage(t, k, "wm-effects")
	groupSwitch(t, p, "decoration.blur.specular").OnChanged(true)
	setRow(t, p, "decoration.blur.specular.width", int64(3))
	k.wm.reset("decoration.blur.specular.width")
	if !isObject(k.wm.value("decoration.blur.specular")) {
		t.Fatal("resetting the sheen's width turned the sheen off")
	}
	if got := k.wm.value("decoration.blur.specular.width"); got != int64(1) {
		t.Errorf("the reset width reads %v, want lys-wm's 1", got)
	}
	if err := k.wm.slot("gaps.inner").set(int64(3)); err != nil {
		t.Fatal(err)
	}
	k.wm.reset("gaps.inner")
	if k.wm.inGroupAlone("gaps.inner") || k.wm.source("gaps.inner").reset {
		t.Error("a key outside any group is held as one")
	}
}

func TestOnlyALysWMGroupTakesAGroupSwitch(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a group over decoration.blur, which is no on/off group, was built")
		}
	}()
	wmGroup("decoration.blur", wmField("decoration.blur.size"))
}

func TestThePreviewLightsTheRimSheenOnBlurredGlass(t *testing.T) {
	k := lysWMKit(t, "")
	l, bad := readWindowLook(k.wm.value, func(text string) (render.Color, bool) { return wmColorRGB(k, text) })
	if bad != nil {
		t.Fatal(bad)
	}
	if l.blur.sheen.shown {
		t.Fatal("lys-wm's defaults preview a rim sheen")
	}
	l.shadow.enabled, l.borderWidth, l.rounding, l.titlebar.shown = false, 0, 0, false
	l.blur.enabled, l.opacity = true, [windowFocusStates]float64{0.8, 0.8}
	const w, h = 400, windowPreviewH
	screen := render.Rect{W: w, H: h}
	frame := windowFrames(l, screen)[windowFocused]
	edge := func(sheen sheenLook) render.Color {
		l.blur.sheen = sheen
		data := make([]byte, render.Stride(w)*h)
		paintWindows(render.New(data, render.Stride(w), w, h), l, screen)
		at := frame.Y*render.Stride(w) + (frame.X+frame.W/2)*4
		return render.ColorFromBytes(data[at:])
	}
	bare := edge(sheenLook{})
	lit := edge(sheenLook{shown: true, strength: 1, width: 4})
	if lit == bare {
		t.Errorf("a full-strength sheen leaves the glass's edge %#x", lit)
	}
	l.blur.enabled = false
	if got := edge(sheenLook{shown: true, strength: 1, width: 4}); got != edge(sheenLook{}) {
		t.Errorf("a sheen without blur lights the edge %#x", got)
	}
}

func TestAGroupSwitchTakesOnlyOnOrOff(t *testing.T) {
	wrote := false
	s := groupSlot(slot{get: func() any { return nil }, set: func(any) error { wrote = true; return nil }, unset: func() { wrote = true }}, func() bool { return false })
	if err := s.set("yes"); err == nil || wrote {
		t.Errorf("a group set to %q wrote %v and said %v", "yes", wrote, err)
	}
	if err := s.set(true); err != nil || !wrote {
		t.Errorf("a group turned on wrote %v and said %v", wrote, err)
	}
}
