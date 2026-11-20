package compositor

import (
	"image"
	"slices"

	"github.com/stubbedev/lys/wm/internal/output"
	sxdg "github.com/stubbedev/lys/wm/internal/protocol/server/xdgshell"
	"github.com/stubbedev/lys/wm/internal/shell"
	"github.com/stubbedev/lys/wm/internal/surface"
	"github.com/stubbedev/lys/wm/internal/wm"
)

type want struct {
	rect      image.Rectangle
	bounds    image.Point
	layer     wm.PlacementLayer
	told      wm.PlacementLayer
	placed    bool
	activated bool
	minimized bool
	suspended bool
	scale     output.Scale
}

type toplevel interface {
	shell.PopupRoot
	Geometry() image.Rectangle
	AppID() string
	Title() string
	Tag() string
	Icon() *shell.Icon
	pid() int32
	sandboxed() bool
	parent() toplevel
	modal() bool
	prefersFloating() bool
	wantsMode() wm.WindowMode
	sizeLimits() (minimum, maximum image.Point)
	scaled() bool
	settled(w want) bool
	apply(w want) error
	resizes(w want) bool
	wakes(w want) bool
	mark() answerMark
	answers(u *surface.Update, m answerMark) bool
	synced() bool
	stale() bool
	shownSize() image.Point
	overflows(frame image.Point) bool
	close() error
	ping() error
	decorated() bool
}

func (t xdgToplevel) decorated() bool {
	return t.ServerDecorated()
}

type xdgToplevel struct {
	*shell.Toplevel
}

func (t xdgToplevel) pid() int32 {
	return t.Surface().Client().Creds().PID
}

func (t xdgToplevel) sandboxed() bool {
	_, sandboxed := t.Surface().Client().SecurityContext()
	return sandboxed
}

func (t xdgToplevel) parent() toplevel {
	if p := t.Parent(); p != nil {
		return xdgToplevel{p}
	}
	return nil
}

func (t xdgToplevel) modal() bool {
	return t.Modal()
}

func (t xdgToplevel) prefersFloating() bool {
	return t.Dialog()
}

func (t xdgToplevel) wantsMode() wm.WindowMode {
	return wantedMode(t.WantsFullscreen(), t.WantsMaximized())
}

func wantedMode(fullscreen, maximized bool) wm.WindowMode {
	switch {
	case fullscreen:
		return wm.ModeFullscreen
	case maximized:
		return wm.ModeMaximized
	}
	return wm.ModeNormal
}

func (t xdgToplevel) sizeLimits() (minimum, maximum image.Point) {
	return t.MinSize(), t.MaxSize()
}

func (t xdgToplevel) scaled() bool {
	return t.Requested().Bounds != (image.Point{})
}

type answerMark struct {
	serial uint32
	size   image.Point
}

type placedKind uint8

const (
	placedPlain placedKind = iota
	placedTiled
	placedMaximized
	placedFullscreen
	placedKinds
)

var placedStates = [placedKinds][]shell.ToplevelState{
	placedTiled: {
		sxdg.XdgToplevelStateTiledLeft, sxdg.XdgToplevelStateTiledRight, sxdg.XdgToplevelStateTiledTop, sxdg.XdgToplevelStateTiledBottom,
		sxdg.XdgToplevelStateConstrainedLeft, sxdg.XdgToplevelStateConstrainedRight, sxdg.XdgToplevelStateConstrainedTop, sxdg.XdgToplevelStateConstrainedBottom,
	},
	placedMaximized:  {sxdg.XdgToplevelStateMaximized},
	placedFullscreen: {sxdg.XdgToplevelStateFullscreen},
}

var stateSets = func() (sets [placedKinds][2][2][]shell.ToplevelState) {
	for kind, base := range placedStates {
		for activated := range 2 {
			for suspended := range 2 {
				set := slices.Clone(base)
				if activated == 1 {
					set = append(set, sxdg.XdgToplevelStateActivated)
				}
				if suspended == 1 {
					set = append(set, sxdg.XdgToplevelStateSuspended)
				}
				sets[kind][activated][suspended] = set
			}
		}
	}
	return sets
}()

func placedKindOf(w want) placedKind {
	if !w.placed {
		return placedPlain
	}
	switch w.told {
	case wm.LayerTiled:
		return placedTiled
	case wm.LayerMaximized:
		return placedMaximized
	case wm.LayerFullscreen:
		return placedFullscreen
	}
	return placedPlain
}

func bit(b bool) int {
	if b {
		return 1
	}
	return 0
}

func xdgStates(w want) []shell.ToplevelState {
	return stateSets[placedKindOf(w)][bit(w.activated)][bit(w.suspended)]
}

func xdgState(w want) shell.State {
	return shell.State{Size: w.rect.Size(), Bounds: w.bounds, States: xdgStates(w)}
}

func (t xdgToplevel) settled(w want) bool {
	return sameState(t.Requested(), xdgState(w))
}

func (t xdgToplevel) apply(w want) error {
	s := t.Surface()
	if w.scale != 0 {
		if err := s.SetPreferredScale(uint32(w.scale)); err != nil {
			return err
		}
	}
	_, err := t.Configure(xdgState(w))
	return err
}

func (t xdgToplevel) resizes(w want) bool {
	was, size := t.Requested(), w.rect.Size()
	flips := (w.placed && w.told == wm.LayerFullscreen) != was.Has(sxdg.XdgToplevelStateFullscreen) || (w.placed && w.told == wm.LayerMaximized) != was.Has(sxdg.XdgToplevelStateMaximized)
	return flips || (size != was.Size && size != t.Geometry().Size())
}

func (t xdgToplevel) wakes(w want) bool {
	return !w.suspended && t.Requested().Has(sxdg.XdgToplevelStateSuspended) && t.Understands(sxdg.XdgToplevelStateSuspended)
}

func (t xdgToplevel) mark() answerMark {
	return answerMark{serial: t.LatestSerial()}
}

func (t xdgToplevel) answers(u *surface.Update, m answerMark) bool {
	return t.Acked(m.serial) && u.Buffer() != nil
}

func (xdgToplevel) synced() bool {
	return true
}

func (t xdgToplevel) stale() bool {
	return t.Requested().Size != t.Current().Size
}

func (t xdgToplevel) shownSize() image.Point {
	return t.Geometry().Size()
}

func (t xdgToplevel) overflows(frame image.Point) bool {
	shown := t.Geometry().Size()
	return (shown.X > frame.X || shown.Y > frame.Y) && !t.stale()
}

func (t xdgToplevel) close() error {
	return t.SendClose()
}

func (t xdgToplevel) ping() error {
	return t.Ping()
}
