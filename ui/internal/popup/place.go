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
// content, not the gutter, the Gap away from the anchor.
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
	g := intconv.Clamp[int32](cfg.Gutter - cfg.Gap)
	p := placement{rect: cfg.AnchorRect}
	p.rect.X -= cfg.ParentMargin
	p.rect.Y -= cfg.ParentMargin
	switch cfg.Gravity {
	case GravityTop:
		p.anchor, p.gravity, p.adjust = xdg.PositionerAnchorTop, xdg.PositionerGravityTop, slideX|flipY|resizeY
		p.offsetY = g
	case GravityRight:
		p.anchor, p.gravity, p.adjust = xdg.PositionerAnchorRight, xdg.PositionerGravityRight, slideY|flipX|resizeX
		p.offsetX = -g
	case GravityLeft:
		p.anchor, p.gravity, p.adjust = xdg.PositionerAnchorLeft, xdg.PositionerGravityLeft, slideY|flipX|resizeX
		p.offsetX = g
	default:
		p.anchor, p.gravity, p.adjust = xdg.PositionerAnchorBottom, xdg.PositionerGravityBottom, slideX|flipY|resizeY
		p.offsetY = -g
	}
	if cfg.Hold == HoldNone {
		return p
	}
	if _, vertical := opposite(cfg.Gravity); vertical {
		p.offsetX = intconv.Clamp[int32](heldStart(cfg.Hold, cfg.HoldAt, cfg.Width) - (p.rect.X + p.rect.W/2 - cfg.Width/2))
		p.adjust &^= slideX
	} else {
		p.offsetY = intconv.Clamp[int32](heldStart(cfg.Hold, cfg.HoldAt, cfg.Height) - (p.rect.Y + p.rect.H/2 - cfg.Height/2))
		p.adjust &^= slideY
	}
	return p
}

// Hold is the edge across a rect-anchored popup's opening axis that a
// resize keeps in place. Such a popup sits centred on its anchor, so
// without a hold it grows and shrinks on both sides at once. A held
// popup never slides along that axis: the compositor would move the
// held edge.
type Hold uint8

const (
	// HoldNone centres the popup on its anchor.
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
