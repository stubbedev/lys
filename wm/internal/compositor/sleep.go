package compositor

import (
	"errors"
	"time"

	"github.com/stubbedev/lys/wm/internal/loop"
	"github.com/stubbedev/lys/wm/internal/surface"
	"github.com/stubbedev/lys/wm/internal/wm"
)

const DefaultSleepGrace = 2 * time.Second

type sleeper struct {
	timer *loop.Timer
	grace time.Duration
	armed bool
	due   time.Duration
}

type rest struct {
	unseenSince time.Duration
	unseen      bool
	asleep      bool
}

func (c *Compositor) seen(w *window) bool {
	if c.shared(w) || c.treeDrawn(w.top.Surface()) {
		return true
	}
	for _, pp := range c.popups(w) {
		if c.treeDrawn(pp.popup.Surface()) {
			return true
		}
	}
	return false
}

func (c *Compositor) treeDrawn(root *surface.Surface) bool {
	for member := range root.Stack() {
		if member == root {
			if c.drawnAnywhere(member) {
				return true
			}
			continue
		}
		if c.treeDrawn(member) {
			return true
		}
	}
	return false
}

func (c *Compositor) asleep(s *surface.Surface) bool {
	w := c.windowWithSurface(s.Root())
	return w != nil && w.rest.asleep
}

func (c *Compositor) noteSeen() error {
	now := c.clock()
	var errs error
	for _, w := range c.windows {
		errs = errors.Join(errs, c.noteWindowSeen(w, now))
	}
	return errors.Join(errs, c.armSleep(now))
}

func (c *Compositor) noteWindowSeen(w *window, now time.Duration) error {
	if !c.seen(w) {
		if !w.rest.unseen {
			w.rest.unseen, w.rest.unseenSince = true, now
		}
		return nil
	}
	w.rest.unseen = false
	if !w.rest.asleep {
		return nil
	}
	goal := w.wanted
	goal.suspended = false
	return errors.Join(c.rouse(w), c.settle(w, goal))
}

func (c *Compositor) suspendedFor(w *window, p wm.Placement) (bool, error) {
	if !w.rest.asleep {
		return false, nil
	}
	if o := c.outputAtRect(p.Rect); !p.Visible || p.Minimized || o == nil || !c.drawing(o) {
		return true, nil
	}
	return false, c.rouse(w)
}

func (c *Compositor) rouse(w *window) error {
	w.rest.asleep, w.rest.unseen = false, false
	var errs error
	c.eachSurfaceOf(w, func(s *surface.Surface) {
		errs = errors.Join(errs, c.throttleUndrawn(s))
	})
	if c.fifo.Hidden() {
		errs = errors.Join(errs, c.armHidden())
	}
	c.stateChanged()
	return errs
}

func (c *Compositor) fallAsleep(w *window) error {
	w.rest.asleep = true
	c.eachSurfaceOf(w, func(s *surface.Surface) {
		delete(c.hidden.waiting, s)
	})
	goal := w.wanted
	goal.suspended = true
	c.stateChanged()
	return c.settle(w, goal)
}

func (c *Compositor) eachSurfaceOf(w *window, f func(*surface.Surface)) {
	var walk func(root *surface.Surface)
	walk = func(root *surface.Surface) {
		for member := range root.Stack() {
			if member == root {
				f(member)
				continue
			}
			walk(member)
		}
	}
	walk(w.top.Surface())
}

func (c *Compositor) armSleep(now time.Duration) error {
	due, found := time.Duration(0), false
	for _, w := range c.windows {
		if !w.rest.unseen || w.rest.asleep {
			continue
		}
		if at := w.rest.unseenSince + c.sleep.grace; !found || at < due {
			due, found = at, true
		}
	}
	switch {
	case !found && c.sleep.armed:
		c.sleep.armed = false
		return c.sleep.timer.Disarm()
	case !found || c.sleep.armed && c.sleep.due == due:
		return nil
	}
	c.sleep.armed, c.sleep.due = true, due
	return c.sleep.timer.Arm(max(due-now, 0))
}

func (c *Compositor) sleepDue() error {
	c.sleep.armed = false
	now := c.clock()
	var errs error
	for _, w := range c.windows {
		errs = errors.Join(errs, c.noteWindowSeen(w, now))
		if w.rest.unseen && !w.rest.asleep && now-w.rest.unseenSince >= c.sleep.grace {
			errs = errors.Join(errs, c.fallAsleep(w))
		}
	}
	return errors.Join(errs, c.armSleep(now))
}
