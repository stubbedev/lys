package widget

import (
	"bytes"
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

func TestMenuKeyboard(t *testing.T) {
	items := []MenuItem{
		{Label: "cut", OnClick: func() {}},
		MenuSeparator(),
		{Label: "copy", OnClick: func() {}},
	}
	menu := NewMenu(entryFace(t), 13, items...)
	SizeWithin(menu, Size{W: 300, H: 300})
	menu.Arrange(render.Rect{X: 10, Y: 20, W: 120, H: SizeWithin(menu, Size{W: 400, H: 400}).H})

	t.Run("arrows move and clamp at the ends (pinned)", func(t *testing.T) {
		menu.KeyAction(KeyDown, 0)
		if menu.hovered != 0 {
			t.Errorf("first down hovered %d, want 0 (skipping the separator)", menu.hovered)
		}
		menu.KeyAction(KeyDown, 0)
		if menu.hovered != 2 {
			t.Errorf("second down hovered %d, want 2 (separator skipped)", menu.hovered)
		}
		menu.KeyAction(KeyDown, 0)
		if menu.hovered != 2 {
			t.Errorf("down at bottom hovered %d, want clamped 2", menu.hovered)
		}
		menu.KeyAction(KeyEnd, 0)
		if menu.hovered != 2 {
			t.Errorf("end hovered %d, want 2", menu.hovered)
		}
		menu.KeyAction(KeyUp, 0)
		menu.KeyAction(KeyUp, 0)
		if menu.hovered != 0 {
			t.Errorf("up twice hovered %d, want clamped 0", menu.hovered)
		}
	})

	t.Run("enter fires the hovered row", func(t *testing.T) {
		fired := ""
		menu.items[2].OnClick = func() { fired = "copy" }
		menu.KeyAction(KeyEnd, 0)
		menu.KeyAction(KeyEnter, 0)
		if fired != "copy" {
			t.Errorf("enter fired %q, want copy", fired)
		}
	})

	t.Run("esc dismisses", func(t *testing.T) {
		fired := false
		menu.OnDismiss = func() { fired = true }
		menu.KeyAction(KeyDismiss, 0)
		if !fired {
			t.Error("esc did not dismiss")
		}
	})
}

func TestMenuKinds(t *testing.T) {
	items := []MenuItem{
		{Label: "show grid", Kind: ItemCheck, Checked: true, OnClick: func() {}},
		{Label: "sort a", Kind: ItemRadio, Group: "sort", Checked: true, OnClick: func() {}},
		{Label: "sort b", Kind: ItemRadio, Group: "sort", OnClick: func() {}},
	}
	menu := NewMenu(entryFace(t), 13, items...)
	SizeWithin(menu, Size{W: 300, H: 300})
	menu.Arrange(render.Rect{X: 0, Y: 0, W: 200, H: SizeWithin(menu, Size{W: 400, H: 400}).H})

	t.Run("check rows toggle on activate", func(t *testing.T) {
		menu.ClickAt(rowCenter(menu, 0))
		if menu.items[0].Checked {
			t.Error("check row did not toggle off")
		}
		menu.ClickAt(rowCenter(menu, 0))
		if !menu.items[0].Checked {
			t.Error("check row did not toggle on")
		}
	})

	t.Run("radio rows are exclusive within their group", func(t *testing.T) {
		menu.ClickAt(rowCenter(menu, 2))
		if menu.items[1].Checked {
			t.Error("radio a stayed checked after choosing b")
		}
		if !menu.items[2].Checked {
			t.Error("radio b did not become checked")
		}
	})
}

func TestMenuSubmenu(t *testing.T) {
	sub := []MenuItem{{Label: "child", OnClick: func() {}}}
	items := []MenuItem{
		{Label: "open recent", Items: sub},
		{Label: "quit", OnClick: func() {}},
	}
	menu := NewMenu(entryFace(t), 13, items...)
	SizeWithin(menu, Size{W: 300, H: 300})
	menu.Arrange(render.Rect{X: 0, Y: 0, W: 200, H: SizeWithin(menu, Size{W: 400, H: 400}).H})

	t.Run("activating a submenu row fires OnSubmenu without dismissing", func(t *testing.T) {
		var got []MenuItem
		dismissed := false
		menu.OnSubmenu = func(i int, items []MenuItem) { got = items }
		menu.OnDismiss = func() { dismissed = true }
		menu.ClickAt(rowCenter(menu, 0))
		if len(got) != 1 || got[0].Label != "child" {
			t.Errorf("OnSubmenu items = %v, want the child items", got)
		}
		if dismissed {
			t.Error("opening a submenu dismissed the parent")
		}
	})

	t.Run("Right opens the hovered submenu", func(t *testing.T) {
		var opened bool
		menu.OnSubmenu = func(int, []MenuItem) { opened = true }
		menu.KeyAction(KeyUp, 0) // back to the submenu row
		menu.KeyAction(KeyRight, 0)
		if !opened {
			t.Error("Right did not open the hovered submenu")
		}
	})
}

// A check or radio row's indicator shows in both states: unchecked, a
// ring of the muted ink around the surface, so the row reads as a
// checkbox; checked, filled with the accent around its mark.
func TestAMenuIndicatorShowsWhetherCheckedOrNot(t *testing.T) {
	t.Cleanup(func() { SetTheme(nil) })
	SetTheme(nil)
	th := Current()
	const size = 20
	box := render.Rect{X: 3, Y: 3, W: menuIndicatorPx, H: menuIndicatorPx}
	paint := func(kind ItemKind, on, enabled bool) []byte {
		data := make([]byte, render.Stride(size)*size)
		paintMenuIndicator(render.New(data, render.Stride(size), size, size), kind, on, enabled, box)
		return data
	}
	edge := func(data []byte) render.Color { return pxAt(data, size, box.X+box.W/2, box.Y) }
	centre := func(data []byte) render.Color { return pxAt(data, size, box.X+box.W/2, box.Y+box.H/2) }
	corner := func(data []byte) render.Color { return pxAt(data, size, box.X, box.Y) }

	off := paint(ItemCheck, false, true)
	if got := edge(off); got != th.DisabledText() {
		t.Errorf("an unchecked box's top edge is %v, want the muted ink %v", got, th.DisabledText())
	}
	if got := centre(off); got != th.Bg {
		t.Errorf("an unchecked box's centre is %v, want the surface %v, no mark", got, th.Bg)
	}
	on := paint(ItemCheck, true, true)
	if got := edge(on); got != th.Accent {
		t.Errorf("a checked box's edge is %v, want the accent %v", got, th.Accent)
	}
	if bytes.Equal(on, paint(ItemCheck, true, false)) {
		t.Error("a disabled checked box paints as an enabled one")
	}
	if got := corner(paint(ItemRadio, false, true)); got.A() != 0 {
		t.Errorf("an unchecked radio's corner is painted %v; a radio is round", got)
	}
	if got := centre(paint(ItemRadio, true, true)); got != th.OnAccent {
		t.Errorf("a checked radio's centre is %v, want its dot in %v", got, th.OnAccent)
	}
}
