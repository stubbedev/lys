package app_test

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/service/lyswm/lyswmtest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/vinput"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/wltest"

	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/internal/framepace"
	"github.com/stubbedev/lys/ui/internal/headlesstest/inproc"
	"github.com/stubbedev/lys/ui/widget"
)

const (
	dragWarm    = 480
	dragMoves   = dragWarm + 960
	dragCols    = 800
	dragOriginX = 100
	dragOriginY = 100
	dragEvery   = 4 * time.Millisecond
	dragMark    = 8
	// dragMissShare bounds the frames that miss lys-wm's latch while the
	// pacer learns it: one in this many.
	dragMissShare = 20
)

var (
	dragBg  = render.RGB(0x10, 0x10, 0x18)
	dragInk = render.RGB(0xf0, 0xc0, 0x20)
)

type dragShown struct {
	move int
	at   time.Time
}

type dragRecord struct {
	mu      sync.Mutex
	offX    int
	offY    int
	aligned bool
	latest  int
	at      widget.Point
	shown   []dragShown
}

func dragTarget(move int) (x, y int) {
	return dragOriginX + move%dragCols, dragOriginY + move/dragCols
}

func (r *dragRecord) moved(x, y float64) (old, now render.Rect) {
	r.mu.Lock()
	defer r.mu.Unlock()
	old = dragMarkAt(r.at)
	r.at = widget.Point{X: int(x), Y: int(y)}
	if !r.aligned {
		r.offX, r.offY, r.aligned = dragOriginX-1-r.at.X, dragOriginY-r.at.Y, true
		r.latest = -1
		return old, dragMarkAt(r.at)
	}
	gx, gy := r.at.X+r.offX, r.at.Y+r.offY
	r.latest = (gy-dragOriginY)*dragCols + gx - dragOriginX
	return old, dragMarkAt(r.at)
}

func dragMarkAt(p widget.Point) render.Rect {
	return render.Rect{X: p.X - dragMark/2, Y: p.Y - dragMark/2, W: dragMark, H: dragMark}
}

func (r *dragRecord) painted() (render.Rect, int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return dragMarkAt(r.at), r.latest, r.aligned
}

func (r *dragRecord) presented(move int, at time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.shown = append(r.shown, dragShown{move: move, at: at})
}

func (r *dragRecord) isAligned() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.aligned
}

func (r *dragRecord) frames() []dragShown {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.shown)
}

type dragReport struct {
	frames, missed      int
	refresh             time.Duration
	p50, p95, worst, mu time.Duration
}

func reportDrag(sent []time.Time, shown []dragShown) (dragReport, error) {
	var during []dragShown
	for _, s := range shown {
		if s.move >= dragWarm {
			during = append(during, s)
		}
	}
	if len(during) < 3 {
		return dragReport{}, fmt.Errorf("%d frames presented during the drag", len(during))
	}
	gaps := make([]time.Duration, 0, len(during)-1)
	for i := 1; i < len(during); i++ {
		gaps = append(gaps, during[i].at.Sub(during[i-1].at))
	}
	sorted := slices.Clone(gaps)
	slices.Sort(sorted)
	rep := dragReport{frames: len(during), refresh: sorted[len(sorted)/2]}
	for _, g := range gaps {
		if skipped := int((g+rep.refresh/2)/rep.refresh) - 1; skipped > 0 {
			rep.missed += skipped
		}
	}
	lat := make([]time.Duration, 0, len(sent))
	next := 0
	for move, at := range sent[dragWarm:] {
		move += dragWarm
		for next < len(during) && during[next].move < move {
			next++
		}
		if next == len(during) {
			return dragReport{}, fmt.Errorf("move %d of %d never presented", move, len(sent))
		}
		lat = append(lat, during[next].at.Sub(at))
	}
	var sum time.Duration
	for _, l := range lat {
		sum += l
	}
	slices.Sort(lat)
	rep.p50, rep.p95, rep.worst, rep.mu = lat[len(lat)/2], lat[len(lat)*95/100], lat[len(lat)-1], sum/time.Duration(len(lat))
	return rep, nil
}

// A pointer dragged across a window on a headless lys-wm at four moves a
// refresh: every frame paces itself by lys-wm's presentation feedback and
// few of them miss lys-wm's latch while the pacer learns it. The input to
// present latency and the vblanks that showed no new frame are logged,
// never gated: they are wall-clock times.
func TestHeadlessDragIsPacedByLysWMPresentationFeedback(t *testing.T) {
	wltest.Require(t)
	lyswmtest.Headless(t)
	t.Setenv("LYS_UI_RENDERER", "cpu")
	rec := &dragRecord{latest: -1}
	var (
		application *app.Application
		win         *app.Window
	)
	inproc.Show(t, func(a *app.Application, _ *app.Output) error {
		application = a
		area := widget.NewDrawingArea(64, 64)
		area.OnDraw = func(cv *render.Canvas, r render.Rect) {
			cv.FillRect(r, dragBg)
			mark, move, aligned := rec.painted()
			if !aligned {
				return
			}
			cv.FillRect(mark, dragInk)
			if err := win.WhenPresented(func() { rec.presented(move, time.Now()) }); err != nil {
				t.Error(err)
			}
		}
		var err error
		win, err = a.NewWindow(app.WindowConfig{
			Title: "drag", Root: area, Background: dragBg,
			OnPointerMove: func(x, y float64) {
				old, now := rec.moved(x, y)
				area.InvalidateRect(old)
				area.InvalidateRect(now)
			},
		})
		return err
	})
	d, err := vinput.Dial("", vinput.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	w, h := d.OutputSize()
	move := func(x, y int) error {
		return d.MotionAbsolute(uint32(x), uint32(y), uint32(w), uint32(h))
	}
	wltest.Eventually(t, "the pointer resting on the window", func() error {
		if err := errors.Join(move(dragOriginX-1, dragOriginY), d.Roundtrip()); err != nil {
			return err
		}
		if !rec.isAligned() {
			return errors.New("the window heard no motion")
		}
		return nil
	})
	sent := make([]time.Time, dragMoves)
	tick := time.NewTicker(dragEvery)
	for i := range dragMoves {
		<-tick.C
		x, y := dragTarget(i)
		sent[i] = time.Now()
		if err := move(x, y); err != nil {
			tick.Stop()
			t.Fatal(err)
		}
	}
	tick.Stop()
	wltest.Eventually(t, "the last move on screen", func() error {
		shown := rec.frames()
		if len(shown) == 0 || shown[len(shown)-1].move < dragMoves-1 {
			return errors.New("not yet")
		}
		return nil
	})
	rep, err := reportDrag(sent, rec.frames())
	if err != nil {
		t.Fatal(err)
	}
	stats := make(chan framepace.Stats, 1)
	application.Invoke(func() { stats <- win.PaceStats() })
	paced := waittest.For(t, stats, "the window's pacing stats")
	t.Logf("%d frames at %v, %d vblanks missed; input to present p50 %v p95 %v mean %v worst %v; paced %+v",
		rep.frames, rep.refresh, rep.missed, rep.p50, rep.p95, rep.mu, rep.worst, paced)
	if paced.Paced < rep.frames {
		t.Errorf("%d of %d frames paced, want each paced by lys-wm's presentation feedback", paced.Paced, rep.frames)
	}
	if limit := rep.frames / dragMissShare; paced.Missed > limit {
		t.Errorf("%d frames committed in time missed lys-wm's latch (%+v), want at most %d of %d", paced.Missed, paced, limit, rep.frames)
	}
}
