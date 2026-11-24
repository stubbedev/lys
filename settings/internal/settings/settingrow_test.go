package settings

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/displays"
	"github.com/stubbedev/lys/settings/internal/locale"
)

// walkAncestry is walkOwn with the widgets above each, from the root
// down.
func walkAncestry(w widget.Widget, above []widget.Widget, visit func(w widget.Widget, above []widget.Widget)) {
	visit(w, above)
	c, ok := w.(interface{ Children() []widget.Widget })
	if !ok || othersInside(w) {
		return
	}
	above = append(above, w)
	for _, k := range c.Children() {
		walkAncestry(k, slices.Clip(above), visit)
	}
}

// rowDescription is row's own description label: the first in its tree.
func rowDescription(row rowWidget) *widget.Label {
	return treetest.WithClass(row, "description")[0].(*widget.Label)
}

// rowsAreTheRowType holds on every page: every row is the row type (an
// item no row type builds is a hand-built row), every shown row's
// description wraps to two lines at most, and every row sits in an item
// group on a card. Each of titles is a row some page shows.
func rowsAreTheRowType(titles ...string) pageCheck {
	rows := 0
	met := map[string]bool{}
	return pageCheck{visit: func(t *testing.T, w *window) {
		checkRowsAreTheRowType(t, w.pageID, w.page, func(row rowWidget) {
			rows++
			met[row.asRow().Title()] = true
		})
	}, done: func(t *testing.T) {
		if rows == 0 {
			t.Error("no page laid out a row")
		}
		for _, title := range titles {
			if !met[title] {
				t.Errorf("no page showed %q as a row", title)
			}
		}
	}}
}

// checkRowsAreTheRowType checks the rows under root, laid out, as
// rowsAreTheRowType does, handing each shown one to each.
func checkRowsAreTheRowType(t *testing.T, id string, root widget.Widget, each func(rowWidget)) {
	t.Helper()
	walkAncestry(root, nil, func(x widget.Widget, above []widget.Widget) {
		if _, ok := x.(*insetRow); ok {
			return
		}
		row, isRow := x.(rowWidget)
		if !isRow {
			if el, ok := x.(interface{ Element() string }); ok && el.Element() == "item" {
				t.Errorf("%s: a %T is a row no row type builds", id, x)
			}
			return
		}
		if !shown(row) {
			return
		}
		each(row)
		if d := rowDescription(row); shown(d) {
			line := d.Measure(widget.Column, 1<<20).Nat
			if h := d.Bounds().H; h > 2*line {
				t.Errorf("%s: %q's description is %dpx tall, more than two %dpx lines", id, row.asRow().Title(), h, line)
			}
		}
		if !onCard(above) {
			t.Errorf("%s: the %q row is in no item group on a card", id, row.asRow().Title())
		}
	})
}

// onCard reports the widgets above a row holding it in an item group
// that is on a card.
func onCard(above []widget.Widget) bool {
	for i, a := range slices.Backward(above) {
		if _, ok := a.(*widget.ItemGroup); !ok {
			continue
		}
		for _, a := range above[:i] {
			if widget.HasClass(a, css.Card) {
				return true
			}
		}
		return false
	}
	return false
}

// resetNeverMovesTheControl holds on every page: showing every setting
// row's reset button leaves every control where it was with them hidden.
func resetNeverMovesTheControl() pageCheck {
	return pageCheck{visit: func(t *testing.T, w *window) {
		at := func(on bool) map[*fieldRow]render.Rect {
			out := map[*fieldRow]render.Rect{}
			for _, r := range w.page.rows {
				r.reset.show(on)
			}
			w.root.Arrange(render.Rect{W: sweepW, H: sweepH})
			for _, r := range w.page.rows {
				if shown(r.ctl) {
					out[r] = r.ctl.(widget.Boundser).Bounds()
				}
			}
			return out
		}
		hidden, showing := at(false), at(true)
		for r, b := range hidden {
			if got := showing[r]; got != b {
				t.Errorf("%s: showing %s's reset moved its control from %v to %v", w.pageID, r.path, b, got)
			}
		}
		for _, r := range w.page.rows {
			r.refreshSource()
		}
		w.root.Arrange(render.Rect{W: sweepW, H: sweepH})
	}}
}

// columnEdges is the x span of each control in a column, keyed for the
// error it names.
type columnEdges map[string]render.Rect

// shareEdges reports every control of edges not spanning the first's x.
func (edges columnEdges) shareEdges(t *testing.T, what string) {
	t.Helper()
	if len(edges) < 2 {
		t.Fatalf("%s: %d controls in the column, want several", what, len(edges))
	}
	var ref render.Rect
	var refName string
	for name, b := range edges {
		if refName == "" || name < refName {
			ref, refName = b, name
		}
	}
	for name, b := range edges {
		if b.X != ref.X || b.W != ref.W {
			t.Errorf("%s: %s spans x %d..%d, %s %d..%d", what, name, b.X, b.X+b.W, refName, ref.X, ref.X+ref.W)
		}
	}
}

// On Mouse & Touchpad every card's dropdowns, spins and entries start
// and end at one x, whatever their choices; a control marked full-width
// takes the row's width under its text instead.
func TestPointerCardsShareOneControlColumn(t *testing.T) {
	p := laidOutPage(t, "", "wm-pointer", sweepW).page
	cards, wide := 0, 0
	for _, g := range treetest.All[*widget.ItemGroup](p) {
		edges := columnEdges{}
		for _, r := range p.rows {
			if !shown(r) || !treetest.Contains(g, r) {
				continue
			}
			b := r.ctl.(widget.Boundser).Bounds()
			switch r.ControlFit() {
			case widget.ItemControlColumn:
				edges[r.path] = b
			case widget.ItemControlWide:
				wide++
				if rb := r.Bounds(); b.W < rb.W*3/4 {
					t.Errorf("full-width %s is %dpx wide in its %dpx row", r.path, b.W, rb.W)
				}
			}
		}
		if len(edges) > 1 {
			cards++
			edges.shareEdges(t, fmt.Sprintf("wm-pointer card %d", cards))
		}
	}
	if cards == 0 || wide == 0 {
		t.Fatalf("%d cards with a column and %d full-width controls, want both", cards, wide)
	}
}

// The night light's switch is the first row of its section's card, the
// temperature row's card, not a row on its own above it.
func TestNightLightSwitchJoinsTheTemperatureCard(t *testing.T) {
	k := testKit(t, "")
	p := layoutPage(t, k, displaysID)
	sw := treetest.All[*shellSwitch[bool]](p)
	if len(sw) != 1 {
		t.Fatalf("%d night light switches on the page, want 1", len(sw))
	}
	var temperature *fieldRow
	for _, r := range p.rows {
		if r.path == nightlightPath+".temperature" {
			temperature = r
		}
	}
	if temperature == nil {
		t.Fatal("no temperature row")
	}
	for _, g := range treetest.All[*widget.ItemGroup](p) {
		if treetest.Contains(g, temperature) {
			if !treetest.Contains(g, sw[0]) {
				t.Error("the night light switch is not in the temperature row's card")
			}
			return
		}
	}
	t.Fatal("the temperature row is in no item group")
}

// On Warning Levels every card's spins and color dropdowns start and end
// at one x, beside labels sharing one column.
func TestWarningLevelCardsShareOneControlColumn(t *testing.T) {
	p := laidOutPage(t, geometryYAML, thresholdsPageID, sweepW).page
	cards := 0
	for _, body := range treetest.WithClass(p, "card-form-body") {
		edges := columnEdges{}
		labels := map[int]bool{}
		for i, row := range treetest.WithClass(body, "threshold-row") {
			kids := row.(*widget.Box).Children()
			edges[fmt.Sprintf("field %d", i)] = kids[1].(widget.Boundser).Bounds()
			labels[kids[0].(widget.Boundser).Bounds().W] = true
		}
		if len(edges) == 0 {
			continue
		}
		cards++
		edges.shareEdges(t, fmt.Sprintf("warning level card %d", cards))
		if len(labels) != 1 {
			t.Errorf("warning level card %d: label widths %v, want one", cards, labels)
		}
	}
	if cards == 0 {
		t.Fatal("no warning level card laid out")
	}
}

// styledLayOut lays w out w pixels wide under the shell's stylesheet.
func styledLayOut(k *kit, root *widget.Box, w int) {
	apptheme.New(k.store.svc.Config()).Attach(root)
	sz := widget.SizeWithin(root, widget.Size{W: w, H: sweepH})
	root.Arrange(render.Rect{W: w, H: sz.H})
}

// On Displays, for a display offering no mode, the empty resolution and
// refresh rate dropdowns hold the card's control column, not the stub their content asks for, and the
// scale picker takes its row's width under the text instead.
func TestDisplaysDropdownsHoldTheColumnWhenEmpty(t *testing.T) {
	k := testKit(t, "")
	fakeHeads(t, []displays.Head{{Name: "DP-1", Enabled: true, Scale: 1}}, nil)
	p := displaysLead(k).(*displaysPanel)
	t.Cleanup(p.close)
	box := widget.NewBox(widget.Column, 0, 0)
	box.Append(p, false)
	styledLayOut(k, box, 900)
	columnEdges{"resolution": p.resolution.Bounds(), "refresh rate": p.refresh.Bounds()}.shareEdges(t, "displays")
	if w := p.resolution.Bounds().W; w < widget.ItemColumnWidth || w <= p.resolution.Measure(widget.Row, widget.Unknown).Nat {
		t.Errorf("the empty resolution dropdown is %dpx wide, want the column's, at least %d, past its content's %d", w, widget.ItemColumnWidth, p.resolution.Measure(widget.Row, widget.Unknown).Nat)
	}
	if s, r := p.scale.Bounds(), p.resolution.Bounds(); s.W <= r.W || s.X >= r.X {
		t.Errorf("the scale picker at %v did not take its row's width beside the column at %v", s, r)
	}
}

// The Sound page's device dropdown with no devices to list is as wide
// as the control column.
func TestAnEmptySoundDeviceDropdownHoldsTheColumn(t *testing.T) {
	k := testKit(t, "")
	c := newDeviceControls(&soundPanel{k: k}, pulse.DeviceOutput)
	c.show(nil, pulse.Device{}, true, nil)
	styledLayOut(k, c.body, 900)
	if w := c.devices.Bounds().W; w != widget.ItemColumnWidth || w <= c.devices.Measure(widget.Row, widget.Unknown).Nat {
		t.Errorf("the empty device dropdown is %dpx wide, want the column's %d", w, widget.ItemColumnWidth)
	}
}

// In the password dialog the entries with a reveal button after them
// start and end at one x with the one without: the reveal sits outside
// the control column.
func TestPasswordEntriesLineUpWithAndWithoutTheirReveal(t *testing.T) {
	k := testKit(t, "")
	f := (&usersPanel{k: k}).newPasswordForm()
	box := widget.NewBox(widget.Column, 0, 0)
	box.Append(f, false)
	styledLayOut(k, box, passwordDialogWidth)
	columnEdges{"current": f.current.Bounds(), "new": f.next.Bounds(), "confirm": f.confirm.Bounds()}.shareEdges(t, "password dialog")
	var reveals []*widget.Button
	for _, b := range treetest.All[*widget.Button](f) {
		if shown(b) {
			reveals = append(reveals, b)
		}
	}
	if len(reveals) != 2 {
		t.Fatalf("%d buttons shown in the form, want the two reveals", len(reveals))
	}
	end := f.confirm.Bounds().X + f.confirm.Bounds().W
	for i, b := range reveals {
		if x := b.Bounds().X; x < end {
			t.Errorf("reveal %d starts at x %d, inside the column ending at %d", i, x, end)
		}
	}
}

// A choice editor is a field of its card's control column: its combobox
// starts and ends at the x an entry of the card does, and with the
// unknown badge beside it the editor still ends at the column's end.
func TestAChoiceEditorFitsTheControlColumn(t *testing.T) {
	k := testKit(t, "")
	value := "foot"
	s := slot{get: func() any { return value }, set: func(v any) error { value, _ = v.(string); return nil }, unset: func() { value = "" }}
	choice := newChoiceEditor(k, s, choiceSpec{list: fixed("foot", "kitty")})
	entry := k.entry()
	rows := widget.NewItemGroup(k.row("Terminal", "").withControl(choice), k.row("Name", "").withControl(entry))
	box := widget.NewBox(widget.Column, 0, 0)
	box.Append(rows, false)
	styledLayOut(k, box, 900)
	columnEdges{"choice": choice.combo.Bounds(), "entry": entry.Bounds()}.shareEdges(t, "a known choice")
	value = "unlisted"
	choice.refresh()
	styledLayOut(k, box, 900)
	if !choice.unknown.Visible() {
		t.Fatal("an unlisted value shows no unknown badge")
	}
	columnEdges{"choice": choice.Bounds(), "entry": entry.Bounds()}.shareEdges(t, "an unknown choice")
	if c, e := choice.combo.Bounds(), entry.Bounds(); c.X+c.W != e.X+e.W {
		t.Errorf("beside the badge the combobox ends at x %d, the column at %d", c.X+c.W, e.X+e.W)
	}
}

// A device picker with its level under it is a field of its card's
// control column: its dropdown and its level start and end at one x with
// an entry of the card, for a device list and for a choice editor alike.
func TestALevelledDeviceFitsTheControlColumn(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		opts       []rowOpt
		dropdown   func(*levelledDevice) widget.Boundser
	}{
		{"microphone", "modules.recorder.microphone-device", []rowOpt{microphoneDevice}, func(c *levelledDevice) widget.Boundser { return c.pick.(*choiceControl).pick.drop }},
		{"cava source", "modules.cava.source", nil, func(c *levelledDevice) widget.Boundser { return c.pick.(*choiceEditor).combo }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, k, _ := levelledRow(t, soundServer(t), "", tc.path, tc.opts...)
			entry := k.entry()
			row := k.row("Device", "").withControl(c)
			rows := widget.NewItemGroup(row, k.row("Name", "").withControl(entry))
			box := widget.NewBox(widget.Column, 0, 0)
			box.Append(rows, false)
			styledLayOut(k, box, 900)
			if fit := row.ControlFit(); fit != widget.ItemControlColumn {
				t.Fatalf("the %s row holds its picker at fit %d, want the column", tc.name, fit)
			}
			if !shown(c.level.meter) {
				t.Fatal("the level meter is not shown")
			}
			columnEdges{"device": c.Bounds(), "dropdown": tc.dropdown(c).Bounds(), "level": c.level.meter.Bounds(), "entry": entry.Bounds()}.shareEdges(t, tc.name)
		})
	}
}

func TestALevelledDevicesFailureLeavesTheControlColumnItsWidth(t *testing.T) {
	const path = "modules.recorder.microphone-device"
	layOut := func(k *kit, c *levelledDevice) (column int, entry widget.Boundser) {
		e := k.entry()
		rows := widget.NewItemGroup(k.row("Device", "").withControl(c), k.row("Name", "").withControl(e))
		box := widget.NewBox(widget.Column, 0, 0)
		box.Append(rows, false)
		styledLayOut(k, box, 900)
		return c.Bounds().W, e
	}
	working, wk, _ := levelledRow(t, soundServer(t), "", path, microphoneDevice)
	wantColumn, _ := layOut(wk, working)
	if shown(working.level.state) {
		t.Fatal("a working level shows a failure")
	}

	before := openPulse
	unreachable := errors.New("no sound server answered on any of the sockets this test points at, which makes for a long line of text")
	openPulse = func(context.Context) (*pulse.Service, error) { return nil, unreachable }
	t.Cleanup(func() { openPulse = before })
	k := testKit(t, "")
	failing := fieldControl(k, k.store, field(path, microphoneDevice)).(*levelledDevice)
	t.Cleanup(failing.close)
	column, entry := layOut(k, failing)
	if !shown(failing.level.state) || failing.level.state.Text() != locale.T().Get("settings-sound-unreachable") {
		t.Fatalf("an unreachable server shows %q, shown %v", failing.level.state.Text(), shown(failing.level.state))
	}
	if column != wantColumn {
		t.Errorf("the failure widened the control column from %d to %d", wantColumn, column)
	}
	columnEdges{"device": failing.Bounds(), "entry": entry.Bounds()}.shareEdges(t, "a failed level")
	if st := failing.level.state.Bounds(); st.X >= failing.Bounds().X {
		t.Errorf("the failure sits in the control column at x %d, the column starts at %d; want it under the row's text", st.X, failing.Bounds().X)
	}
}

// The composite editors outside the control column keep their own width
// at the row's end beside a card's column, however wide the column: a
// path field and its browse button, a button choice with its code spin, a
// size's mode and spin with or without a slider, the sleep action over
// its lines, a soft range's slider and an offset's two.
func TestCompositeEditorsKeepTheirOwnWidth(t *testing.T) {
	k := testKit(t, "")
	stored := func(v any) slot { return memSlot(&v) }
	whole := config.FieldMeta{Min: 0, Max: 1000}
	editors := []struct {
		name string
		ctl  widget.Widget
	}{
		{"path", newFileEditor(k, stored("/usr/share/backgrounds/default.png"))},
		{"button", newNamedOrWhole(k, stored(int64(275)), config.FieldMeta{Variants: []string{"left", "right", "middle"}, Min: 0, Max: 1000})},
		{"size", newSizeEditor(k, stored(1.0), 1)},
		{"size slider", newSizeSliderEditor(k, stored(1.0), 1, 100, 800)},
		{"sleep action", fieldControl(k, k.store, field("idle.suspend-action", sleepActionRow))},
		{"soft range", newSoftRange(k, stored(int64(10)), whole, 0, 100, 1, unitPixels)},
		{"offset", newOffsetEditor(k, stored([]any{int64(0), int64(4)}), "decoration.shadow.offset", -32, 32)},
	}
	for _, e := range editors {
		t.Run(e.name, func(t *testing.T) {
			wide := k.entry()
			wide.SetWidthChars(150, 150)
			row := k.row(e.name, "").withControl(e.ctl)
			rows := widget.NewItemGroup(row, k.row("Name", "").withControl(wide))
			box := widget.NewBox(widget.Column, 0, 0)
			box.Append(rows, false)
			styledLayOut(k, box, 3200)
			if fit := row.ControlFit(); fit != widget.ItemControlNatural {
				t.Fatalf("the %s row holds its editor at fit %d, want its own width", e.name, fit)
			}
			b, w := e.ctl.(widget.Boundser).Bounds(), wide.Bounds()
			if nat := e.ctl.Measure(widget.Row, widget.Unknown).Nat; b.W != nat {
				t.Errorf("the %s editor is %dpx wide, not its own %dpx", e.name, b.W, nat)
			}
			if b.W >= w.W {
				t.Errorf("the %s editor is %dpx wide, stretched to the %dpx column", e.name, b.W, w.W)
			}
			if b.X+b.W != w.X+w.W {
				t.Errorf("the %s editor ends at x %d, not at the row's end %d", e.name, b.X+b.W, w.X+w.W)
			}
		})
	}
}
