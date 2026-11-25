package settings

import (
	"log"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// tileKeys make a drawn part of a picture a button: it takes the focus,
// reads as a button, and Enter or Space runs activate.
type tileKeys struct {
	activate func()
}

func (t tileKeys) Role() widget.Role { return widget.RoleButton }

func (t tileKeys) KeyAction(a widget.KeyAction, _ widget.Mods) {
	if a == widget.KeyEnter || a == widget.KeySpace {
		t.activate()
	}
}

// tile is a picture that is its own button: a press anywhere on it, or
// Enter or Space while it has the focus, runs activate, and it repaints
// as the focus comes and goes so it can draw its focus ring. self is the
// widget embedding it, which its hit test finds.
type tile struct {
	widget.Base
	tileKeys
	self widget.Widget
}

// newTile is self's tile, running activate.
func newTile(self widget.Widget, activate func()) tile {
	return tile{Base: widget.NewBase(), activate: activate, self: self}
}

func (t *tile) HitTest(p widget.Point) widget.Widget {
	if t.Bounds().Contains(p.X, p.Y) {
		return t.self
	}
	return nil
}

func (t *tile) PressAt(widget.Point) { t.activate() }

func (t *tile) FocusMoved(_, _ widget.Widget) { t.Invalidate() }

// accentOf is the accent w marks its choice in: the theme's accent, or
// its disabled accent while w or a row holding it is disabled, so a
// disabled row's picture dims with its title.
func accentOf(th *widget.Theme, w widget.Widget) render.Color {
	if widget.IsEnabled(w) {
		return th.Accent
	}
	return th.DisabledAccent()
}

// disabledInk is the text colour a disabled row's title takes from the
// stylesheet, the palette's subtle foreground, for text a picture sets
// in a disabled row.
func (k *kit) disabledInk() render.Color {
	p := k.palette()
	c, ok := p.Token(config.TokenFgSubtle)
	if !ok {
		log.Panicf("settings: the palette resolves no %s", config.TokenFgSubtle)
	}
	return c
}

// spot is the focus stand-in of one spot a picture draws and hit-tests
// itself, a screen corner or a finger: it takes no room and no pointer,
// and Enter or Space while it has the focus runs activate. The picture
// draws its focus ring and repaints as the focus moves among its spots.
type spot[K any] struct {
	widget.Base
	tileKeys
	key K
}

// newSpot is the spot of key, running activate with it.
func newSpot[K any](key K, activate func(K)) *spot[K] {
	return &spot[K]{Base: widget.NewBase(), activate: func() { activate(key) }, key: key}
}

func (s *spot[K]) Measure(widget.Axis, int) widget.Sizes { return widget.FixedSizes(0) }

func (s *spot[K]) Paint(*render.Canvas) {}

func (s *spot[K]) HitTest(widget.Point) widget.Widget { return nil }
