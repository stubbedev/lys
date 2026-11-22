package settings

import (
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// sliderRow is a shown slider of a page, the row it reads in, the item
// group the row is in, and the control holding it: its field row's, or
// the slider itself in another row.
type sliderRow struct {
	slider *widgets.DebouncedSlider
	row    *settingRow
	group  *widget.ItemGroup
	ctl    widget.Widget
}

// shownSliders are the shown sliders under page, each with its row and
// group.
func shownSliders(page widget.Widget) []sliderRow {
	var out []sliderRow
	walkAncestry(page, nil, func(x widget.Widget, above []widget.Widget) {
		d := embeddedSlider(x)
		if d == nil || !shown(d) {
			return
		}
		s := sliderRow{slider: d, ctl: d}
		for _, a := range slices.Backward(above) {
			if r, ok := a.(rowWidget); ok && s.row == nil {
				s.row = r.asRow()
				if f, ok := r.(*fieldRow); ok {
					s.ctl = f.ctl
				}
			}
			if g, ok := a.(*widget.ItemGroup); ok && s.group == nil {
				s.group = g
			}
		}
		out = append(out, s)
	})
	return out
}

// embeddedSlider is the slider x is, or the one it is built on.
func embeddedSlider(x widget.Widget) *widgets.DebouncedSlider {
	switch c := x.(type) {
	case *widgets.DebouncedSlider:
		return c
	case *slider:
		return c.DebouncedSlider
	case *softRange:
		return c.DebouncedSlider
	}
	return nil
}

// reading is a slider's reading: the slot after its knob.
func reading(d *widgets.DebouncedSlider) widget.Boundser {
	kids := d.Children()
	return kids[len(kids)-1].(widget.Boundser)
}

// The slider rows of every section, more than one to a section, end
// their tracks at one x and read in one width; a row whose control takes
// the line under its text is not in the section's column.
func sliderSectionsShareOneValueColumn() pageCheck {
	sections := map[string]int{}
	return pageCheck{visit: func(t *testing.T, w *window) {
		groups := map[*widget.ItemGroup][]sliderRow{}
		for _, s := range shownSliders(w.page) {
			if s.row == nil || s.group == nil || s.row.ControlFit() == widget.ItemControlWide {
				continue
			}
			groups[s.group] = append(groups[s.group], s)
		}
		for _, rows := range groups {
			if len(rows) < 2 {
				continue
			}
			sections[w.pageID]++
			ref := rows[0]
			end := func(s sliderRow) int { b := s.slider.Knob.Bounds(); return b.X + b.W }
			for _, s := range rows[1:] {
				if end(s) != end(ref) {
					t.Errorf("%s: %q's track ends at x=%d, %q's at %d", w.pageID, s.row.Title(), end(s), ref.row.Title(), end(ref))
				}
				if got, want := reading(s.slider).Bounds().W, reading(ref.slider).Bounds().W; got != want {
					t.Errorf("%s: %q reads in %dpx, %q in %d", w.pageID, s.row.Title(), got, ref.row.Title(), want)
				}
			}
		}
	}, done: func(t *testing.T) {
		for _, id := range []string{"motion", "bar", "wm-keyboard"} {
			if sections[id] == 0 {
				t.Errorf("%s laid out no section of several slider rows; sections checked: %v", id, sections)
			}
		}
	}}
}

// valueReadouts are the shown texts and fields under ctl that hold a
// number: the labels with a digit in them, and the spins.
func valueReadouts(ctl widget.Widget) []widget.Widget {
	var out []widget.Widget
	walkOwn(ctl, func(x widget.Widget) {
		if !shown(x) {
			return
		}
		switch c := x.(type) {
		case *widget.SpinButton, *number:
			out = append(out, c)
		case *widget.Label:
			if strings.IndexFunc(c.Text(), unicode.IsDigit) >= 0 {
				out = append(out, c)
			}
		}
	})
	return out
}

// Every slider row on every page reads each of its sliders' values once:
// one text or one field holding it, never a label and a field both.
func sliderRowsReadTheirValueOnce() pageCheck {
	rows := 0
	return pageCheck{visit: func(t *testing.T, w *window) {
		per := map[widget.Widget]int{}
		titles := map[widget.Widget]string{}
		for _, s := range shownSliders(w.page) {
			if s.row != nil {
				per[s.ctl]++
				titles[s.ctl] = s.row.Title()
			}
		}
		for ctl, sliders := range per {
			rows++
			if got := valueReadouts(ctl); len(got) != sliders {
				t.Errorf("%s: %q reads %d slider values in %d texts and fields, its labels %q", w.pageID, titles[ctl], sliders, len(got), treetest.Labels(ctl))
			}
		}
	}, done: func(t *testing.T) {
		if rows == 0 {
			t.Error("no page laid out a slider row")
		}
	}}
}

// A soft range reads its value once, in its field, with the unit as the
// field's addon, even past the slider's end; a drag shows the knob's
// value there as it moves.
func TestASoftRangeReadsItsValueOnceInItsField(t *testing.T) {
	k := testKit(t, "animations:\n  duration: 5000\n")
	c := fieldControl(k, k.store, field("animations.duration", animationDuration)).(*softRange)
	readouts := valueReadouts(laidOut(c))
	if len(readouts) != 1 || readouts[0] != widget.Widget(c.number) {
		t.Fatalf("5 s reads in %v, want the one field", readouts)
	}
	if c.number.Value() != 5000 || c.Knob.Value() != animationSliderMS {
		t.Errorf("5 s reads %v with the knob at %v", c.number.Value(), c.Knob.Value())
	}
	if labels := treetest.Labels(c); len(labels) != 1 || labels[0] != unitMilliseconds.addon() {
		t.Errorf("the field's addons read %q, want the unit %q alone", labels, unitMilliseconds.addon())
	}
	c.Knob.SetPressed(true)
	c.Knob.SetValue(300)
	c.Knob.SetPressed(false)
	if c.number.Value() != 300 || k.store.value("animations.duration") != int64(300) {
		t.Errorf("a drag to 300 reads %v and stored %v", c.number.Value(), k.store.value("animations.duration"))
	}
}

// laidOut lays w out at its natural size and returns it.
func laidOut(w widget.Widget) widget.Widget {
	sz := widget.SizeWithin(w, widget.Size{W: 2000, H: 400})
	w.Arrange(render.Rect{W: sz.W, H: sz.H})
	return w
}

// A number in a unit reads the unit once: a slider in its label, a
// field as its addon; the zero unit is a bare count, and a unit without
// a value to render is a bug.
func TestAUnitReadsOnceOnSlidersAndFields(t *testing.T) {
	if got := strings.NewReplacer("\u2068", "", "\u2069", "").Replace(unitPixels.format(0)(16)); got != "16 px" {
		t.Errorf("16 pixels read %q", got)
	}
	if got := bareCount.format(2)(0.5); got != "0.50" || bareCount.addon() != "" {
		t.Errorf("a bare count reads %q with the addon %q", got, bareCount.addon())
	}
	k := testKit(t, "")
	n := k.numberField(newNumber(k, slot{get: func() any { return int64(300) }, set: func(any) error { return nil }}, 0, 5000, 1, 0), unitMilliseconds)
	if labels := treetest.Labels(n); len(labels) != 1 || labels[0] != unitMilliseconds.addon() {
		t.Errorf("a field in milliseconds reads %q beside it", labels)
	}
	if _, ok := k.numberField(newNumber(k, slot{get: func() any { return int64(3) }}, 0, 10, 1, 0), bareCount).(*number); !ok {
		t.Error("a bare count's field is not the number alone")
	}
	defer func() {
		if recover() == nil {
			t.Error("a unit with no value to render formatted one")
		}
	}()
	unitSeconds.format(0)
}

// At the window's default width the keyboard's repeat rows and the bar's
// hide delay read their whole titles beside their sliders.
func TestSliderRowTitlesReadWholeAtTheDefaultWidth(t *testing.T) {
	for _, c := range []struct {
		yaml, page string
		paths      []string
	}{
		{"", "wm-keyboard", []string{"input.keyboard.repeat_rate", "input.keyboard.repeat_delay"}},
		{"bar:\n  auto-hide: true\n", "bar", []string{"bar.auto-hide-delay-ms"}},
	} {
		win := laidOutPage(t, c.yaml, c.page, windowWidth)
		for _, path := range c.paths {
			row := rowOf(t, win.page, path)
			title := treetest.WithClass(row, "title")[0].(*widget.Label)
			if !shown(title) {
				t.Fatalf("%s: %s's title is not shown", c.page, path)
			}
			if w, whole := title.Bounds().W, widget.SizeWithin(title, widget.Size{W: 1 << 20, H: 1 << 20}).W; w < whole {
				t.Errorf("%s: %q is cut to %dpx of its %dpx", c.page, title.Text(), w, whole)
			}
		}
	}
}

// The balance reads the side it leans to and how far, or 0 at the
// centre, in a column as wide as its widest reading.
func TestTheBalanceReadsItsSide(t *testing.T) {
	bare := strings.NewReplacer("\u2068", "", "\u2069", "")
	for b, want := range map[float64]string{-20: "L 20", 0: "0", 0.4: "0", 35: "R 35", balanceRange: "R 100"} {
		if got := bare.Replace(balanceLabel(b)); got != want {
			t.Errorf("a balance of %v reads %q, want %q", b, got, want)
		}
	}
	k := testKit(t, "")
	d := k.valueSlider(-balanceRange, balanceRange, balanceLabel)
	d.Set(0)
	centre := reading(laidOut(d).(*widgets.DebouncedSlider)).Bounds().W
	d.Set(-balanceRange)
	if left := reading(laidOut(d).(*widgets.DebouncedSlider)).Bounds().W; d.ValueLabel().Text() != balanceLabel(-balanceRange) || left != centre {
		t.Errorf("full left reads %q in %dpx, the centre in %dpx", d.ValueLabel().Text(), left, centre)
	}
}
