package harness

import (
	"errors"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stubbedev/lys/wm/internal/backend"
	"github.com/stubbedev/lys/wm/internal/compositor"
	"github.com/stubbedev/lys/wm/internal/testclient"
	"github.com/stubbedev/lys/wm/internal/testenv"
)

const chromiumEnv = "LYS_WM_CHROMIUM"

var (
	chromiumSlow  = backend.Mode{Size: image.Pt(1280, 720), RefreshMHz: 60000, Preferred: true}
	chromiumFast  = backend.Mode{Size: image.Pt(1280, 720), RefreshMHz: 144000, Preferred: true}
	chromiumFrame = regexp.MustCompile(`RAF (\d+)`)
)

type chromium struct {
	t *testing.T
	h *Harness
	c *Client
}

func startChromium(t *testing.T) *chromium {
	t.Helper()
	if os.Getenv(chromiumEnv) != "1" {
		t.Skipf("set %s=1, with chromium on PATH, to drive it through the scenarios of #397", chromiumEnv)
	}
	page, err := filepath.Abs(filepath.Join("testdata", "raf.html"))
	if err != nil {
		t.Fatal(err)
	}
	h := StartVulkan(t, func(*compositor.Config) {}, chromiumSlow, chromiumFast)
	c := h.Run("chromium", "--ozone-platform=wayland", "--no-sandbox", "--no-first-run", "--no-default-browser-check",
		"--enable-logging=stderr", "--user-data-dir="+t.TempDir(), "--enable-features=WaylandExternalBeginFrameSource",
		"file://"+page)
	ch := &chromium{t: t, h: h, c: c}
	ch.paints("its first frames", 30)
	return ch
}

func (ch *chromium) frames() int {
	m := chromiumFrame.FindAllStringSubmatch(ch.c.Output(), -1)
	if len(m) == 0 {
		return 0
	}
	n, err := strconv.Atoi(m[len(m)-1][1])
	if err != nil {
		ch.t.Fatal(err)
	}
	return n
}

func (ch *chromium) paints(after string, n int) {
	ch.t.Helper()
	from := ch.frames()
	deadline := time.Now().Add(testenv.HangLimit)
	for ch.frames() < from+n {
		if time.Now().After(deadline) {
			ch.t.Fatalf("Chromium drew %d of %d animation frames after %s within %v, so it stopped painting:\n%s", ch.frames()-from, n, after, testenv.HangLimit, ch.c.Output())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestChromiumKeepsPaintingAcrossOutputsPowerAndCaps(t *testing.T) {
	ch := startChromium(t)
	for range 15 {
		ch.h.dispatch("movetooutput HEADLESS-2")
		ch.paints("a move from 60 to 144 Hz", 20)
		ch.h.dispatch("movetooutput HEADLESS-1")
		ch.paints("a move from 144 to 60 Hz", 20)
	}
	for _, output := range []string{"HEADLESS-1", "HEADLESS-2"} {
		ch.h.dispatch("movetooutput " + output)
		ch.paints("moving to "+output, 60)
		ch.h.dispatch("dpms off " + output)
		ch.paints("powering "+output+" off, at the hidden rate", 3)
		ch.h.dispatch("dpms on " + output)
		ch.paints("powering "+output+" on", 60)
	}
	ch.h.dispatch("movetooutput HEADLESS-1")
	ch.h.dispatch("togglefloating")
	ch.h.dispatch("place right")
	ch.h.dispatch("resizeactive 400 0")
	ch.paints("spanning a 60 and a 144 Hz output", 120)
	powerSaver(ch.h, true)
	ch.paints("power saver capping the frame rate", 60)
	powerSaver(ch.h, false)
	ch.paints("power saver ending", 60)
	ch.h.dispatch("workspace 9")
	ch.paints("its workspace hidden, at the hidden rate", 3)
	ch.capturedWhileHidden(60)
	ch.h.dispatch("workspace previous")
	ch.paints("its workspace shown again", 60)
}

func (ch *chromium) capturedWhileHidden(n int) {
	ch.t.Helper()
	c := testclient.Connect(ch.t, ch.h.Socket())
	r := newCaptureRig(ch.t, ch.h, c)
	session, view := r.windowSession()
	from := ch.frames()
	deadline := time.Now().Add(testenv.HangLimit)
	for ch.frames() < from+n {
		if time.Now().After(deadline) {
			ch.t.Fatalf("Chromium drew %d of %d animation frames while its hidden window was captured, within %v:\n%s", ch.frames()-from, n, testenv.HangLimit, ch.c.Output())
		}
		f, frame, _ := grab(ch.t, c, session, view.size)
		c.DispatchUntil("a capture of the hidden window", func() bool { return f.ready || f.failed })
		if f.failed {
			ch.t.Fatal("a capture of the hidden Chromium window failed")
		}
		if err := errors.Join(frame.Destroy(), c.Display.Flush()); err != nil {
			ch.t.Fatal(err)
		}
	}
}
