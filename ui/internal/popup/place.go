package popup

import (
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/third_party/wayland/xdg"
)

// placement is one xdg_positioner setup: the anchor rect, which point
// of it the popup hangs from, the direction it grows, how the
// compositor may adjust it against the output, and the offset that
// keeps the shadow gutter outside the content's alignment.
type placement struct {
	rect             render.Rect
	anchor, gravity  uint32
	adjust           uint32
	offsetX, offsetY int32
}

// placementFor maps a Config onto the positioner. A point (menus) is
// a 1x1 rect the popup grows down and right of, sliding on both axes
// and flipping up. A rect (popovers, tooltips) opens on the Gravity
// side centered on the anchor along the other axis, as GtkPopover
// places itself (a bottom popover's rect anchor south, surface anchor
// north), and may flip across the anchor on that axis and slide along
// the other, so a bar's dropdown that has no room below the screen
// edge opens above the bar instead of over it; with room on neither
// side it shrinks on that axis (GtkPopover's resize), its content
// giving way, rather than leave the output. The shadow gutter rides
// on every side, so only the opening axis needs an offset to put the
// content, not the gutter, the Gap away from the anchor. AlignStart
// hangs the popup from the anchor's start corner instead, growing away
// from it along the other axis, its gutter offset on that axis too, so
// the content's start edge, not the shadow's, lines up with the
// anchor's.
func placementFor(cfg Config) placement {
	const (
		slideX  = xdg.PositionerConstraintAdjustmentSlideX
		slideY  = xdg.PositionerConstraintAdjustmentSlideY
		flipX   = xdg.PositionerConstraintAdjustmentFlipX
		flipY   = xdg.PositionerConstraintAdjustmentFlipY
		resizeX = xdg.PositionerConstraintAdjustmentResizeX
		resizeY = xdg.PositionerConstraintAdjustmentResizeY
	)
	if cfg.AnchorRect.Empty() {
		return placement{
			rect:    render.Rect{X: cfg.X - cfg.ParentMargin, Y: cfg.Y - cfg.ParentMargin, W: 1, H: 1},
			anchor:  xdg.PositionerAnchorTopLeft,
			gravity: xdg.PositionerGravityBottomRight,
			adjust:  slideX | slideY | flipY,
		}
	}
	p := placement{rect: cfg.AnchorRect}
	p.rect.X -= cfg.ParentMargin
	p.rect.Y -= cfg.ParentMargin
	p.anchor, p.gravity = edges(cfg.Gravity, cfg.Align)
	vertical := false
	switch cfg.Gravity {
	case GravityTop, GravityBottom:
		p.adjust, vertical = slideX|flipY|resizeY, true
	default:
		p.adjust = slideY | flipX | resizeX
	}
	var rest int32
	p.rect, rest = gutterRect(p.rect, cfg.Gutter-cfg.Gap, vertical)
	switch cfg.Gravity {
	case GravityTop:
		p.offsetY = rest
	case GravityRight:
		p.offsetX = -rest
	case GravityLeft:
		p.offsetX = rest
	default:
		p.offsetY = -rest
	}
	if cfg.Align == AlignStart {
		cross := intconv.Clamp[int32](-cfg.Gutter)
		if vertical {
			p.offsetX = cross
		} else {
			p.offsetY = cross
		}
	}
	if cfg.Hold == HoldNone {
		return p
	}
	if vertical {
		p.offsetX = intconv.Clamp[int32](heldStart(cfg.Hold, cfg.HoldAt, cfg.Width) - crossStart(cfg.Align, p.rect.X, p.rect.W, cfg.Width))
		p.adjust &^= slideX
	} else {
		p.offsetY = intconv.Clamp[int32](heldStart(cfg.Hold, cfg.HoldAt, cfg.Height) - crossStart(cfg.Align, p.rect.Y, p.rect.H, cfg.Height))
		p.adjust &^= slideY
	}
	return p
}

// gutterRect is the anchor rect r a popup's content opens the Gap off
// once its shadow gutter rides outside it: r shrunk on the opening axis
// by g, the gutter less the Gap, on both sides (grown where the Gap is
// the larger), so a compositor flipping the popup across the anchor
// keeps the content the Gap away on the other side too, which an
// offset, which a flip leaves as it is, would not. An anchor too short
// to shrink by g keeps a 1px rect at its middle, and rest is the part
// of g left to the offset.
func gutterRect(r render.Rect, g int, vertical bool) (render.Rect, int32) {
	at, span := &r.X, &r.W
	if vertical {
		at, span = &r.Y, &r.H
	}
	shrink := min(g, (*span-1)/2)
	*at += shrink
	*span -= 2 * shrink
	return r, intconv.Clamp[int32](g - shrink)
}

// Align is where a rect-anchored popup sits along its anchor across
// the opening axis.
type Align uint8

const (
	// AlignCenter centres the popup on its anchor, as GtkPopover does.
	AlignCenter Align = iota
	// AlignStart lines the content's start edge up with the anchor's:
	// the left edge of a popup opening up or down, the top edge of one
	// opening sideways, as a combobox's list hangs under its face.
	AlignStart
)

// edges is the anchor point and the growth direction of a popup
// opening along gravity, aligned by align across it.
func edges(gravity Gravity, align Align) (anchor, grow uint32) {
	start := align == AlignStart
	switch {
	case gravity == GravityTop && start:
		return xdg.PositionerAnchorTopLeft, xdg.PositionerGravityTopRight
	case gravity == GravityTop:
		return xdg.PositionerAnchorTop, xdg.PositionerGravityTop
	case gravity == GravityRight && start:
		return xdg.PositionerAnchorTopRight, xdg.PositionerGravityBottomRight
	case gravity == GravityRight:
		return xdg.PositionerAnchorRight, xdg.PositionerGravityRight
	case gravity == GravityLeft && start:
		return xdg.PositionerAnchorTopLeft, xdg.PositionerGravityBottomLeft
	case gravity == GravityLeft:
		return xdg.PositionerAnchorLeft, xdg.PositionerGravityLeft
	case start:
		return xdg.PositionerAnchorBottomLeft, xdg.PositionerGravityBottomRight
	}
	return xdg.PositionerAnchorBottom, xdg.PositionerGravityBottom
}

// crossStart is where a popup extent long starts along the cross axis
// before any offset, hung from an anchor spanning at to at+span.
func crossStart(align Align, at, span, extent int) int {
	if align == AlignStart {
		return at
	}
	return at + span/2 - extent/2
}

// Hold is the edge across a rect-anchored popup's opening axis that a
// resize keeps in place. A centred popup grows and shrinks on both
// sides at once without a hold. A held
// popup never slides along that axis: the compositor would move the
// held edge.
type Hold uint8

const (
	// HoldNone places the popup by its Align.
	HoldNone Hold = iota
	// HoldStart keeps the left edge of a popup opening up or down, the
	// top edge of one opening sideways.
	HoldStart
	// HoldEnd keeps the right edge, or the bottom one.
	HoldEnd
)

// heldStart is where a popup extent long starts along the cross axis
// when hold keeps its edge at at.
func heldStart(hold Hold, at, extent int) int {
	if hold == HoldEnd {
		return at - extent
	}
	return at
}

// holdAt is the cross-axis coordinate of the edge hold keeps, for a
// popup placed at x, y and w by h that opens along gravity.
func holdAt(hold Hold, gravity Gravity, x, y, w, h int) int {
	start, extent := y, h
	if _, vertical := opposite(gravity); vertical {
		start, extent = x, w
	}
	if hold == HoldEnd {
		return start + extent
	}
	return start
}

// sideOf is the side of its anchor a popup placed at x, y and w by h
// sits on: the gravity's, or the opposite one after a flip. A point
// anchored popup keeps its gravity.
func sideOf(cfg Config, x, y, w, h int) Gravity {
	if cfg.AnchorRect.Empty() {
		return cfg.Gravity
	}
	r := cfg.AnchorRect
	if _, vertical := opposite(cfg.Gravity); vertical {
		if 2*(y+cfg.ParentMargin)+h < 2*r.Y+r.H {
			return GravityTop
		}
		return GravityBottom
	}
	if 2*(x+cfg.ParentMargin)+w < 2*r.X+r.W {
		return GravityLeft
	}
	return GravityRight
}

// place applies the placement to a positioner.
func place(pos *xdg.Positioner, cfg Config) error {
	p := placementFor(cfg)
	r := p.rect
	if err := pos.SetAnchorRect(intconv.Clamp[int32](r.X), intconv.Clamp[int32](r.Y), intconv.Clamp[int32](r.W), intconv.Clamp[int32](r.H)); err != nil {
		return err
	}
	if err := pos.SetAnchor(p.anchor); err != nil {
		return err
	}
	if err := pos.SetGravity(p.gravity); err != nil {
		return err
	}
	if err := pos.SetConstraintAdjustment(p.adjust); err != nil {
		return err
	}
	if p.offsetX != 0 || p.offsetY != 0 {
		return pos.SetOffset(p.offsetX, p.offsetY)
	}
	return nil
}
