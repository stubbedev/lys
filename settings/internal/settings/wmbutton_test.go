package settings

import (
	"slices"
	"testing"
)

func scrollButtonOf(t *testing.T, p *settingsPage, path string) *namedOrWhole {
	t.Helper()
	c, ok := rowOf(t, p, path).ctl.(*namedOrWhole)
	if !ok {
		t.Fatalf("the %s row holds %T, want the button picker", path, rowOf(t, p, path).ctl)
	}
	return c
}

func TestTheScrollButtonIsPickedByNameOrByCode(t *testing.T) {
	k := lysWMKit(t, "")
	p := layoutPage(t, k, "wm-pointer")
	const path = "input.pointer.scroll_button"
	c := scrollButtonOf(t, p, path)
	if c.pick.Selected() != 0 || c.code.Visible() {
		t.Fatalf("an unset scroll button reads choice %d with the code shown %v", c.pick.Selected(), c.code.Visible())
	}
	if !slices.Contains(c.names, "middle") {
		t.Fatalf("lys-wm's buttons read %v", c.names)
	}
	pickChoice(c.pick, slices.Index(c.names, "middle")+1)
	if got := k.wm.value(path); got != "middle" {
		t.Errorf("picking the middle button wrote %v", got)
	}
	pickChoice(c.pick, c.customIndex())
	if !c.code.Visible() {
		t.Fatal("picking a button code hides the code")
	}
	c.code.OnValueChanged(275)
	if got := k.wm.value(path); got != int64(275) {
		t.Errorf("the code 275 wrote %v", got)
	}
	pickChoice(c.pick, 0)
	if got := k.wm.value(path); got != nil || c.code.Visible() {
		t.Errorf("inheriting left %v with the code shown %v", got, c.code.Visible())
	}
}

func TestAScrollButtonCodeInTheConfigFileReadsAsACode(t *testing.T) {
	k := lysWMKit(t, "input:\n  touchpad:\n    scroll_button: 280\n")
	c := scrollButtonOf(t, layoutPage(t, k, "wm-pointer"), "input.touchpad.scroll_button")
	if c.pick.Selected() != c.customIndex() || c.code.Value() != 280 || !c.code.Visible() {
		t.Errorf("the code 280 reads choice %d, code %v shown %v", c.pick.Selected(), c.code.Value(), c.code.Visible())
	}
	if lo, _ := c.code.Range(); lo != 1 {
		t.Errorf("the code starts at %v, want lys-wm's minimum 1", lo)
	}
}
