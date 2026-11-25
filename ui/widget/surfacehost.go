package widget

import (
	"fmt"

	"github.com/stubbedev/lys/lib/render"
)

// SurfaceHost is what a widget tree's surface does for its widgets:
// the application attaches one to every surface's root
// (SetSurfaceHost), and the widgets reach it through OpenMenu,
// OpenPopover and GrabFocus. The host owns the surface plumbing and
// reports its own failures.
type SurfaceHost interface {
	// OpenMenu opens items as a popup menu beside anchor, whose bounds
	// are in the tree's coordinates, painted with face at sizePx.
	OpenMenu(anchor Boundser, face render.Font, sizePx float64, items []MenuItem, opts MenuOptions)
	// OpenPopover opens content in a popover beside anchor, and returns
	// what closes it.
	OpenPopover(anchor Boundser, content Widget, opts PopoverOptions) (close func())
	// Focus moves the surface's keyboard focus to w, failing as
	// Router.SetFocus does.
	Focus(w Widget) error
}

// SetSurfaceHost makes h serve the tree under root. A root from
// outside this package carries no host, like SetStyleRoot's; asking
// its tree for a popup or a focus move then panics.
func SetSurfaceHost(root Widget, h SurfaceHost) {
	if n := nodeOf(root); n != nil {
		n.surfaceHost = h
	}
}

// hostOf is the host of w's tree; a tree no surface hosts is a caller
// bug, named by what asked.
func hostOf(w Widget, what string) SurfaceHost {
	for p := w; p != nil; p = parentOf(p) {
		if n := nodeOf(p); n != nil && n.surfaceHost != nil {
			return n.surfaceHost
		}
	}
	panic(fmt.Sprintf("widget: %s from %T: no surface hosts its tree", what, w))
}

// MenuOptions shape a widget-opened menu.
type MenuOptions struct {
	// OnClosed, when set, runs once the menu goes away.
	OnClosed func()
	// Gap is the distance between the anchor and the root menu.
	Gap int
}

// OpenMenu opens items as a popup menu beside anchor through the host
// of from's tree.
func OpenMenu(from Widget, anchor Boundser, face render.Font, sizePx float64, items []MenuItem, opts MenuOptions) {
	hostOf(from, "OpenMenu").OpenMenu(anchor, face, sizePx, items, opts)
}

// PopoverOptions shape a widget-opened popover.
type PopoverOptions struct {
	// Focus takes the keyboard focus as the popover opens.
	Focus Widget
	// NoGrab leaves the keyboard and pointer with the window: a hover
	// card shows beside what the user is pointing at instead of taking
	// the input, and closes only when told.
	NoGrab bool
	// OnClosed runs once the popover is gone, however it closed.
	OnClosed func()
	// Gap is the distance between the anchor and the popover.
	Gap int
	// Side is the side of the anchor the popover opens on, below
	// unless set.
	Side PopoverSide
	// Align is where the popover sits along the anchor across Side,
	// centred unless set.
	Align PopoverAlign
	// MinWidth is the least width of the popover's card, its border
	// included: a list as wide as the face that opened it.
	MinWidth int
}

// PopoverAlign is where a popover sits along its anchor across the
// side it opens on; the compositor slides it where it would leave the
// screen.
type PopoverAlign uint8

// The popover alignments.
const (
	// PopoverCentered centres the popover on its anchor, GtkPopover's
	// placement.
	PopoverCentered PopoverAlign = iota
	// PopoverStart lines the card's start edge up with the anchor's:
	// the left edge of one opening above or below, the top edge of one
	// opening beside.
	PopoverStart
	// PopoverEnd lines the card's end edge up with the anchor's: the
	// right edge, or the bottom one.
	PopoverEnd
	// PopoverHang hangs the card from its anchor as a dropdown's list
	// hangs under its face: PopoverStart for an anchor in the start half
	// of its surface, PopoverEnd for one in the end half, so a card
	// wider than its anchor grows toward the surface's middle, never
	// past the edge of the content the anchor ends.
	PopoverHang
)

// PopoverSide is the side of its anchor a popover opens on; the
// compositor flips it to the other side where there is no room.
type PopoverSide uint8

// The popover sides.
const (
	PopoverBelow PopoverSide = iota
	PopoverAbove
	PopoverRight
	PopoverLeft
)

// PopoverGap is the distance a popover or menu keeps from the trigger
// that opened it.
const PopoverGap = 4

// OpenPopover opens content in a popover beside anchor through the
// host of from's tree, and returns what closes it. The popover's own
// node is a box around content, at least opts.MinWidth wide, so
// content keeps its element and style.
func OpenPopover(from Widget, anchor Boundser, content Widget, opts PopoverOptions) (close func()) {
	card := NewBox(Column, 0, 0).Append(content, true)
	card.SetSizeRequest(opts.MinWidth, 0)
	if opts.Align == PopoverHang {
		opts.Align = hangAlign(from, anchor.Bounds(), opts.Side)
	}
	return hostOf(from, "OpenPopover").OpenPopover(anchor, card, opts)
}

// hangAlign is the edge a PopoverHang card opening on side lines up
// with anchor's: the start edge when anchor's middle lies in the start
// half of the root of from's tree across side, the end edge otherwise.
func hangAlign(from Widget, anchor render.Rect, side PopoverSide) PopoverAlign {
	root := from
	for p := parentOf(root); p != nil; p = parentOf(p) {
		root = p
	}
	b, ok := root.(Boundser)
	if !ok {
		return PopoverStart
	}
	surface := b.Bounds()
	at, span, extent := anchor.X-surface.X, anchor.W, surface.W
	if side == PopoverLeft || side == PopoverRight {
		at, span, extent = anchor.Y-surface.Y, anchor.H, surface.H
	}
	if 2*at+span > extent {
		return PopoverEnd
	}
	return PopoverStart
}

// GrabFocus moves the keyboard focus of w's surface to w (GTK's
// gtk_widget_grab_focus): a form focusing its first invalid field. A
// widget that cannot take focus fails with ErrNotFocusable and leaves
// it where it was.
func GrabFocus(w Widget) error { return hostOf(w, "GrabFocus").Focus(w) }
