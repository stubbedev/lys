package treetest

import (
	"errors"
	"fmt"
	"image"
	"os"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/gpu"
	"github.com/stubbedev/lys/lib/gputest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/widget"
)

// DrawMode is how a Frames driver turns a frame into pixels.
type DrawMode int

const (
	// DrawRaster paints on the CPU, as lys-ui's shared-memory path.
	DrawRaster DrawMode = iota
	// DrawRecorded records a List and plays it onto a raster canvas:
	// the GPU path's recording and layer keeping, the CPU as player.
	DrawRecorded
	// DrawGPU records a List and renders it with Vulkan, as lys-ui's
	// GPU path, onto two offscreen targets in turn.
	DrawGPU
)

func (m DrawMode) String() string {
	return [...]string{"cpu", "recorded", "vulkan"}[m]
}

// ReportModes are the modes a frame cost report measures: the CPU, and
// Vulkan when LYS_TEST_GPU names a device.
func ReportModes() []DrawMode {
	if gputest.Enabled() {
		return []DrawMode{DrawRaster, DrawGPU}
	}
	return []DrawMode{DrawRaster}
}

// ReportSize is one output a frame cost report measures at: W x H
// logical pixels at device scale Num/120.
type ReportSize struct{ W, H, Num int }

// ReportSizes are the owner's laptop, 2880x1800 at scale 1.5, and a
// 1920x1080 output at scale 1.
var ReportSizes = []ReportSize{{1920, 1200, 180}, {1920, 1080, 120}}

// Tag names mode and size in a report line.
func (s ReportSize) Tag(mode DrawMode) string {
	return fmt.Sprintf("%s %dx%d@%d/120", mode, s.W*s.Num/120, s.H*s.Num/120, s.Num)
}

// Frames draws a widget tree's frames headless the way a lys-ui window
// draws them: layout, the damage drain, each scroll move at a whole
// device step copied in place, and the owed regions painted over the
// window background. Frames come at 120 a second of the local clock,
// the one animations launch on, a late frame where it lands; the tree's
// animations run on the driver's clock, and a timer handed to After
// fires once its frame comes.
type Frames struct {
	root   widget.Widget
	w, h   int
	num    int
	dw, dh int
	bg     render.Color
	mode   DrawMode
	buf    []byte
	cv     *render.Canvas
	rec    *render.Canvas
	damage widget.DamageCollector
	clock  *anim.Clock
	now    time.Time
	timers []*frameTimer

	dev   *gpu.Device
	tg    [2]*gpu.Target
	drawn [2]bool
	cur   int
}

type frameTimer struct {
	at      time.Time
	fn      func()
	stopped bool
}

// FrameStat is what one frame cost: the CPU time of its phases (Paint
// covers recording and handing the List to the GPU), the device pixels
// it painted, the List ops it recorded, its heap allocations, and the
// scroll moves it copied of those it found.
type FrameStat struct {
	Layout, Collect, Paint time.Duration
	Painted, Ops           int
	Allocs, Bytes          uint64
	Moves, Copied          int
}

// Total is the frame's CPU time.
func (f FrameStat) Total() time.Duration { return f.Layout + f.Collect + f.Paint }

// NewFrames is a driver drawing in mode. Host a tree with Start.
func NewFrames(mode DrawMode) *Frames {
	return &Frames{mode: mode, now: time.Now(), clock: new(anim.Clock)}
}

// Now is the driver's time.
func (d *Frames) Now() time.Time { return d.now }

// After is the window's timer: fn runs in the frame dur from now. The
// returned func cancels it.
func (d *Frames) After(dur time.Duration, fn func()) func() {
	tm := &frameTimer{at: d.now.Add(dur), fn: fn}
	d.timers = append(d.timers, tm)
	return func() { tm.stopped = true }
}

// Start hosts root in a w x h window at device scale num/120 over bg.
func (d *Frames) Start(t testing.TB, root widget.Widget, w, h, num int, bg render.Color) {
	t.Helper()
	d.root, d.w, d.h, d.num, d.bg = root, w, h, num, bg
	d.dw, d.dh = w*num/120, h*num/120
	stride := render.Stride(d.dw)
	d.buf = make([]byte, stride*d.dh)
	d.cv = render.NewScaled(d.buf, stride, d.dw, d.dh, num, 120)
	if d.mode != DrawRaster {
		d.rec = render.NewRecorder(d.dw, d.dh, num, 120)
	}
	if d.mode == DrawGPU {
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

// DeviceRect is r, in the window's logical pixels, on the device grid.
func (d *Frames) DeviceRect(r render.Rect) render.Rect { return render.MapRect(r, d.num, 120) }

func (d *Frames) tick() {
	d.now = d.now.Add(time.Second / 120)
	if ahead := time.Until(d.now); ahead > 0 {
		time.Sleep(ahead)
	} else {
		d.now = time.Now()
	}
	d.clock.Tick(d.now)
	due := d.timers
	d.timers = nil
	for _, tm := range due {
		switch {
		case tm.stopped:
		case tm.at.After(d.now):
			d.timers = append(d.timers, tm)
		default:
			tm.fn()
		}
	}
}

// Settle draws n frames, for timers and animations to run out.
func (d *Frames) Settle(t testing.TB, n int) {
	t.Helper()
	for range n {
		d.Frame(t)
	}
}

// Frame advances the clock one frame and draws what the tree owes.
func (d *Frames) Frame(t testing.TB) FrameStat {
	t.Helper()
	d.tick()
	var st FrameStat
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
	carries := d.mode == DrawGPU && d.drawn[1-d.cur]
	var carry []gpu.Carry
	if carries {
		carry = append(carry, gpu.Carry{From: d.tg[1-d.cur], Rects: []image.Rectangle{image.Rect(0, 0, d.dw, d.dh)}})
	}
	var regions []render.Rect
	for _, m := range moves {
		st.Moves++
		view := render.MapRect(m.View, d.num, 120)
		if m.DX != 0 || m.DY*d.num%120 != 0 || d.mode == DrawGPU && !carries {
			regions = append(regions, view)
			continue
		}
		st.Copied++
		dy := m.DY * d.num / 120
		if d.mode == DrawGPU {
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
	if d.mode != DrawRaster {
		cv = d.rec.ResetRecorder(d.dw, d.dh, d.num, 120)
	}
	for _, region := range regions {
		prev := cv.PushClipDevice(region)
		cv.ClearDevice(region, d.bg)
		cv.BeginOverlays()
		d.root.Paint(cv)
		cv.FlushOverlays()
		cv.PopClip(prev)
		st.Painted += region.W * region.H
	}
	if d.mode != DrawRaster {
		l := cv.Recording()
		st.Ops = l.Len()
		d.play(t, l, carry)
		l.End()
	}
	painted := time.Now()
	widget.AttachAnimations(d.clock)
	runtime.ReadMemStats(&after)
	st.Layout, st.Collect, st.Paint = laid.Sub(start), collected.Sub(laid), painted.Sub(collected)
	st.Allocs, st.Bytes = after.Mallocs-before.Mallocs, after.TotalAlloc-before.TotalAlloc
	return st
}

// play hands the frame's List to its player: the raster canvas, or the
// GPU, waiting while the GPU is a frame behind as a window waits for its
// next frame callback.
func (d *Frames) play(t testing.TB, l *render.List, carry []gpu.Carry) {
	t.Helper()
	if d.mode == DrawRecorded {
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

// Scenario is one interaction's frames.
type Scenario struct {
	Name   string
	Frames []FrameStat
}

// Median is the median over the scenario's frames of what f reads.
func Median[T int | uint64 | time.Duration](s Scenario, f func(FrameStat) T) T {
	v := make([]T, len(s.Frames))
	for i, fr := range s.Frames {
		v[i] = f(fr)
	}
	slices.Sort(v)
	return v[len(v)/2]
}

// Max is the most over the scenario's frames of what f reads.
func Max[T int | uint64 | time.Duration](s Scenario, f func(FrameStat) T) T {
	var m T
	for _, fr := range s.Frames {
		m = max(m, f(fr))
	}
	return m
}

func (s Scenario) String() string {
	us := func(d time.Duration) float64 { return float64(d) / float64(time.Microsecond) }
	copied, moves := 0, 0
	for _, f := range s.Frames {
		copied, moves = copied+f.Copied, moves+f.Moves
	}
	return fmt.Sprintf("%-40s n=%3d  layout %7.0f us  paint %8.0f us  total %8.0f us (max %8.0f)  painted %8d px  ops %6d  allocs %6d  bytes %9d  copied %d/%d",
		s.Name, len(s.Frames),
		us(Median(s, func(f FrameStat) time.Duration { return f.Layout })),
		us(Median(s, func(f FrameStat) time.Duration { return f.Paint })),
		us(Median(s, FrameStat.Total)), us(Max(s, FrameStat.Total)),
		Median(s, func(f FrameStat) int { return f.Painted }),
		Median(s, func(f FrameStat) int { return f.Ops }),
		Median(s, func(f FrameStat) uint64 { return f.Allocs }),
		Median(s, func(f FrameStat) uint64 { return f.Bytes }),
		copied, moves)
}

// Run draws n frames, calling step before each when it is not nil.
func (d *Frames) Run(t testing.TB, name string, n int, step func(i int)) Scenario {
	t.Helper()
	s := Scenario{Name: name, Frames: make([]FrameStat, 0, n)}
	for i := range n {
		if step != nil {
			step(i)
		}
		s.Frames = append(s.Frames, d.Frame(t))
	}
	return s
}

// RequireFrameCost skips a report unless LYS_FRAMECOST asks for it: its
// times are for reading, never for gating.
func RequireFrameCost(t testing.TB) {
	t.Helper()
	if os.Getenv("LYS_FRAMECOST") == "" {
		t.Skip("LYS_FRAMECOST asks for the frame cost report")
	}
}
