package harness

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/udmabuf"

	"github.com/stubbedev/lys/wm/internal/anim"
	"github.com/stubbedev/lys/wm/internal/backend"
	"github.com/stubbedev/lys/wm/internal/backend/headless"
	"github.com/stubbedev/lys/wm/internal/compositor"
	"github.com/stubbedev/lys/wm/internal/testclient"
)

func idleFrames(h *Harness, n uint64) {
	h.t.Helper()
	var o *headless.Output
	var before uint64
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		o = b.Outputs()[0]
		before = o.Frames()
		return o.ScheduleFrame()
	})
	h.Eventually("frames on the inherited output", func(*headless.Backend, *compositor.Compositor) bool {
		if o.Frames() < before+n {
			_ = o.ScheduleFrame()
			return false
		}
		return true
	})
}

func takeover(h *Harness) (presents uint64, inherited bool) {
	h.t.Helper()
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		o := b.Outputs()[0]
		presents, inherited = o.Presents(), o.Inherited()
		return nil
	})
	return presents, inherited
}

func TestAnInheritedFrameStaysUntilAClientDraws(t *testing.T) {
	t.Parallel()
	h := StartInherited(t, func(*compositor.Config) {}, small)
	idleFrames(h, 5)
	if presents, inherited := takeover(h); presents != 0 || !inherited {
		t.Fatalf("lys-wm presented %d frames over the inherited one before a client drew (still inherited: %v)", presents, inherited)
	}
	c := testclient.Connect(t, h.Socket())
	w := c.Window("greeter", quadrants)
	c.DispatchUntil("the window to map", w.Mapped)
	h.Eventually("the frame that replaces the inherited one", func(b *headless.Backend, _ *compositor.Compositor) bool {
		return b.Outputs()[0].Takeover() != nil
	})
	var first [3]byte
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		first = pixel(b.Outputs()[0].Takeover(), 1, 1)
		return nil
	})
	if first != [3]byte{0xff, 0, 0} {
		t.Fatalf("the frame that replaced the inherited one shows %v at (1, 1), not the client's red", first)
	}
}

func TestAnInheritedFrameGivesWayWhenNoClientDraws(t *testing.T) {
	t.Parallel()
	r := &reports{}
	timeout := time.Hour
	h := StartInherited(t, func(cfg *compositor.Config) {
		cfg.InheritTimeout = timeout
		cfg.Report = r.report
	}, small)
	h.Freeze()
	h.Advance(timeout / 2)
	if presents, inherited := takeover(h); presents != 0 || !inherited {
		t.Fatalf("lys-wm presented %d frames over the inherited one halfway to the %v timeout (still inherited: %v)", presents, timeout, inherited)
	}
	h.Advance(timeout)
	h.Eventually("the frame that replaces the inherited one", func(b *headless.Backend, _ *compositor.Compositor) bool {
		return b.Outputs()[0].Takeover() != nil
	})
	var first [3]byte
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		first = pixel(b.Outputs()[0].Takeover(), 1, 1)
		return nil
	})
	if bg := compositor.Background; first != [3]byte{bg.R, bg.G, bg.B} {
		t.Fatalf("the frame drawn after the timeout shows %v, not the background", first)
	}
	if got := r.take(); len(got) != 1 || !strings.Contains(got[0].Error(), "no client drew on HEADLESS-1 within "+timeout.String()) {
		t.Fatalf("the timeout reported %q", got)
	}
}

func TestTheFirstFrameOverAnInheritedOneFadesFromIt(t *testing.T) {
	t.Run("imported on the GPU", func(t *testing.T) { testTheFirstFrameOverAnInheritedOneFadesFromIt(t, StartInheritedVulkan) })
	t.Run("copied by the CPU", func(t *testing.T) { testTheFirstFrameOverAnInheritedOneFadesFromIt(t, StartInherited) })
}

func testTheFirstFrameOverAnInheritedOneFadesFromIt(t *testing.T, start func(*testing.T, func(*compositor.Config), ...backend.Mode) *Harness) {
	probe, err := udmabuf.New(4096)
	if err != nil {
		t.Skipf("the headless backend makes the inherited frame a udmabuf: %v", err)
	}
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	d := 10 * time.Second
	off, on := false, true
	table, errs := anim.Resolve(true, map[anim.Event]anim.Setting{anim.Global: {Enabled: &off}, anim.FadeIn: {Enabled: &on, Shape: anim.Linear, Duration: &d}})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	r := &reports{}
	h := start(t, func(cfg *compositor.Config) {
		cfg.Animations = table
		cfg.Report = r.report
	}, small)
	h.Freeze()
	c := testclient.Connect(t, h.Socket())
	w := c.Window("greeter", quadrants)
	c.DispatchUntil("the window to map", w.Mapped)
	h.Eventually("the frame that replaces the inherited one", func(b *headless.Backend, _ *compositor.Compositor) bool {
		return b.Outputs()[0].Takeover() != nil
	})
	inherited := [3]byte{headless.InheritedColor.R, headless.InheritedColor.G, headless.InheritedColor.B}
	red := [3]byte{0xff, 0, 0}
	var first [3]byte
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		first = pixel(b.Outputs()[0].Takeover(), 1, 1)
		return nil
	})
	if !near(first, inherited) {
		t.Fatalf("the first frame over the inherited one shows %v at (1, 1), not the inherited %v it fades from (reported %q)", first, inherited, r.take())
	}
	at := h.Hold()
	last := 0.0
	for step := range 3 {
		h.AdvanceTo(at + time.Duration(step+1)*d/4)
		var got [3]byte
		var progress float64
		h.Eventually("a frame further into the fade", func(b *headless.Backend, _ *compositor.Compositor) bool {
			got = pixel(snapshot(b.Outputs()[0]), 1, 1)
			progress = float64(int(got[0])-int(inherited[0])) / float64(int(red[0])-int(inherited[0]))
			return progress > last
		})
		if progress >= 1 || !near(got, blend(red, inherited, progress)) {
			t.Fatalf("a frame of the fade shows %v at (1, 1), not a blend of the inherited %v and the client's %v", got, inherited, red)
		}
		last = progress
	}
	h.AdvanceTo(at + d + small.Interval())
	h.Eventually("the client's frame alone", func(b *headless.Backend, _ *compositor.Compositor) bool {
		return near(pixel(snapshot(b.Outputs()[0]), 1, 1), red)
	})
	if got := r.take(); len(got) != 0 {
		t.Fatalf("the fade reported %q", got)
	}
}

func TestThePointerStaysHiddenOverAnInheritedFrameUntilItMoves(t *testing.T) {
	t.Parallel()
	h := StartInherited(t, func(*compositor.Config) {}, small)
	r := inputOn(t, h)
	h.Eventually("the frame that replaces the inherited one", func(b *headless.Backend, _ *compositor.Compositor) bool {
		return b.Outputs()[0].Takeover() != nil
	})
	hidden := func() (hidden bool) {
		h.Do(func(_ *headless.Backend, c *compositor.Compositor) error {
			hidden = c.CursorHidden()
			return nil
		})
		return hidden
	}
	if !hidden() {
		t.Fatal("the pointer, which the inherited frame does not show, appeared with the first frame over it")
	}
	r.moveTo(40, 50)
	h.Eventually("the pointer shown once it moved", func(_ *headless.Backend, c *compositor.Compositor) bool {
		return !c.CursorHidden()
	})
}

func TestThePointerShowsFromTheStartWithoutAnInheritedFrame(t *testing.T) {
	t.Parallel()
	h := StartUntraced(t, small)
	inputOn(t, h)
	h.Do(func(_ *headless.Backend, c *compositor.Compositor) error {
		if c.CursorHidden() {
			return errors.New("the pointer starts hidden on an output that inherited nothing")
		}
		return nil
	})
}
