package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// pressAt clicks the router's root at p, the real input path.
func pressAt(r *widget.Router, p widget.Point) {
	r.Move(p)
	r.Press(widget.BTNLeft, p, 0)
	r.Release(widget.BTNLeft, p)
}

// midpoint is the middle of w's bounds.
func midpoint(w widget.Boundser) widget.Point {
	b := w.Bounds()
	return widget.Point{X: b.X + b.W/2, Y: b.Y + b.H/2}
}

// A collapsible section opens and closes from a click anywhere on its
// header, its title, its description, its chevron and the space between,
// and from Enter and Space with the header focused: its card shows only
// while open, its chevron is turned over only while open, and the header
// is one button named by the section's title. A click on a control in
// the open card leaves the section open.
func TestACollapsibleSectionOpensFromAnywhereOnItsHeaderAndByKeys(t *testing.T) {
	k := testKit(t, "")
	sw := widget.NewSwitch(false)
	section, group, fold := collapsibleSection(k, "settings-section-durations", widget.NewItemGroup(k.row("Switch", "").withControl(sw)))
	root := widget.NewBox(widget.Column, 0, 0)
	root.Append(section, false)
	styledLayOut(k, root, 900)
	r := &widget.Router{Root: root}
	check := func(when string, open bool) {
		t.Helper()
		styledLayOut(k, root, 900)
		if fold.isOpen() != open || group.Visible() != open || fold.chevron.HasState(widget.StateChecked) != open {
			t.Fatalf("%s: open %v, card shown %v, chevron turned %v; want all %v", when, fold.isOpen(), group.Visible(), fold.chevron.HasState(widget.StateChecked), open)
		}
	}
	check("built", false)
	if a := widget.Describe(fold.button); a.Role != widget.RoleButton || a.Name != locale.T().Get("settings-section-durations") || !a.TabStop {
		t.Fatalf("the header reads as %+v, want one button named by the title", a)
	}
	head := fold.button.Bounds()
	title := treetest.WithClass(section, "settings-section-title")[0].(*widget.Label)
	desc := treetest.WithClass(section, "settings-section-description")[0].(*widget.Label)
	gap := widget.Point{X: (title.Bounds().X + title.Bounds().W + fold.chevron.Bounds().X) / 2, Y: midpoint(title).Y}
	open := false
	for name, p := range map[string]widget.Point{
		"title":       midpoint(title),
		"description": midpoint(desc),
		"chevron":     midpoint(fold.chevron),
		"between":     gap,
		"right edge":  {X: head.X + head.W - 1, Y: head.Y + head.H/2},
	} {
		pressAt(r, p)
		open = !open
		check("a click on the "+name, open)
	}
	if err := r.SetFocus(fold.button); err != nil {
		t.Fatal(err)
	}
	r.KeyAction(widget.KeyEnter, 0)
	open = !open
	check("Enter", open)
	r.Type(' ')
	open = !open
	check("Space", open)
	if !open {
		r.KeyAction(widget.KeyEnter, 0)
		check("Enter again", true)
	}
	pressAt(r, midpoint(sw))
	check("a click on the card's switch", true)
	if !sw.On() {
		t.Error("the click on the open card's switch did not reach it")
	}
}

// No section header carries a separate text button beside its title,
// the "Show" and "Hide" that once folded a section away: a section heads
// itself with its title, or is folded under its header, which is the one
// button there.
func sectionHeadersCarryNoTextButton() pageCheck {
	return pageCheck{visit: func(t *testing.T, w *window) {
		for _, s := range treetest.WithClass(w.page, "settings-section") {
			kids := s.(interface{ Children() []widget.Widget }).Children()
			if len(kids) == 0 {
				continue
			}
			head := kids[0]
			for _, b := range treetest.All[*widget.Button](head) {
				if b == head {
					continue
				}
				t.Errorf("%s: the header of the section %q carries the button %q", w.pageID, treetest.Labels(head), treetest.Labels(b))
			}
		}
	}}
}

// A device card and a card-form card fold away under one disclosure
// type, and each builds its body once, on the first open, however often
// it is opened and closed after.
func TestDeviceAndFormCardsShareOneDisclosureAndFillOnce(t *testing.T) {
	var _ *disclosure = (&deviceCard{}).disclose
	var _ *disclosure = (&card{}).disclose

	dk := deviceKit(t, deviceConfig, someDevices)
	device := cardNamed(t, devicesOf(t, dk, wmPointerPage()), "Synaptics TM3276")
	if device.rows != nil || device.body.Visible() {
		t.Fatal("a closed device card built or shows its rows")
	}
	device.disclose.button.ClickAt(widget.Point{})
	built := len(device.body.Children())
	if built == 0 || !device.body.Visible() || !device.disclose.chevron.HasState(widget.StateChecked) {
		t.Fatalf("opened, the device card holds %d rows, shows %v", built, device.body.Visible())
	}
	device.disclose.button.ClickAt(widget.Point{})
	device.disclose.button.ClickAt(widget.Point{})
	if n := len(device.body.Children()); n != built || !device.body.Visible() {
		t.Errorf("reopened, the device card holds %d rows of %d", n, built)
	}

	k, w, _, _ := wmKit(t, "rules:\n  - {app_id: foot}\n")
	c := fieldControl(k, w, wmObjects("rules", "app_id")).(*cardList)
	form := c.cards[0]
	if len(form.controls) != 0 {
		t.Fatal("a closed card-form card built its fields")
	}
	form.disclose.button.ClickAt(widget.Point{})
	fields := len(form.controls)
	if fields == 0 || !form.disclose.chevron.HasState(widget.StateChecked) {
		t.Fatalf("opened, the card-form card holds %d fields", fields)
	}
	form.disclose.button.ClickAt(widget.Point{})
	if form.disclose.chevron.HasState(widget.StateChecked) {
		t.Error("closed, the chevron stays turned over")
	}
	form.disclose.button.ClickAt(widget.Point{})
	if len(form.controls) != fields {
		t.Errorf("reopened, the card-form card holds %d fields of %d", len(form.controls), fields)
	}
}

// Every mode toggle, "Edit as text" among them, reads as a link, the
// accent ink on no fill, and starts its text where the editor holding it
// starts, at the row's inset: the label-builder rows, Custom, Mail and
// World Clock among them.
func modeTogglesAreInsetLinks(accent render.Color) pageCheck {
	seen := map[string]bool{}
	kind := func(above []widget.Widget) string {
		for i := len(above) - 1; i >= 0; i-- {
			switch above[i].(type) {
			case *customModulesEditor:
				return "custom"
			case *mailQueryEditor:
				return "mail"
			case *worldClockEditor:
				return "world clock"
			case *labelBuilder:
				return "label builder"
			}
		}
		return ""
	}
	visit := func(t *testing.T, w *window) {
		walkAncestry(w.page, nil, func(x widget.Widget, above []widget.Widget) {
			m, ok := x.(*modeToggle)
			if !ok || !shown(m) {
				return
			}
			if k := kind(above); k != "" {
				seen[k] = true
			}
			if fill := widget.StyleBackground(m.Button); fill.A() != 0 {
				t.Errorf("%s: the toggle %q rests on fill %#x", w.pageID, m.label.Text(), uint32(fill))
			}
			if ink := widget.CascadeColor(m.label); ink != accent {
				t.Errorf("%s: the toggle %q is inked %#x, want the accent %#x", w.pageID, m.label.Text(), uint32(ink), uint32(accent))
			}
			editor := above[len(above)-1].(widget.Boundser).Bounds()
			if b, l := m.Bounds(), m.label.Bounds(); b.X != editor.X || l.X != editor.X {
				t.Errorf("%s: the toggle %q starts at x=%d, its text at %d, the editor at %d", w.pageID, m.label.Text(), b.X, l.X, editor.X)
			}
		})
	}
	return pageCheck{visit: visit, done: func(t *testing.T) {
		for _, k := range []string{"custom", "mail", "world clock", "label builder"} {
			if !seen[k] {
				t.Errorf("no page showed the %s's mode toggle", k)
			}
		}
	}}
}

// A mode toggle reads its first text until clicked, its second after,
// running flip with each new state.
func TestAModeToggleFlipsBetweenItsTwoTexts(t *testing.T) {
	k := testKit(t, "")
	var flips []bool
	m := k.modeToggle("Parts", "Text", func(on bool) { flips = append(flips, on) })
	if m.isOn() || m.label.Text() != "Parts" || !widget.HasClass(m.Button, css.Link) {
		t.Fatalf("built on %v reading %q, classes %v", m.isOn(), m.label.Text(), m.Classes())
	}
	m.OnClick()
	if !m.isOn() || m.label.Text() != "Text" {
		t.Fatalf("clicked, on %v reading %q", m.isOn(), m.label.Text())
	}
	m.OnClick()
	if m.isOn() || m.label.Text() != "Parts" || len(flips) != 2 || flips[0] != true || flips[1] != false {
		t.Errorf("clicked again, on %v reading %q, flips %v", m.isOn(), m.label.Text(), flips)
	}
}
