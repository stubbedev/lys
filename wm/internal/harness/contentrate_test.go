package harness

import (
	"slices"
	"testing"
	"time"

	cct "github.com/stubbedev/lys/wm/internal/protocol/client/committiming"
	ccty "github.com/stubbedev/lys/wm/internal/protocol/client/contenttype"
	"github.com/stubbedev/lys/wm/internal/testclient"
)

type clip struct {
	start    time.Duration
	interval time.Duration
	frame    int
}

func (cl *clip) timestamped(timer *cct.WpCommitTimerV1) func() error {
	return func() error {
		at := cl.start + time.Duration(cl.frame)*cl.interval
		cl.frame++
		return timer.SetTimestamp(timestamp(at))
	}
}

func uncapped(t *testing.T, draw func() []*feedbackEvents) {
	t.Helper()
	for range uncappedTries {
		if slices.Min(refreshGaps(t, draw())) == 1 {
			return
		}
	}
	t.Fatal("a client drawing every frame never got the next refresh")
}

func TestAVideoAloneOnAnOutputPacesAtItsContentRate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		content   ccty.WpContentTypeV1Type
		hz        int
		refreshes uint64
	}{
		{"a 24 Hz video", ccty.WpContentTypeV1TypeVideo, 24, 5},
		{"a 30 Hz video", ccty.WpContentTypeV1TypeVideo, 30, 4},
		{"a 25 Hz video, which 120 Hz has no whole multiple of", ccty.WpContentTypeV1TypeVideo, 25, 0},
		{"a 24 Hz photo", ccty.WpContentTypeV1TypePhoto, 24, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := Start(t, fast)
			pc := pace(t, h)
			w := pc.c.Window("video", quadrants)
			pc.c.DispatchUntil("the window to map", w.Mapped)
			h.dispatch("fullscreen")
			pc.c.DispatchUntil("the fullscreen configure", func() bool { return w.Logical == fast.Size })
			quiesce(t, h, pc.c, w)
			ct, err := contentTypeManager(t, pc.c).GetSurfaceContentType(w.Surface)
			if err != nil {
				t.Fatal(err)
			}
			timer, err := pc.timing.GetTimer(w.Surface)
			if err != nil {
				t.Fatal(err)
			}
			cl := &clip{interval: time.Second / time.Duration(tc.hz), start: h.Now() / 2}
			typed := func() error { return ct.SetContentType(tc.content) }
			const frames = 24
			shown := drawContinuously(t, pc, w, frames, typed, cl.timestamped(timer))
			if tc.refreshes == 0 {
				uncapped(t, func() []*feedbackEvents { return drawContinuously(t, pc, w, 6, cl.timestamped(timer)) })
				return
			}
			exact := 0
			for i, gap := range refreshGaps(t, shown[2:]) {
				if gap < tc.refreshes {
					t.Fatalf("frame %d of %s alone on a 120 Hz output followed the one before after %d refreshes, want at least %d", i+5, tc.name, gap, tc.refreshes)
				}
				if gap == tc.refreshes {
					exact++
				}
			}
			if exact*2 < frames-4 {
				t.Fatalf("%d of %d frames came %d refreshes after the one before: %s should pace at %d Hz", exact, frames-4, tc.refreshes, tc.name, tc.hz)
			}

			h.dispatch("fullscreen")
			beside := pc.c.Window("beside", testclient.Fill(0, 0xff, 0))
			pc.c.DispatchUntil("a second window to map", beside.Mapped)
			uncapped(t, func() []*feedbackEvents { return drawContinuously(t, pc, w, 6, cl.timestamped(timer)) })
		})
	}
}

func TestAVideoWithoutADeclaredRatePacesAtTheOutputsRate(t *testing.T) {
	t.Parallel()
	h := Start(t, fast)
	pc := pace(t, h)
	w := pc.c.Window("video", quadrants)
	pc.c.DispatchUntil("the window to map", w.Mapped)
	h.dispatch("fullscreen")
	pc.c.DispatchUntil("the fullscreen configure", func() bool { return w.Logical == fast.Size })
	quiesce(t, h, pc.c, w)
	ct, err := contentTypeManager(t, pc.c).GetSurfaceContentType(w.Surface)
	if err != nil {
		t.Fatal(err)
	}
	timer, err := pc.timing.GetTimer(w.Surface)
	if err != nil {
		t.Fatal(err)
	}
	cl := &clip{interval: time.Second / 24, start: h.Now() / 2}
	video := func() error { return ct.SetContentType(ccty.WpContentTypeV1TypeVideo) }
	drawContinuously(t, pc, w, 6, video, cl.timestamped(timer))
	uncapped(t, func() []*feedbackEvents { return drawContinuously(t, pc, w, 6) })
}
