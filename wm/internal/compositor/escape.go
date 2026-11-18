package compositor

import (
	"errors"
	"slices"

	sle "github.com/stubbedev/lys/wm/internal/protocol/server/lysescape"
	sw "github.com/stubbedev/lys/wm/internal/protocol/server/wayland"
	"github.com/stubbedev/lys/wm/internal/server"
	"github.com/stubbedev/lys/wm/internal/surface"
	"github.com/stubbedev/lys/wm/internal/wire"
)

const escapeVersion = 2

type escapeArm struct {
	res    *sle.LysEscapeArmV1
	client *server.Client
}

func (*escapeArm) Destroy(*sle.LysEscapeArmV1) error { return nil }

func (c *Compositor) addEscape(d *server.Display) error {
	_, err := d.AddGlobal(sle.LysEscapeManagerV1Interface, escapeVersion, func(cl *server.Client, id wire.ObjectID, version uint32) error {
		_, err := sle.NewLysEscapeManagerV1(cl, id, version, escapeManager{c, cl})
		return err
	})
	return err
}

type escapeManager struct {
	c  *Compositor
	cl *server.Client
}

func (escapeManager) Destroy(*sle.LysEscapeManagerV1) error { return nil }

func (m escapeManager) Arm(_ *sle.LysEscapeManagerV1, res *sle.LysEscapeArmV1) (sle.LysEscapeArmV1Handler, error) {
	a := &escapeArm{res: res, client: m.cl}
	m.c.escapeArms = append(m.c.escapeArms, a)
	server.OnDestroy(res, func() error {
		m.c.dropEscapeArm(a)
		return nil
	})
	return a, nil
}

type pressWatch struct {
	res     *sle.LysPressWatchV1
	ignored []*surface.Surface
}

func (*pressWatch) Destroy(*sle.LysPressWatchV1) error { return nil }

func (w *pressWatch) Ignore(_ *sle.LysPressWatchV1, wl *sw.WlSurface) error {
	s, err := surface.From(wl)
	if err != nil || slices.Contains(w.ignored, s) {
		return err
	}
	w.ignored = append(w.ignored, s)
	server.OnDestroy(wl, func() error {
		w.ignored = slices.DeleteFunc(w.ignored, func(o *surface.Surface) bool { return o == s })
		return nil
	})
	return nil
}

func (w *pressWatch) hears(s *surface.Surface) bool {
	return s == nil || !slices.ContainsFunc(w.ignored, s.Within)
}

func (m escapeManager) WatchPresses(_ *sle.LysEscapeManagerV1, res *sle.LysPressWatchV1) (sle.LysPressWatchV1Handler, error) {
	w := &pressWatch{res: res}
	m.c.pressWatches = append(m.c.pressWatches, w)
	server.OnDestroy(res, func() error {
		m.c.pressWatches = slices.DeleteFunc(m.c.pressWatches, func(o *pressWatch) bool { return o == w })
		return nil
	})
	return w, nil
}

func (c *Compositor) pressedOn(s *surface.Surface) error {
	var errs []error
	for _, w := range c.pressWatches {
		if w.hears(s) {
			errs = append(errs, w.res.SendPressed())
		}
	}
	return errors.Join(errs...)
}

func (c *Compositor) dropEscapeArm(a *escapeArm) {
	c.escapeArms = slices.DeleteFunc(c.escapeArms, func(o *escapeArm) bool { return o == a })
}

func (c *Compositor) escapeTaker() *escapeArm {
	if len(c.escapeArms) == 0 || c.keyboardClaimed() {
		return nil
	}
	a := c.escapeArms[len(c.escapeArms)-1]
	if f := c.seat.Focus(); f != nil && f.Client() == a.client {
		return nil
	}
	return a
}

func (c *Compositor) keyboardClaimed() bool {
	return c.shell.Locked() || len(c.shell.Grab()) > 0 || c.exclusiveLayer() != nil ||
		c.seat.KeyboardGrabbed() || c.drag != nil || c.inhibiting != nil
}

func (c *Compositor) fireEscape(a *escapeArm) error {
	c.dropEscapeArm(a)
	return a.res.SendPressed()
}

func (c *Compositor) takeEscape(code uint32, pressed bool) (bool, error) {
	if !pressed {
		return false, nil
	}
	if slices.Contains(c.held, code) {
		return true, nil
	}
	a := c.escapeTaker()
	if a == nil || !c.seat.BareEscape(code) {
		return false, nil
	}
	c.held = append(c.held, code)
	return true, c.fireEscape(a)
}

func (v *virtualKeyboard) takeEscape(code uint32, pressed bool) (bool, error) {
	i := slices.Index(v.taken, code)
	if !pressed {
		if i < 0 {
			return false, nil
		}
		v.taken = slices.Delete(v.taken, i, i+1)
		if h := v.c.repeat.held; h != nil && h.key == code {
			return true, v.c.repeat.stop()
		}
		return true, nil
	}
	if i >= 0 {
		return true, nil
	}
	if v.c.overviewHolds() && v.c.seat.Focus() == nil {
		if sym, ok := v.c.seat.VirtualBareKeysym(v.kb, code); ok {
			taken, err := v.c.overviewSym(sym, code)
			if taken {
				v.taken = append(v.taken, code)
				return true, err
			}
		}
	}
	a := v.c.escapeTaker()
	if a == nil || !v.c.seat.VirtualBareEscape(v.kb, code) {
		return false, nil
	}
	v.taken = append(v.taken, code)
	return true, v.c.fireEscape(a)
}
