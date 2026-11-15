package harness

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/wm/internal/backend/headless"
	"github.com/stubbedev/lys/wm/internal/compositor"
	cpt "github.com/stubbedev/lys/wm/internal/protocol/client/presentationtime"
	cw "github.com/stubbedev/lys/wm/internal/protocol/client/wayland"
	"github.com/stubbedev/lys/wm/internal/testclient"
)

func frameOwedWhenTheOutputGoes(t *testing.T, gone func(b *headless.Backend, c *compositor.Compositor) error) {
	t.Helper()
	slow := small
	slow.RefreshMHz = 1000
	h := StartWith(t, throttled, slow)
	c := testclient.Connect(t, h.Socket())
	w := c.Window("painting", quadrants)
	c.DispatchUntil("the window to map", w.Mapped)
	onScreen := showing(1, 1, [3]byte{0xff, 0, 0})
	c.DispatchUntil("the window on the output", func() bool {
		shown := false
		h.Do(func(b *headless.Backend, comp *compositor.Compositor) error {
			shown = onScreen(b, comp)
			return nil
		})
		return shown
	})
	w.Stop()
	var times frameTimes
	s := w.Surface
	if _, err := s.Frame(&times); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(s.Attach(c.Buffer(w.Size, 1, quadrants), 0, 0), s.DamageBuffer(0, 0, int32(w.Size.X), int32(w.Size.Y)), s.Commit()); err != nil {
		t.Fatal(err)
	}
	if err := c.Display.Roundtrip(); err != nil {
		t.Fatal(err)
	}
	if len(times) > 0 {
		t.Fatal("the frame callback was answered before the output went, so the test proves nothing; the output's refresh is too fast")
	}
	fired := hiddenFired(h)
	h.Do(gone)
	c.DispatchUntil("the frame callback owed when the output went", func() bool { return len(times) > 0 })
	if hiddenFired(h) == fired {
		t.Fatal("the frame callback was answered without a hidden frame, so something other than the hidden rate answered it")
	}
}

func TestAFrameCallbackOwedWhenTheOutputPowersOffIsAnsweredAtTheHiddenRate(t *testing.T) {
	t.Parallel()
	frameOwedWhenTheOutputGoes(t, func(_ *headless.Backend, c *compositor.Compositor) error {
		return c.SetPower(c.Outputs()[0], false)
	})
}

func TestAFrameCallbackOwedWhenTheOutputIsUnpluggedIsAnsweredAtTheHiddenRate(t *testing.T) {
	t.Parallel()
	frameOwedWhenTheOutputGoes(t, func(b *headless.Backend, _ *compositor.Compositor) error {
		return b.RemoveOutput(b.Outputs()[0])
	})
}

type orderedFrame struct{ log *testclient.Log }

func (f orderedFrame) Done(*cw.WlCallback, uint32) error { return f.log.Note("done") }

type orderedFeedback struct {
	log     *testclient.Log
	refresh time.Duration
}

func (*orderedFeedback) SyncOutput(*cpt.WpPresentationFeedback, *cw.WlOutput) error { return nil }

func (f *orderedFeedback) Presented(_ *cpt.WpPresentationFeedback, _, _, _, refresh, _, _ uint32, _ cpt.WpPresentationFeedbackKind) error {
	f.refresh = time.Duration(refresh)
	return f.log.Note("presented")
}

func (f *orderedFeedback) Discarded(*cpt.WpPresentationFeedback) error {
	return f.log.Note("discarded")
}

func TestAWindowMovedToASlowerOutputLearnsItsRefreshBeforeItsNextFrameCallback(t *testing.T) {
	t.Parallel()
	fast := small
	fast.RefreshMHz = 144000
	h := Start(t, fast, small)
	c := testclient.Connect(t, h.Socket())
	p, err := cpt.NewWpPresentation(c.Display, 2, &presentationEvents{})
	if err != nil {
		t.Fatal(err)
	}
	c.Bind(p)
	w := c.Window("moved", quadrants)
	c.DispatchUntil("the window to map", w.Mapped)
	w.Stop()
	commit := func(frame int) (testclient.Log, time.Duration) {
		t.Helper()
		var log testclient.Log
		fb := &orderedFeedback{log: &log}
		s := w.Surface
		_, errFrame := s.Frame(orderedFrame{&log})
		_, errFeedback := p.Feedback(s, fb)
		if err := errors.Join(errFrame, errFeedback, s.Attach(c.Buffer(w.Size, frame, quadrants), 0, 0), s.DamageBuffer(0, 0, int32(w.Size.X), int32(w.Size.Y)), s.Commit(), c.Display.Flush()); err != nil {
			t.Fatal(err)
		}
		c.DispatchUntil("the frame callback and the feedback", func() bool { return len(log) == 2 })
		return log, fb.refresh
	}
	order := testclient.Log{"presented", "done"}
	if log, refresh := commit(1); refresh != fast.Interval() || !slices.Equal(log, order) {
		t.Fatalf("on the %v output a commit got %v with refresh %v, want %v with refresh %v", fast.Interval(), log, refresh, order, fast.Interval())
	}
	h.dispatch("movetooutput HEADLESS-2")
	for frame := 2; ; frame++ {
		log, refresh := commit(frame)
		if !slices.Equal(log, order) {
			t.Fatalf("frame %d got %v: a client that draws on its frame callback times that frame by the refresh it knew before, and Chrome drops a begin frame whose time runs backwards from it, never to paint again", frame, log)
		}
		if refresh == small.Interval() {
			return
		}
		if frame > 64 {
			t.Fatalf("the window still reported refresh %v %d frames after it moved to the %v output", refresh, frame, small.Interval())
		}
	}
}
