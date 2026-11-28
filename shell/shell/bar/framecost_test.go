package bar

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/reveal"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// frameDropdowns are the dropdowns the frame cost report and gate open:
// the calendar, quick settings and audio.
var frameDropdowns = []struct {
	name  string
	build func(ModuleContext) widget.Widget
}{
	{"calendar", calendarDropdown},
	{"dashboard", dashboardDropdown},
	{"audio", audioDropdown},
}

// frameDropdown is a dropdown's popover as the registry opens it, the
// built panel in its card in its revealer, styled as the bar styles
// it, hosted closed in a frame driver at the popover's size and scale
// num/120.
type frameDropdown struct {
	ctx     ModuleContext
	d       *treetest.Frames
	rev     *widget.Revealer
	content widget.Widget
}

func newFrameDropdown(t *testing.T, mode treetest.DrawMode, num int, name string, build func(ModuleContext) widget.Widget) *frameDropdown {
	t.Helper()
	cfg := config.Defaults()
	ctx := styledContext(t, cfg)
	f := &frameDropdown{ctx: ctx, d: treetest.NewFrames(mode)}
	host := onLoopOf(ctx, func() *widget.Box {
		f.content = dropdownPanel(name, cfg, build(ctx))
		f.rev = dropdownTree(f.content, cfg)
		host := widget.NewBox(widget.Column, 0, 0)
		host.Append(f.rev, true)
		ctx.Theme.Attach(host)
		return host
	})
	doOnLoopOf(ctx, func() {
		size := widget.SizeWithinAsChild(host, widget.Size{W: 1920, H: 1000})
		f.d.Start(t, host, size.W+2*widget.PopoverGap, size.H+2*widget.PopoverGap, num, render.Color(0))
	})
	f.settle(t, 8)
	if c, ok := f.content.(dropdownCloser); ok {
		t.Cleanup(func() { closeView(ctx, c) })
	}
	return f
}

func (f *frameDropdown) frame(t *testing.T) treetest.FrameStat {
	t.Helper()
	return onLoopOf(f.ctx, func() treetest.FrameStat { return f.d.Frame(t) })
}

func (f *frameDropdown) settle(t *testing.T, n int) {
	t.Helper()
	for range n {
		f.frame(t)
	}
}

// open shows the dropdown as the registry does and draws its frames
// until the card lands, then the frames it draws resting open.
func (f *frameDropdown) open(t *testing.T) (opening, rest treetest.Scenario) {
	t.Helper()
	doOnLoopOf(f.ctx, func() {
		reveal.Show(f.rev, f.ctx.Config.Animations, config.AnimDropdown)
		if o, ok := f.content.(dropdownOpener); ok {
			o.dropdownOpened()
		}
	})
	opening = treetest.Scenario{Name: "open"}
	for range 120 {
		opening.Frames = append(opening.Frames, f.frame(t))
		if onLoopOf(f.ctx, func() bool { return f.rev.Progress() >= 1 }) {
			break
		}
	}
	f.settle(t, 2)
	rest = treetest.Scenario{Name: "open, at rest"}
	for range 30 {
		rest.Frames = append(rest.Frames, f.frame(t))
	}
	return opening, rest
}

// close plays the card's exit and draws its frames until it is gone.
func (f *frameDropdown) close(t *testing.T) treetest.Scenario {
	t.Helper()
	gone := false
	doOnLoopOf(f.ctx, func() {
		reveal.Hide(f.rev, f.ctx.Config.Animations, config.AnimDropdown, func() { gone = true })
	})
	s := treetest.Scenario{Name: "close"}
	for range 120 {
		s.Frames = append(s.Frames, f.frame(t))
		if onLoopOf(f.ctx, func() bool { return gone }) {
			break
		}
	}
	return s
}

// TestDropdownFrameCostReport measures opening, resting open and
// closing the calendar, quick settings and audio dropdowns at scale 1.5
// and 1, on the CPU and on Vulkan when LYS_TEST_GPU names a device. It
// runs only when LYS_FRAMECOST is set: the times are for reading, not
// for gating.
func TestDropdownFrameCostReport(t *testing.T) {
	treetest.RequireFrameCost(t)
	for _, mode := range treetest.ReportModes() {
		for _, size := range treetest.ReportSizes {
			for _, dd := range frameDropdowns {
				f := newFrameDropdown(t, mode, size.Num, dd.name, dd.build)
				tag := size.Tag(mode) + " " + dd.name
				opening, rest := f.open(t)
				t.Log(tag, opening)
				t.Log(tag, rest)
				t.Log(tag, f.close(t))
			}
		}
	}
}

// TestDropdownFramesDoBoundedWork is the regression gate of #479 for
// the bar's dropdowns, at the owner's scale through the GPU path's
// recording: opening and closing the calendar, quick settings and
// audio paint at most the card and allocate within budget, and open at
// rest quick settings and audio paint nothing, the calendar no more
// than its blinking separator. It counts work, never time.
func TestDropdownFramesDoBoundedWork(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector allocates")
	}
	for _, dd := range frameDropdowns {
		f := newFrameDropdown(t, treetest.DrawRecorded, treetest.ReportSizes[0].Num, dd.name, dd.build)
		opening, rest := f.open(t)
		closing := f.close(t)
		card := onLoopOf(f.ctx, func() render.Rect { return f.d.DeviceRect(f.rev.Bounds()) })
		for _, s := range []treetest.Scenario{opening, closing} {
			if got, limit := treetest.Median(s, func(f treetest.FrameStat) int { return f.Painted }), card.W*card.H; got > limit {
				t.Errorf("%s: a frame of its %s paints %d device px at the median, want at most its card's %d", dd.name, s.Name, got, limit)
			}
			if got, limit := treetest.Median(s, func(f treetest.FrameStat) uint64 { return f.Allocs }), uint64(16); got > limit {
				t.Errorf("%s: a frame of its %s allocates %d times at the median, budget %d", dd.name, s.Name, got, limit)
			}
		}
		limit := 0
		if dd.name == "calendar" {
			sep := onLoopOf(f.ctx, func() render.Rect {
				return f.d.DeviceRect(treetest.WithClass(f.content, "clock-separator")[0].(widget.Boundser).Bounds())
			})
			limit = render.UniformInsets(2).Grow(sep).W * render.UniformInsets(2).Grow(sep).H
		}
		if got := treetest.Max(rest, func(f treetest.FrameStat) int { return f.Painted }); got > limit {
			t.Errorf("%s: open at rest a frame painted %d device px, want at most %d", dd.name, got, limit)
		}
	}
}
