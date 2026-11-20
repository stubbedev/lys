package harness

import (
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/wm/internal/backend/headless"
	"github.com/stubbedev/lys/wm/internal/compositor"
	cxdg "github.com/stubbedev/lys/wm/internal/protocol/client/xdgshell"
	"github.com/stubbedev/lys/wm/internal/testclient"
	"github.com/stubbedev/lys/wm/internal/wm"
)

const sleepGrace = time.Second

func sleepy(cfg *compositor.Config) {
	throttled(cfg)
	cfg.SleepGrace = sleepGrace
}

func suspended(w *testclient.Window) bool {
	return slices.Contains(w.States, cxdg.XdgToplevelStateSuspended)
}

func asleep(h *Harness, id wm.WindowID) bool {
	var a bool
	h.Do(func(_ *headless.Backend, c *compositor.Compositor) error {
		a = c.Asleep(id)
		return nil
	})
	return a
}

func fallsAsleep(t *testing.T, h *Harness, c *testclient.Client, w *testclient.Window, id wm.WindowID, what string) {
	t.Helper()
	h.Advance(sleepGrace / 2)
	c.Sync()
	if suspended(w) || asleep(h, id) {
		t.Fatalf("%s was suspended within its grace", what)
	}
	h.Advance(sleepGrace)
	c.DispatchUntil(what+" to be suspended", func() bool { return suspended(w) })
	if !asleep(h, id) {
		t.Fatalf("%s is suspended but lys-wm does not count it asleep", what)
	}
	frames, fired := w.Frames, hiddenFired(h)
	h.Advance(10 * sleepGrace)
	c.Sync()
	if got := w.Frames - frames; got != 0 {
		t.Fatalf("%s got %d frame callbacks asleep, want none", what, got)
	}
	if got := hiddenFired(h) - fired; got != 0 {
		t.Fatalf("lys-wm woke for %d hidden frames with only %s hidden, want none", got, what)
	}
}

func wakesBeforeItIsShown(t *testing.T, h *Harness, c *testclient.Client, w *testclient.Window, id wm.WindowID, what string) {
	t.Helper()
	frames := w.Frames
	c.DispatchUntil(what+" to get a frame callback again", func() bool { return w.Frames > frames })
	if slices.Contains(w.FramedStates, cxdg.XdgToplevelStateSuspended) {
		t.Fatalf("%s got the frame callback of a frame that shows it while still suspended", what)
	}
	if asleep(h, id) {
		t.Fatalf("%s is shown but lys-wm still counts it asleep", what)
	}
}

func TestAWindowOnAHiddenWorkspaceSleepsAndWakesBeforeItIsShown(t *testing.T) {
	t.Parallel()
	h := StartWith(t, sleepy, small)
	c := testclient.Connect(t, h.Socket())
	w := c.Window("away", fineChecker)
	c.DispatchUntil("the window to map", w.Mapped)
	h.Freeze()
	h.dispatch("workspace 2")
	h.Eventually("the second workspace shown", showingWorkspace(2))
	fallsAsleep(t, h, c, w, 1, "a window on a hidden workspace")
	h.dispatch("workspace 1")
	wakesBeforeItIsShown(t, h, c, w, 1, "a window whose workspace is shown again")
}

func TestAMinimizedWindowSleepsAndWakesBeforeItIsShown(t *testing.T) {
	t.Parallel()
	h := StartWith(t, sleepy, small)
	c := testclient.Connect(t, h.Socket())
	w := c.Window("minimized", fineChecker)
	c.DispatchUntil("the window to map", w.Mapped)
	h.Freeze()
	h.dispatch("minimize")
	fallsAsleep(t, h, c, w, 1, "a minimized window")
	h.dispatch("restore")
	wakesBeforeItIsShown(t, h, c, w, 1, "a restored window")
}

func TestAWindowUnderAnOpaqueWindowSleepsAndWakesWhenUncovered(t *testing.T) {
	t.Parallel()
	h := StartWith(t, sleepy, small)
	c := testclient.Connect(t, h.Socket())
	behind := c.Window("behind", fineChecker)
	c.DispatchUntil("the window behind to map", behind.Mapped)
	wall := c.Window("wall", testclient.Fill(0, 0, 0xff))
	c.DispatchUntil("the wall to map", wall.Mapped)
	wall.Stop()
	h.Freeze()
	fallsAsleep(t, h, c, behind, 1, "a window under an opaque window")
	if err := wall.Destroy(); err != nil {
		t.Fatal(err)
	}
	c.DispatchUntil("the uncovered window to be resumed", func() bool { return !suspended(behind) })
	frames := behind.Frames
	c.DispatchUntil("the uncovered window to get frame callbacks again", func() bool { return behind.Frames > frames })
}

func TestAShownWindowNeverSleeps(t *testing.T) {
	t.Parallel()
	h := StartWith(t, sleepy, small)
	c := testclient.Connect(t, h.Socket())
	w := c.Window("shown", fineChecker)
	c.DispatchUntil("the window to map", w.Mapped)
	h.Freeze()
	h.Advance(10 * sleepGrace)
	c.Sync()
	if suspended(w) || asleep(h, 1) {
		t.Fatal("a shown window was suspended")
	}
}

func TestACapturedWindowNeverSleeps(t *testing.T) {
	t.Parallel()
	h := StartWith(t, sleepy, small)
	c := testclient.Connect(t, h.Socket())
	w := c.Window("shared", quadrants)
	c.DispatchUntil("the window to map", w.Mapped)
	r := newCaptureRig(t, h, c)
	session, _ := r.windowSession()
	h.Freeze()
	h.dispatch("workspace 2")
	h.Eventually("the second workspace shown", showingWorkspace(2))
	h.Advance(10 * sleepGrace)
	c.Sync()
	if suspended(w) || asleep(h, 1) {
		t.Fatal("a hidden window being captured was suspended")
	}
	if err := session.Destroy(); err != nil {
		t.Fatal(err)
	}
	c.Sync()
	fallsAsleep(t, h, c, w, 1, "a hidden window no longer captured")
}

func TestAHiddenWindowStartsBeingCapturedWakes(t *testing.T) {
	t.Parallel()
	h := StartWith(t, sleepy, small)
	c := testclient.Connect(t, h.Socket())
	w := c.Window("shared", quadrants)
	c.DispatchUntil("the window to map", w.Mapped)
	r := newCaptureRig(t, h, c)
	h.Freeze()
	h.dispatch("workspace 2")
	h.Eventually("the second workspace shown", showingWorkspace(2))
	fallsAsleep(t, h, c, w, 1, "a hidden window")
	r.windowSession()
	c.DispatchUntil("the captured window to be resumed", func() bool { return !suspended(w) })
	if asleep(h, 1) {
		t.Fatal("a captured window is still asleep")
	}
}
