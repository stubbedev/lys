package settings

import (
	"errors"
	"fmt"
	"image"
	"os"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/lib/gpu"
	"github.com/stubbedev/lys/lib/gputest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/testbuild"
	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/widget"
)

// drawMode is how a frameDriver turns a frame into pixels.
type drawMode int

const (
	// drawRaster paints on the CPU, as lys-ui's shared-memory path.
	drawRaster drawMode = iota
	// drawRecorded records a List and plays it onto a raster canvas:
	// the GPU path's recording and layer keeping, the CPU as player.
	drawRecorded
	// drawGPU records a List and renders it with Vulkan, as lys-ui's
	// GPU path, onto two offscreen targets in turn.
	drawGPU
)

func (m drawMode) String() string {
	return [...]string{"cpu", "recorded", "vulkan"}[m]
}

// frameDriver draws the settings window's frames headless the way a
// lys-ui window draws them: layout, the damage drain, each scroll move
// at a whole device step copied in place, and the owed regions painted
// over the window background. Time runs at 120 frames a second, and a
// timer the window arms fires once its frame comes.
type frameDriver struct {
	root   widget.Widget
	w, h   int
	num    int
	dw, dh int
	bg     render.Color
	mode   drawMode
	buf    []byte
	cv     *render.Canvas
	rec    *render.Canvas
	damage widget.DamageCollector
	clock  *anim.Clock
	now    time.Time
	timers []driverTimer

	dev   *gpu.Device
	tg    [2]*gpu.Target
	drawn [2]bool
	cur   int
}

type driverTimer struct {
	at time.Time
	fn func()
}

// frameStat is what one frame cost: the CPU time of its phases (paint
// covers recording and handing the List to the GPU), the device pixels
// it painted, the List ops it recorded, its heap allocations, and the
// scroll moves it copied of those it found.
type frameStat struct {
	layout, collect, paint time.Duration
	painted, ops           int
	allocs, bytes          uint64
	moves, copied          int
}

func newFrameDriver(mode drawMode, now time.Time) *frameDriver {
	return &frameDriver{mode: mode, now: now, clock: new(anim.Clock)}
}

// after is the window's timer: fn runs in the frame d from now.
func (d *frameDriver) after(dur time.Duration, fn func()) {
	d.timers = append(d.timers, driverTimer{at: d.now.Add(dur), fn: fn})
}

// start hosts root in a w x h window at device scale num/120.
func (d *frameDriver) start(t *testing.T, root widget.Widget, w, h, num int, bg render.Color) {
	t.Helper()
	d.root, d.w, d.h, d.num, d.bg = root, w, h, num, bg
	d.dw, d.dh = w*num/120, h*num/120
	stride := render.Stride(d.dw)
	d.buf = make([]byte, stride*d.dh)
	d.cv = render.NewScaled(d.buf, stride, d.dw, d.dh, num, 120)
	if d.mode != drawRaster {
		d.rec = render.NewRecorder(d.dw, d.dh, num, 120)
	}
	if d.mode == drawGPU {
		if _, err := gputest.SetupTest(t); err != nil {
			t.Fatalf("GPU for LYS_TEST_GPU=%s: %v", os.Getenv("LYS_TEST_GPU"), err)
		}
		dev, err := gpu.Open()
		if err != nil {
			t.Fatalf("open the GPU: %v", err)
		}
		t.Cleanup(dev.Close)
		d.dev = dev
		for i := range d.tg {
			if d.tg[i], err = dev.NewOffscreen(d.dw, d.dh); err != nil {
				t.Fatalf("offscreen target %dx%d: %v", d.dw, d.dh, err)
			}
			t.Cleanup(d.tg[i].Close)
		}
	}
	t.Cleanup(widget.Host(root, d.clock))
	widget.SetDeviceScale(root, num, 120)
}

func (d *frameDriver) tick() {
	d.now = d.now.Add(time.Second / 120)
	d.clock.Tick(d.now)
	due := d.timers
	d.timers = nil
	for _, tm := range due {
		if tm.at.After(d.now) {
			d.timers = append(d.timers, tm)
			continue
		}
		tm.fn()
	}
}

func (d *frameDriver) frame(t *testing.T) frameStat {
	t.Helper()
	d.tick()
	var st frameStat
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	r := render.Rect{W: d.w, H: d.h}
	d.root.Arrange(r)
	for range 4 {
		if !widget.LayoutPending(d.root) {
			break
		}
		d.root.Arrange(r)
	}
	laid := time.Now()
	rects, moves, _ := d.damage.CollectFrame(d.root)
	collected := time.Now()
	full := render.Rect{W: d.dw, H: d.dh}
	carries := d.mode == drawGPU && d.drawn[1-d.cur]
	var carry []gpu.Carry
	if carries {
		carry = append(carry, gpu.Carry{From: d.tg[1-d.cur], Rects: []image.Rectangle{image.Rect(0, 0, d.dw, d.dh)}})
	}
	var regions []render.Rect
	for _, m := range moves {
		st.moves++
		view := render.MapRect(m.View, d.num, 120)
		if m.DX != 0 || m.DY*d.num%120 != 0 || d.mode == drawGPU && !carries {
			regions = append(regions, view)
			continue
		}
		st.copied++
		dy := m.DY * d.num / 120
		if d.mode == drawGPU {
			to := render.Rect{X: view.X, Y: view.Y + dy, W: view.W, H: view.H}.Intersect(view)
			carry = append(carry, gpu.Carry{From: d.tg[1-d.cur], Rects: []image.Rectangle{image.Rect(to.X, to.Y, to.X+to.W, to.Y+to.H)}, Offset: image.Pt(0, -dy)})
			continue
		}
		moveRows(d.buf, render.Stride(d.dw), view, dy)
	}
	for _, x := range rects {
		regions = append(regions, render.MapRect(x, d.num, 120).Intersect(full))
	}
	regions = disjoint(regions)
	cv := d.cv.Reset(d.buf, render.Stride(d.dw), d.dw, d.dh, d.num, 120)
	if d.mode != drawRaster {
		cv = d.rec.ResetRecorder(d.dw, d.dh, d.num, 120)
	}
	for _, region := range regions {
		prev := cv.PushClipDevice(region)
		cv.ClearDevice(region, d.bg)
		cv.BeginOverlays()
		d.root.Paint(cv)
		cv.FlushOverlays()
		cv.PopClip(prev)
		st.painted += region.W * region.H
	}
	if d.mode != drawRaster {
		l := cv.Recording()
		st.ops = l.Len()
		d.play(t, l, carry)
		l.End()
	}
	painted := time.Now()
	widget.AttachAnimations(d.clock)
	runtime.ReadMemStats(&after)
	st.layout, st.collect, st.paint = laid.Sub(start), collected.Sub(laid), painted.Sub(collected)
	st.allocs, st.bytes = after.Mallocs-before.Mallocs, after.TotalAlloc-before.TotalAlloc
	return st
}

// play hands the frame's List to its player: the raster canvas, or the
// GPU, waiting while the GPU is a frame behind as a window waits for its
// next frame callback.
func (d *frameDriver) play(t *testing.T, l *render.List, carry []gpu.Carry) {
	t.Helper()
	if d.mode == drawRecorded {
		l.Paint(d.cv.Reset(d.buf, render.Stride(d.dw), d.dw, d.dh, d.num, 120))
		return
	}
	for {
		err := d.dev.Render(l, d.tg[d.cur], carry...)
		if errors.Is(err, gpu.ErrBusy) {
			runtime.Gosched()
			continue
		}
		if err != nil {
			t.Fatalf("render a frame on the GPU: %v", err)
		}
		break
	}
	d.drawn[d.cur] = true
	d.cur = 1 - d.cur
}

// moveRows moves area's rows of buf by dy device pixels, dropping what
// falls outside area, as a window's buffer copy does.
func moveRows(buf []byte, stride int, area render.Rect, dy int) {
	to := render.Rect{X: area.X, Y: area.Y + dy, W: area.W, H: area.H}.Intersect(area)
	for i := range to.H {
		y := to.Y + i
		if dy > 0 {
			y = to.Y + to.H - 1 - i
		}
		at := y*stride + to.X*4
		copy(buf[at:at+to.W*4], buf[at-dy*stride:])
	}
}

// disjoint merges overlapping regions, so none paints twice.
func disjoint(rs []render.Rect) []render.Rect {
	for merged := true; merged; {
		merged = false
		for i := 0; i < len(rs); i++ {
			for j := i + 1; j < len(rs); j++ {
				if !rs[i].Intersect(rs[j]).Empty() {
					rs[i] = rs[i].Union(rs[j])
					rs = slices.Delete(rs, j, j+1)
					merged, j = true, j-1
				}
			}
		}
	}
	return slices.DeleteFunc(rs, func(r render.Rect) bool { return r.Empty() })
}

// frameWindow is the settings window over a test kit, hosted in a
// frameDriver at w x h and scale num/120, settled on page.
func frameWindow(t *testing.T, mode drawMode, w, h, num int, page string) (*frameDriver, *window) {
	t.Helper()
	k := testKit(t, "")
	cfg := k.store.svc.Config()
	d := newFrameDriver(mode, time.Unix(1_000_000, 0))
	win := newWindow(k, cfg, d.after)
	theme := apptheme.New(cfg)
	theme.Attach(win.root)
	d.start(t, win.root, w, h, num, theme.RenderPalette().Surface)
	win.navigate(page)
	settle(t, d)
	return d, win
}

// settle draws frames until the switch's timers ran and the window
// owes nothing.
func settle(t *testing.T, d *frameDriver) {
	t.Helper()
	for range 40 {
		d.frame(t)
	}
}

// scenario is one interaction's frames.
type scenario struct {
	name   string
	frames []frameStat
}

func median[T int | uint64 | time.Duration](fs []frameStat, f func(frameStat) T) T {
	v := make([]T, len(fs))
	for i, fr := range fs {
		v[i] = f(fr)
	}
	slices.Sort(v)
	return v[len(v)/2]
}

func (s scenario) String() string {
	us := func(d time.Duration) float64 { return float64(d) / float64(time.Microsecond) }
	total := func(f frameStat) time.Duration { return f.layout + f.collect + f.paint }
	worst := slices.MaxFunc(s.frames, func(a, b frameStat) int { return int(total(a) - total(b)) })
	copied, moves := 0, 0
	for _, f := range s.frames {
		copied, moves = copied+f.copied, moves+f.moves
	}
	return fmt.Sprintf("%-44s n=%3d  layout %7.0f us  paint %8.0f us  total %8.0f us (max %8.0f)  painted %8d px  ops %6d  allocs %6d  bytes %8d  copied %d/%d",
		s.name, len(s.frames),
		us(median(s.frames, func(f frameStat) time.Duration { return f.layout })),
		us(median(s.frames, func(f frameStat) time.Duration { return f.paint })),
		us(median(s.frames, total)), us(total(worst)),
		median(s.frames, func(f frameStat) int { return f.painted }),
		median(s.frames, func(f frameStat) int { return f.ops }),
		median(s.frames, func(f frameStat) uint64 { return f.allocs }),
		median(s.frames, func(f frameStat) uint64 { return f.bytes }),
		copied, moves)
}

// scrollSteps scrolls the open page n times by step and draws each.
func scrollSteps(t *testing.T, d *frameDriver, win *window, name string, n int, step func(*widget.Scroll)) scenario {
	t.Helper()
	win.page.SetOffset(0, 0)
	settle(t, d)
	s := scenario{name: name}
	for range n {
		step(win.page.Scroll)
		s.frames = append(s.frames, d.frame(t))
	}
	return s
}

// switchPages opens from, settles, opens to, and draws the switch: its
// first frame, which builds and lays out the page, and the frames of
// the cross-fade after it.
func switchPages(t *testing.T, d *frameDriver, win *window, from, to string) (first, fade scenario) {
	t.Helper()
	win.navigate(from)
	settle(t, d)
	win.navigate(to)
	first = scenario{name: "switch " + from + " -> " + to + ", first frame", frames: []frameStat{d.frame(t)}}
	fade = scenario{name: "switch " + from + " -> " + to + ", cross-fade"}
	for range int(pageTransition/(time.Second/120)) - 1 {
		fade.frames = append(fade.frames, d.frame(t))
	}
	return first, fade
}

func wheelStep(s *widget.Scroll)    { s.ScrollBy(0, 1) }
func touchpadStep(s *widget.Scroll) { s.ScrollPixels(0, 7.3) }

// TestScrollAndSwitchFramesDoBoundedWork is the regression gate of
// #471, on the settings window's longest page at the owner's laptop
// size, 2880x1800 at scale 1.5, with the GPU path's recording: a scroll
// step copies its view and paints only what it exposed and what floats
// over it, and a page cross-fade, once its first frame painted both
// pages, records no page again. It counts work, never time, and
// allocations only in a build whose instrumentation does not allocate.
func TestScrollAndSwitchFramesDoBoundedWork(t *testing.T) {
	d, win := frameWindow(t, drawRecorded, 1920, 1200, 180, "launcher")
	view := render.MapRect(win.page.Bounds(), 180, 120)
	for _, s := range []scenario{
		scrollSteps(t, d, win, "wheel", 12, wheelStep),
		scrollSteps(t, d, win, "touchpad", 24, touchpadStep),
	} {
		for i, f := range s.frames {
			if f.copied != 1 {
				t.Errorf("%s step %d copied %d of %d moves, want its view copied", s.name, i, f.copied, f.moves)
			}
		}
		if got, limit := median(s.frames, func(f frameStat) int { return f.painted }), view.W*view.H/6; got > limit {
			t.Errorf("%s steps paint %d device px at the median, want the exposed strip and the close button, at most %d", s.name, got, limit)
		}
		if got, limit := median(s.frames, func(f frameStat) uint64 { return f.allocs }), uint64(64); testbuild.AllocBudgetsHold && got > limit {
			t.Errorf("%s steps allocate %d times at the median, budget %d", s.name, got, limit)
		}
	}
	_, fade := switchPages(t, d, win, "wm-binds", "launcher")
	if got, limit := median(fade.frames, func(f frameStat) int { return f.ops }), 16; got > limit {
		t.Errorf("a cross-fade frame records %d ops at the median, want only the composite of the pages it kept, at most %d", got, limit)
	}
	if got, limit := median(fade.frames, func(f frameStat) uint64 { return f.allocs }), uint64(16); testbuild.AllocBudgetsHold && got > limit {
		t.Errorf("a cross-fade frame allocates %d times at the median, budget %d", got, limit)
	}
}

// TestFrameCostReport measures scrolling the longest page and switching
// between heavy pages at the owner's laptop size and at 1920x1080, on
// the CPU renderer and on Vulkan when LYS_TEST_GPU names a device (the
// numbers ui/bench/baseline.md records). It runs only when
// LYS_FRAMECOST is set: the times are for reading, not for gating.
func TestFrameCostReport(t *testing.T) {
	if os.Getenv("LYS_FRAMECOST") == "" {
		t.Skip("LYS_FRAMECOST asks for the frame cost report")
	}
	modes := []drawMode{drawRaster}
	if gputest.Enabled() {
		modes = append(modes, drawGPU)
	}
	for _, mode := range modes {
		for _, size := range [][3]int{{1920, 1200, 180}, {1920, 1080, 120}} {
			d, win := frameWindow(t, mode, size[0], size[1], size[2], "launcher")
			tag := fmt.Sprintf("%s %dx%d@%d/120 ", mode, size[0]*size[2]/120, size[1]*size[2]/120, size[2])
			t.Log(tag, scrollSteps(t, d, win, "launcher wheel", 20, wheelStep))
			t.Log(tag, scrollSteps(t, d, win, "launcher touchpad 7.3 px", 60, touchpadStep))
			for _, pair := range [][2]string{{"wm-binds", "launcher"}, {"bar", "theme"}} {
				first, fade := switchPages(t, d, win, pair[0], pair[1])
				t.Log(tag, first)
				t.Log(tag, fade)
			}
		}
	}
}
