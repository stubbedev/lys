package settings

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	cornerScreenW = 160
	cornerScreenH = 100
	cornerDotR    = 8
)

var hotCornerKeys = []string{"top_left", "top_right", "bottom_left", "bottom_right"}

func hotCornersRow() rowSpec {
	paths := make([]string, len(hotCornerKeys))
	for i, c := range hotCornerKeys {
		paths[i] = "hot_corners." + c
	}
	return wmField(paths[0], withKey("wm-section-hot-corners"), func(r *rowSpec) {
		r.fullWidth = true
		r.also = paths[1:]
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			slots := make([]slot, len(paths))
			slots[0] = s
			for i := 1; i < len(paths); i++ {
				slots[i] = k.wm.slot(paths[i])
			}
			return newHotCorners(k, paths, slots)
		}
	})
}

type hotCorners struct {
	*widget.Box
	k        *kit
	paths    []string
	slots    []slot
	screen   *cornerScreen
	title    *widget.Label
	editor   *widget.Box
	current  control
	selected int
}

func newHotCorners(k *kit, paths []string, slots []slot) *hotCorners {
	h := &hotCorners{Box: gapBox(widget.Row), k: k, paths: paths, slots: slots}
	h.AddClass("hot-corners")
	h.screen = newCornerScreen(h)
	h.title = k.label("", "setting-label")
	h.editor = gapBox(widget.Column)
	side := gapBox(widget.Column)
	side.Append(h.title, false)
	side.Append(h.editor, false)
	h.AppendAligned(h.screen, false, widget.AlignStart)
	h.AppendAligned(side, true, widget.AlignStart)
	h.choose(0)
	return h
}

func (h *hotCorners) set(i int) bool {
	v := h.slots[i].get()
	on, isBool := v.(bool)
	return v != nil && (!isBool || on)
}

func (h *hotCorners) choose(i int) {
	h.selected = i
	h.title.SetText(locale.T().Get(wmKey(h.paths[i])))
	h.editor.Clear()
	h.current = newWMSwitch(h.k, h.slots[i], wmItemFields(h.k.wm, wmPath(h.paths[i]), nil, "hot_corners.corner"))
	h.editor.Append(h.current, false)
	h.screen.Invalidate()
}

func (h *hotCorners) refresh() {
	h.current.refresh()
	h.screen.Invalidate()
}

type cornerScreen struct {
	widget.Base
	h       *hotCorners
	handles []*cornerHandle
}

type cornerHandle struct {
	widget.Base
	s  *cornerScreen
	at int
}

func newCornerScreen(h *hotCorners) *cornerScreen {
	s := &cornerScreen{Base: widget.NewBase(), h: h}
	for i, p := range h.paths {
		c := &cornerHandle{Base: widget.NewBase(), s: s, at: i}
		c.SetAccessibleName(locale.T().Get(wmKey(p)))
		s.handles = append(s.handles, c)
		widget.SetParents(s, c)
	}
	return s
}

func (s *cornerScreen) dots() []render.Rect {
	b := s.Bounds()
	d := 2 * cornerDotR
	left, right := b.X, b.X+b.W-d
	top, bottom := b.Y, b.Y+b.H-d
	return []render.Rect{{X: left, Y: top, W: d, H: d}, {X: right, Y: top, W: d, H: d}, {X: left, Y: bottom, W: d, H: d}, {X: right, Y: bottom, W: d, H: d}}
}

func (s *cornerScreen) Children() []widget.Widget {
	out := make([]widget.Widget, len(s.handles))
	for i, c := range s.handles {
		out[i] = c
	}
	return out
}

func (s *cornerScreen) Measure(axis widget.Axis, _ int) widget.Sizes {
	if axis == widget.Row {
		return widget.FixedSizes(cornerScreenW)
	}
	return widget.FixedSizes(cornerScreenH)
}

func (s *cornerScreen) Arrange(r render.Rect) {
	s.ArrangeSelf(r)
	for i, d := range s.dots() {
		s.handles[i].Arrange(d)
	}
}

func (s *cornerScreen) Paint(cv *render.Canvas) {
	th := widget.Current()
	b := s.Bounds()
	screen := render.Rect{X: b.X + cornerDotR, Y: b.Y + cornerDotR, W: b.W - 2*cornerDotR, H: b.H - 2*cornerDotR}
	cv.RoundedRect(screen, tileRadius, th.MutedColor())
	cv.RoundedBorder(screen, render.UniformCorners(tileRadius), render.UniformInsets(1), th.Border)
	for i, d := range s.dots() {
		fill := th.CardColor()
		if s.h.set(i) {
			fill = th.Accent
		}
		cv.RoundedRect(d, cornerDotR, fill)
		ring, width := th.Border, 1
		if i == s.h.selected {
			ring, width = th.Text, choiceRing
		}
		if widget.FocusVisible(s.handles[i]) {
			ring, width = th.Text, focusRing
		}
		cv.RoundedBorder(d, render.UniformCorners(cornerDotR), render.UniformInsets(width), ring)
	}
}

func (s *cornerScreen) HitTest(p widget.Point) widget.Widget { return s.HitLeaf(s, p) }

func (s *cornerScreen) PressAt(p widget.Point) {
	for i, d := range s.dots() {
		if d.Contains(p.X, p.Y) {
			s.h.choose(i)
			return
		}
	}
}

func (s *cornerScreen) FocusMoved(_, _ widget.Widget) { s.Invalidate() }

func (c *cornerHandle) Measure(widget.Axis, int) widget.Sizes { return widget.FixedSizes(0) }

func (c *cornerHandle) Paint(*render.Canvas) {}

func (c *cornerHandle) HitTest(widget.Point) widget.Widget { return nil }

func (c *cornerHandle) Role() widget.Role { return widget.RoleButton }

func (c *cornerHandle) KeyAction(a widget.KeyAction, _ widget.Mods) {
	if a == widget.KeyEnter || a == widget.KeySpace {
		c.s.h.choose(c.at)
	}
}
