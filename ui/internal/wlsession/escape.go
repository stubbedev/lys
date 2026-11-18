// Escape arm: optional lys_escape_v1 support. An open dropdown that
// leaves the keyboard with another client's window claims the next
// Escape press through it, which no standard protocol can do. Feature-
// detected: without the global ArmEscape fails with
// ErrEscapeUnavailable.
package wlsession

import (
	"errors"
	"fmt"

	"github.com/stubbedev/lys/lib/debug"
	"github.com/stubbedev/lys/lib/third_party/wayland/wl"
	"github.com/stubbedev/lys/lib/wlr"
)

// ErrEscapeUnavailable reports that the compositor advertised no
// lys_escape_manager_v1.
var ErrEscapeUnavailable = errors.New("wlsession: no lys-escape protocol")

// escapeManagerAPI is the request side of lys_escape_manager_v1, seen
// through a narrow interface so tests can record the requests.
type escapeManagerAPI interface {
	Arm() (escapeArmAPI, error)
}

// ErrPressWatchUnavailable reports that the compositor's
// lys_escape_manager_v1, if any, predates the press watch.
var ErrPressWatchUnavailable = errors.New("wlsession: no lys-escape press watch")

// pressWatchManagerAPI is the press watch request of a version 2
// lys_escape_manager_v1.
type pressWatchManagerAPI interface {
	WatchPresses() (pressWatchAPI, error)
}

// pressWatchAPI is a lys_press_watch_v1. *wlr.LysPressWatchV1 satisfies
// it.
type pressWatchAPI interface {
	Destroy() error
	Ignore(surface *wl.Surface) error
	AddPressedHandler(h wlr.LysPressWatchV1PressedHandler)
}

// escapeArmAPI is a lys_escape_arm_v1. *wlr.LysEscapeArmV1 satisfies it.
type escapeArmAPI interface {
	Destroy() error
	AddPressedHandler(h wlr.LysEscapeArmV1PressedHandler)
}

// wireEscapeManager adapts the generated manager proxy to
// escapeManagerAPI.
type wireEscapeManager struct {
	mgr *wlr.LysEscapeManagerV1
}

// Arm implements escapeManagerAPI.
func (m wireEscapeManager) Arm() (escapeArmAPI, error) {
	a, err := m.mgr.Arm()
	if err != nil {
		return nil, err
	}
	return a, nil
}

// WatchPresses implements pressWatchManagerAPI.
func (m wireEscapeManager) WatchPresses() (pressWatchAPI, error) {
	w, err := m.mgr.WatchPresses()
	if err != nil {
		return nil, err
	}
	return w, nil
}

// pressWatchVersion is the lys_escape_manager_v1 version that brought
// the press watch.
const pressWatchVersion = 2

// bindEscapeManager binds lys_escape_manager_v1, and offers its press
// watch where the compositor's version has one.
func (s *Session) bindEscapeManager(ev wl.RegistryGlobalEvent) {
	mgr := wlr.NewLysEscapeManagerV1(s.registry.Context())
	if !s.bindOptional(ev, pressWatchVersion, mgr) {
		return
	}
	wire := wireEscapeManager{mgr: mgr}
	s.escapeMgr = wire
	if bindVersion(ev.Version, pressWatchVersion) >= pressWatchVersion {
		s.pressWatchMgr = wire
	}
}

// EscapeArm is one claim on the next Escape press: pressed runs when
// the compositor kept that press from the focused window, at most once.
type EscapeArm struct {
	req     escapeArmAPI
	pressed func()
}

// ArmEscape claims the next Escape press for pressed.
func (s *Session) ArmEscape(pressed func()) (*EscapeArm, error) {
	if s.escapeMgr == nil {
		return nil, ErrEscapeUnavailable
	}
	req, err := s.escapeMgr.Arm()
	if err != nil {
		return nil, fmt.Errorf("wlsession: arm escape: %w", err)
	}
	a := &EscapeArm{req: req, pressed: pressed}
	req.AddPressedHandler(a)
	return a, nil
}

// HandleLysEscapeArmV1Pressed implements
// wlr.LysEscapeArmV1PressedHandler.
func (a *EscapeArm) HandleLysEscapeArmV1Pressed(wlr.LysEscapeArmV1PressedEvent) {
	if a.req != nil {
		a.pressed()
	}
}

// Disarm gives the claim up; a second call does nothing.
func (a *EscapeArm) Disarm() {
	if a.req == nil {
		return
	}
	if err := a.req.Destroy(); err != nil {
		debug.Log("input", "escape disarm: %v", err)
	}
	a.req = nil
}

// PressWatch hears every press outside the surfaces it ignores without
// taking it: the press still lands where it was going.
type PressWatch struct {
	req     pressWatchAPI
	pressed func()
}

// WatchPresses runs pressed for every pointer press and touch outside
// the ignored surfaces and their subsurfaces, until Close.
func (s *Session) WatchPresses(pressed func(), ignore ...*wl.Surface) (*PressWatch, error) {
	if s.pressWatchMgr == nil {
		return nil, ErrPressWatchUnavailable
	}
	req, err := s.pressWatchMgr.WatchPresses()
	if err != nil {
		return nil, fmt.Errorf("wlsession: watch presses: %w", err)
	}
	w := &PressWatch{req: req, pressed: pressed}
	req.AddPressedHandler(w)
	for _, surf := range ignore {
		if err := w.Ignore(surf); err != nil {
			w.Close()
			return nil, err
		}
	}
	return w, nil
}

// Ignore stops presses on surf and its subsurfaces from counting.
func (w *PressWatch) Ignore(surf *wl.Surface) error {
	if w.req == nil {
		return errors.New("wlsession: ignore a surface on a closed press watch")
	}
	if err := w.req.Ignore(surf); err != nil {
		return fmt.Errorf("wlsession: press watch ignore: %w", err)
	}
	return nil
}

// HandleLysPressWatchV1Pressed implements
// wlr.LysPressWatchV1PressedHandler.
func (w *PressWatch) HandleLysPressWatchV1Pressed(wlr.LysPressWatchV1PressedEvent) {
	if w.req != nil {
		w.pressed()
	}
}

// Close ends the watch; a second call does nothing.
func (w *PressWatch) Close() {
	if w.req == nil {
		return
	}
	if err := w.req.Destroy(); err != nil {
		debug.Log("input", "press watch destroy: %v", err)
	}
	w.req = nil
}
