package settings

import (
	"cmp"
	"image"
	"math"
	"slices"
	"strings"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/displays"
	"github.com/stubbedev/lys/settings/internal/locale"
)

const monitorRing = 3

type monitorTile struct {
	name  string
	label string
	rect  displays.Rect
}

var builtInConnectors = []string{"eDP", "LVDS", "DSI"}

func headLabel(h displays.Head) string {
	if slices.ContainsFunc(builtInConnectors, func(prefix string) bool { return strings.HasPrefix(h.Name, prefix) }) {
		return locale.T().Get("settings-displays-built-in")
	}
	if model := strings.TrimSpace(h.Make + " " + h.Model); h.Model != "" {
		return model
	}
	return cmp.Or(h.Description, h.Name)
}

func headLabels(heads []displays.Head) map[string]string {
	out := make(map[string]string, len(heads))
	for _, h := range heads {
		out[h.Name] = headLabel(h)
	}
	return out
}

func monitorTiles(heads []displays.Head) []monitorTile {
	var out []monitorTile
	labels := headLabels(heads)
	for _, s := range displays.Settings(heads) {
		if s.Enabled {
			out = append(out, monitorTile{name: s.Name, label: labels[s.Name], rect: displays.Logical(s)})
		}
	}
	return out
}

type monitorPicker struct {
	widget.Base
	arrangement
	k        *kit
	tiles    []monitorTile
	handles  []*monitorHandle
	thumbs   map[string]image.Image
	selected []string
	// multi toggles each display a press chooses, instead of choosing it
	// alone.
	multi  bool
	onPick func(name string)
}

type monitorHandle struct {
	widget.Base
	p    *monitorPicker
	name string
}

func newMonitorPicker(k *kit, onPick func(name string)) *monitorPicker {
	return &monitorPicker{Base: widget.NewBase(), k: k, thumbs: map[string]image.Image{}, onPick: onPick}
}

func (p *monitorPicker) setTiles(tiles []monitorTile) {
	p.tiles = tiles
	p.handles = p.handles[:0]
	for _, t := range tiles {
		h := &monitorHandle{Base: widget.NewBase(), p: p, name: t.name}
		h.SetAccessibleName(t.label + " (" + t.name + ")")
		p.handles = append(p.handles, h)
	}
	for _, h := range p.handles {
		widget.SetParents(p, h)
	}
	p.InvalidateLayout()
	p.Invalidate()
}

func (p *monitorPicker) setThumb(name string, img image.Image) {
	if img == nil {
		delete(p.thumbs, name)
	} else {
		p.thumbs[name] = img
	}
	p.Invalidate()
}

func (p *monitorPicker) isSelected(name string) bool { return slices.Contains(p.selected, name) }

// current is the display a single choice holds, "" before one.
func (p *monitorPicker) current() string {
	if len(p.selected) == 0 {
		return ""
	}
	return p.selected[0]
}

func (p *monitorPicker) setSelected(names []string) {
	p.selected = names
	p.Invalidate()
}

func (p *monitorPicker) choose(name string) {
	if !slices.ContainsFunc(p.tiles, func(t monitorTile) bool { return t.name == name }) {
		return
	}
	switch {
	case !p.multi:
		p.selected = []string{name}
	case p.isSelected(name):
		p.selected = slices.DeleteFunc(slices.Clone(p.selected), func(n string) bool { return n == name })
	default:
		p.selected = append(slices.Clone(p.selected), name)
	}
	p.Invalidate()
	if p.onPick != nil {
		p.onPick(name)
	}
}

func (p *monitorPicker) rects() []displays.Rect {
	out := make([]displays.Rect, len(p.tiles))
	for i, t := range p.tiles {
		out[i] = t.rect
	}
	return out
}

func (p *monitorPicker) Children() []widget.Widget {
	out := make([]widget.Widget, len(p.handles))
	for i, h := range p.handles {
		out[i] = h
	}
	return out
}

func (p *monitorPicker) Measure(axis widget.Axis, _ int) widget.Sizes {
	if axis == widget.Row {
		return widget.Sizes{Min: canvasMinW, Nat: canvasMinW}
	}
	return widget.FixedSizes(canvasHeight)
}

func (p *monitorPicker) Arrange(r render.Rect) {
	p.ArrangeSelf(r)
	p.fit(p.rects(), r)
	for i, h := range p.handles {
		h.Arrange(p.screen(p.tiles[i].rect))
	}
}

func (p *monitorPicker) Paint(cv *render.Canvas) {
	th := widget.Current()
	cv.RoundedRect(p.Bounds(), tileRadius, th.MutedColor())
	corners := render.UniformCorners(tileRadius)
	for i, t := range p.tiles {
		tile := p.screen(t.rect)
		ink := th.Text
		if img, ok := p.thumbs[t.name]; ok {
			cv.DrawImageCover(img, tile, corners)
		} else {
			cv.RoundedRect(tile, tileRadius, th.CardColor())
		}
		if p.isSelected(t.name) {
			cv.RoundedBorder(tile, corners, render.UniformInsets(monitorRing), th.Accent)
		} else if widget.FocusVisible(p.handles[i]) {
			cv.RoundedBorder(tile, corners, render.UniformInsets(focusRing), th.Text)
		}
		px := p.k.textPx()
		w, h := int(math.Ceil(p.k.face.Shape(t.label, px).Advance())), int(math.Ceil(px*1.4))
		pad := th.Padding / 2
		pill := render.Rect{X: tile.X + (tile.W-w)/2 - pad, Y: tile.Y + tile.H - h - 2*pad, W: w + 2*pad, H: h}
		cv.RoundedRect(pill, tileRadius, th.CardColor())
		p.k.face.DrawAligned(cv, t.label, pill, p.k.textPx(), ink, render.AlignCenter)
	}
}

// tileFeed hands its show the enabled displays as tiles, again whenever
// they change, until it is closed; without the output manager it hands
// the outputs the system lists once, laid side by side.
type tileFeed struct {
	mgr    displayManager
	closed bool
}

func watchTiles(k *kit, show func([]monitorTile)) *tileFeed {
	f := &tileFeed{}
	async(k, func() displaysConnection {
		mgr, err := connectDisplays(func(heads []displays.Head) {
			k.invoke(func() {
				if !f.closed {
					show(monitorTiles(heads))
				}
			})
		})
		return displaysConnection{mgr, err}
	}, func(conn displaysConnection) {
		if conn.err != nil {
			var names []string
			for _, ch := range systemLists["Monitor"]() {
				if ch.value != "" {
					names = append(names, ch.value)
				}
			}
			if !f.closed {
				show(outputTiles(names))
			}
			return
		}
		if f.closed {
			conn.mgr.Close()
			return
		}
		f.mgr = conn.mgr
	})
	return f
}

func (f *tileFeed) close() {
	f.closed = true
	if f.mgr != nil {
		f.mgr.Close()
		f.mgr = nil
	}
}

func outputTiles(names []string) []monitorTile {
	out := make([]monitorTile, len(names))
	for i, n := range names {
		out[i] = monitorTile{name: n, rect: displays.Rect{X: i * 1920, W: 1920, H: 1080}}
	}
	return out
}

func (p *monitorPicker) HitTest(pt widget.Point) widget.Widget { return p.HitLeaf(p, pt) }

func (p *monitorPicker) PressAt(pt widget.Point) {
	x, y := p.logical(pt)
	if i := rectAt(p.rects(), x, y); i >= 0 {
		p.choose(p.tiles[i].name)
	}
}

func (p *monitorPicker) FocusMoved(_, _ widget.Widget) { p.Invalidate() }

func (h *monitorHandle) Measure(widget.Axis, int) widget.Sizes { return widget.FixedSizes(0) }

func (h *monitorHandle) Paint(*render.Canvas) {}

func (h *monitorHandle) HitTest(widget.Point) widget.Widget { return nil }

func (h *monitorHandle) Role() widget.Role { return widget.RoleButton }

func (h *monitorHandle) KeyAction(a widget.KeyAction, _ widget.Mods) {
	if a == widget.KeyEnter || a == widget.KeySpace {
		h.p.choose(h.name)
	}
}
