package filebrowser

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/widget"
)

// qlEnter and qlExit are how long Quick Look's enter and exit play in
// these tests: long against the test's own pace, so every frame the
// test draws lands where its tick puts it.
const qlEnter, qlExit = 10 * time.Second, 10 * time.Second

// qlFrame is one frame of Quick Look's card: whether any of it paints,
// how far it is revealed, and where it lies.
type qlFrame struct {
	shows    bool
	progress float64
	card     render.Rect
}

// slowly has Quick Look enter and leave over qlEnter and qlExit.
func slowly(t *testing.T, hs *harness) {
	t.Helper()
	animated(t, hs)
	enter, exit := config.AnimationMS(qlEnter/time.Millisecond), config.AnimationMS(qlExit/time.Millisecond)
	hs.do(func() {
		a := config.DefaultsAnimations()
		a.Launcher.EnterDuration, a.Launcher.ExitDuration = &enter, &exit
		hs.c.d.Animations = func() config.AnimationsConfig { return a }
	})
}

// clocked hosts hs's window on a new clock the test ticks: a clock's
// ticks never go back, so each transition the test steps through from
// now runs on one of its own.
func clocked(t *testing.T, hs *harness) *anim.Clock {
	t.Helper()
	clock := new(anim.Clock)
	hs.do(func() { t.Cleanup(widget.Host(hs.cfg.Root, clock)) })
	return clock
}

// qlFrameAt draws the window's next frame as its loop does: what was
// launched since the last one starts on clock, clock ticks to at, and
// the window lays out.
func (hs *harness) qlFrameAt(clock *anim.Clock, at time.Time) qlFrame {
	var f qlFrame
	hs.do(func() {
		widget.AttachAnimations(clock)
		clock.Tick(at)
		lay(hs.cfg.Root, 1400, 1000)
		lay(hs.cfg.Root, 1400, 1000)
		q := hs.c.ui.quicklook
		f = qlFrame{shows: q.Visible() && q.rev.Progress() > 0, progress: q.rev.Progress(), card: q.rev.Bounds()}
	})
	return f
}

// The fixtures Quick Look enters on: a picture, source text and a
// document's pages.
var qlFixtures = []struct {
	name   string
	shows  preview.Renderer
	create func(t *testing.T, hs *harness) string
}{
	{"wide.png", preview.Image, func(t *testing.T, hs *harness) string {
		dir := filepath.Join(hs.root, "pics")
		f, err := os.Create(filepath.Join(dir, "wide.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.Close() }()
		if err := png.Encode(f, image.NewGray(image.Rect(0, 0, 3000, 1500))); err != nil {
			t.Fatal(err)
		}
		return dir
	}},
	{"main.go", preview.Text, func(t *testing.T, hs *harness) string {
		dir := filepath.Join(hs.root, "docs")
		writeFile(t, filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"))
		return dir
	}},
	{"reading.pdf", preview.Document, func(t *testing.T, hs *harness) string {
		readDocuments(t, hs)
		return docFixture(t, hs, "reading.pdf")
	}},
}

// Space shows nothing of Quick Look's card until its content has
// loaded; the card then enters at the size it rests at from its first
// visible frame, fading in over the frames the clock ticks, and leaves
// at that size too, fading out until it is hidden.
func TestQuicklookEntersAtItsContentsSize(t *testing.T) {
	for _, fx := range qlFixtures {
		t.Run(fx.name, func(t *testing.T) {
			hs := newHarness(t, "")
			t.Setenv("XDG_CACHE_HOME", t.TempDir())
			dir := fx.create(t, hs)
			hs.open(t, in(openFile, dir))
			hs.pick(t, fx.name)
			slowly(t, hs)
			clock := clocked(t, hs)
			w := watchLoads(t, hs)
			w.mu.Lock()
			w.hold = make(chan struct{})
			w.mu.Unlock()
			hs.key(xkb.KeySpace, 0, nil)
			waittest.Until(t, func() bool { return w.count() > 0 }, "Quick Look's load to start")
			for i := range 3 {
				if f := hs.qlFrameAt(clock, time.Now()); f.shows {
					t.Fatalf("frame %d with the content still loading shows the card %+v", i, f)
				}
			}
			w.mu.Lock()
			close(w.hold)
			w.hold = nil
			w.mu.Unlock()
			hs.waitPreview(t, fx.name)
			var shown preview.Renderer
			hs.do(func() { shown = hs.c.ui.ql.Renderer() })
			if shown != fx.shows {
				t.Fatalf("Quick Look shows %s as %v, want %v", fx.name, shown, fx.shows)
			}

			start := time.Now()
			enter := hs.playQuicklook(t, clock, start, qlEnter)
			rest := hs.qlFrameAt(clock, start.Add(2*qlEnter))
			if rest.progress != 1 || rest.card.W <= quicklookMinW || rest.card.H <= quicklookMinH {
				t.Fatalf("after its enter the card rests at %+v", rest)
			}
			checkFrames(t, "enter", enter, rest.card, func(a, b float64) bool { return b >= a })
			if enter[0].progress >= 1 {
				t.Errorf("the card's first visible frame is already whole: %+v", enter[0])
			}

			clock = clocked(t, hs)
			hs.key(xkb.KeyEscape, 0, nil)
			start = time.Now()
			exit := hs.playQuicklook(t, clock, start, qlExit)
			checkFrames(t, "exit", exit, rest.card, func(a, b float64) bool { return b <= a })
			if f := hs.qlFrameAt(clock, start.Add(2*qlExit)); f.shows || hs.qlState().visible {
				t.Errorf("after its exit the card still shows: %+v", f)
			}
			hs.hide()
		})
	}
}

// playQuicklook draws the frames of a transition of d started at start,
// a tenth of d apart, and is those that show the card.
func (hs *harness) playQuicklook(t *testing.T, clock *anim.Clock, start time.Time, d time.Duration) []qlFrame {
	t.Helper()
	var shown []qlFrame
	for at := start; at.Before(start.Add(d)); at = at.Add(d / 10) {
		if f := hs.qlFrameAt(clock, at); f.shows {
			shown = append(shown, f)
		}
	}
	return shown
}

// checkFrames checks a transition's frames: several of them part way,
// each at the card's resting rect, their progress in order.
func checkFrames(t *testing.T, name string, frames []qlFrame, rest render.Rect, ordered func(a, b float64) bool) {
	t.Helper()
	partway := 0
	for i, f := range frames {
		if f.card != rest {
			t.Errorf("%s frame %d lays the card out at %+v, want its resting %+v", name, i, f.card, rest)
		}
		if f.progress > 0 && f.progress < 1 {
			partway++
		}
		if i > 0 && !ordered(frames[i-1].progress, f.progress) {
			t.Errorf("%s frame %d goes back from %v to %v", name, i, frames[i-1].progress, f.progress)
		}
	}
	if partway < 5 {
		t.Errorf("the %s shows the card part way on %d frames, want it played: %+v", name, partway, frames)
	}
}

// holdLoads has hs's preview loads wait until the returned release
// runs.
func holdLoads(t *testing.T, hs *harness) (w *loadWatch, release func()) {
	t.Helper()
	w = watchLoads(t, hs)
	w.mu.Lock()
	w.hold = make(chan struct{})
	w.mu.Unlock()
	return w, func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.hold != nil {
			close(w.hold)
			w.hold = nil
		}
	}
}

// Content that takes longer than quicklookPatience to load does not
// keep Quick Look away: the card enters showing the file loading, and
// not before its patience runs out.
func TestQuicklookEntersLoadingPastItsPatience(t *testing.T) {
	hs := newHarness(t, "")
	dir := notes(t, hs)
	hs.open(t, in(openFile, dir))
	hs.pick(t, "note0.txt")
	slowly(t, hs)
	clock := clocked(t, hs)
	w, release := holdLoads(t, hs)
	t.Cleanup(release)
	start := time.Now()
	hs.key(xkb.KeySpace, 0, nil)
	waittest.Until(t, func() bool { return w.count() > 0 }, "Quick Look's load to start")
	if f := hs.qlFrameAt(clock, start.Add(quicklookPatience/2)); f.shows || !hs.qlState().open {
		t.Fatalf("within its patience Quick Look shows %+v, open %v; want it open and awaiting", f, hs.qlState().open)
	}
	hs.qlFrameAt(clock, time.Now().Add(quicklookPatience))
	f := hs.qlFrameAt(clock, time.Now().Add(quicklookPatience+qlEnter/2))
	var shown preview.Renderer
	hs.do(func() { shown = hs.c.ui.ql.Renderer() })
	if !f.shows || f.progress >= 1 || shown != preview.Loading {
		t.Errorf("past its patience Quick Look shows %+v as %v, want it entering on the file loading", f, shown)
	}
	release()
	hs.hide()
}

// Escape while Quick Look awaits its content closes it at once: the
// card never shows, the load gives up, and the key is Quick Look's,
// not the dialog's.
func TestQuicklookClosedWhileAwaitingNeverShows(t *testing.T) {
	hs := newHarness(t, "")
	dir := notes(t, hs)
	hs.open(t, in(openFile, dir))
	hs.pick(t, "note0.txt")
	slowly(t, hs)
	clock := clocked(t, hs)
	w, release := holdLoads(t, hs)
	t.Cleanup(release)
	hs.key(xkb.KeySpace, 0, nil)
	waittest.Until(t, func() bool { return w.count() > 0 }, "Quick Look's load to start")
	hs.key(xkb.KeyEscape, 0, nil)
	if s := hs.qlState(); s.visible || s.open || s.path != "" {
		t.Errorf("Escape while awaiting left Quick Look %+v", s)
	}
	w.mu.Lock()
	ctx := w.ctxs["note0.txt"]
	w.mu.Unlock()
	if ctx.Err() == nil {
		t.Error("the awaited load was not cancelled")
	}
	if len(hs.windows) == 0 || hs.windows[len(hs.windows)-1].closed {
		t.Error("Escape while Quick Look awaited closed the dialog")
	}
	if f := hs.qlFrameAt(clock, time.Now().Add(2*quicklookPatience)); f.shows {
		t.Errorf("after its patience the closed Quick Look shows %+v", f)
	}
	hs.hide()
}
