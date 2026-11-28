package launcher

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
)

// frameApps are the words the frame cost panel's applications are named
// from: every pair, so a typed prefix finds many and narrows as it grows.
var frameApps = [2][]string{
	{"Fire", "File", "Film", "Font", "Photo", "Paint", "Power", "Print", "Text", "Term", "Disk", "Data", "Mail", "Music", "Map", "Media"},
	{"fox", "Manager", "Viewer", "Editor", "Studio", "Monitor", "Player", "Browser", "Settings", "Tool", "Writer", "Reader", "Mixer", "Center", "Shot"},
}

var frameIcons = []string{"ld-search-symbolic", "ld-calculator-symbolic", "ld-globe-symbolic", "ld-link-symbolic", "ld-activity-symbolic"}

// framePanel is the search panel over 240 installed applications, the
// owner's animations on, hosted in a frame driver at w x h and scale
// num/120 and settled open.
func framePanel(t *testing.T, mode treetest.DrawMode, size treetest.ReportSize) (*panelHarness, *panel, *treetest.Frames) {
	t.Helper()
	useBundledIcons(t)
	h := newPanelHarness(t)
	apps := filepath.Join(os.Getenv("XDG_DATA_HOME"), "applications")
	if err := os.MkdirAll(apps, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	n := 0
	for _, a := range frameApps[0] {
		for _, b := range frameApps[1] {
			entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=%s%s\nExec=true\nIcon=%s\n", a, b, frameIcons[n%len(frameIcons)])
			if err := os.WriteFile(filepath.Join(apps, fmt.Sprintf("app%03d.desktop", n)), []byte(entry), 0o600); err != nil {
				t.Fatal(err)
			}
			n++
		}
	}
	h.cfg.Animations.Enabled = true
	h.cfg.Launcher.Search.Providers = []config.LauncherSearchProvider{config.SearchApps, config.SearchCalculator, config.SearchWeb}
	h.cfg.Launcher.Search.Previews = false
	h.cfg.Launcher.Lines = 5
	h.s.d.Sheet = apptheme.New(h.cfg).Sheet
	d := hostFrames(t, h, mode, size)
	p, _ := h.openPanel()
	p.v.list.setScale(size.Num, 120)
	h.typed(p, "e")
	h.until("the applications", func() bool { return len(p.frame.Rows) >= 8 })
	settleFrames(t, h, p, d)
	h.typed(p, "")
	settleFrames(t, h, p, d)
	return h, p, d
}

// hostFrames hosts the surface h opens next in a frame driver at w x h
// and scale num/120, as the window hosts its root as it maps.
func hostFrames(t *testing.T, h *panelHarness, mode treetest.DrawMode, size treetest.ReportSize) *treetest.Frames {
	t.Helper()
	d := treetest.NewFrames(mode)
	open := h.s.d.Open
	h.s.d.Open = func(c app.LayerConfig) (Window, error) {
		d.Start(t, c.Root, size.W, size.H, size.Num, render.Color(0))
		return open(c)
	}
	return d
}

// settleFrames draws until the panel's springs rest and its icons came.
func settleFrames(t *testing.T, h *panelHarness, p *panel, d *treetest.Frames) {
	t.Helper()
	for range 60 {
		h.loop.pump()
		d.Frame(t)
	}
	h.until("the rows' icons", func() bool { return len(p.v.list.waiting) == 0 })
	d.Settle(t, 4)
}

// typeKeys types text one key at a time and draws each key: the loop's
// work showing the rows the engine sent back (apply), the key's first
// frame, and the frames after it until the next key, a key every
// interval at 120 frames a second.
func typeKeys(t *testing.T, h *panelHarness, p *panel, d *treetest.Frames, text string, interval int) (apply, first, after treetest.Scenario) {
	t.Helper()
	apply = treetest.Scenario{Name: "keystroke, the rows applied on the loop"}
	first = treetest.Scenario{Name: "keystroke, its frame"}
	after = treetest.Scenario{Name: "keystroke, the frames after"}
	for i := range text {
		typed := text[:i+1]
		p.v.Entry.SetText(typed)
		var st treetest.FrameStat
		waited := measureLoop(&st, func() {
			h.until("rows for "+typed, func() bool { return p.frame.Query == typed })
		})
		st.Layout = waited
		apply.Frames = append(apply.Frames, st)
		first.Frames = append(first.Frames, d.Frame(t))
		for range interval - 1 {
			h.loop.pump()
			after.Frames = append(after.Frames, d.Frame(t))
		}
	}
	return apply, first, after
}

// measureLoop runs f and adds its heap allocations to st.
func measureLoop(st *treetest.FrameStat, f func()) time.Duration {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	f()
	took := time.Since(start)
	runtime.ReadMemStats(&after)
	st.Allocs, st.Bytes = after.Mallocs-before.Mallocs, after.TotalAlloc-before.TotalAlloc
	return took
}

// wheelList turns the wheel over the list n times, a step every interval
// frames, and draws every frame.
func wheelList(t *testing.T, h *panelHarness, p *panel, d *treetest.Frames, name string, n, interval int) treetest.Scenario {
	t.Helper()
	return d.Run(t, name, n*interval, func(i int) {
		h.loop.pump()
		if i%interval == 0 {
			p.v.list.ScrollBy(0, 1)
		}
	})
}

// TestLauncherFrameCostReport measures typing into and scrolling the
// search panel at the owner's laptop size and at 1920x1080, on the CPU
// and on Vulkan when LYS_TEST_GPU names a device. It runs only when
// LYS_FRAMECOST is set: the times are for reading, not for gating.
func TestLauncherFrameCostReport(t *testing.T) {
	treetest.RequireFrameCost(t)
	for _, mode := range treetest.ReportModes() {
		for _, size := range treetest.ReportSizes {
			h, p, d := framePanel(t, mode, size)
			tag := size.Tag(mode)
			apply, first, after := typeKeys(t, h, p, d, "photo", 10)
			t.Log(tag, apply)
			t.Log(tag, first)
			t.Log(tag, after)
			h.typed(p, "e")
			settleFrames(t, h, p, d)
			t.Log(tag, wheelList(t, h, p, d, "results wheel, a step every 4 frames", 3, 4))
			h.stop()
			h, p, d, path := quickLookFrames(t, mode, size)
			open, rest, closing := lookFrames(t, h, p, d, path)
			t.Log(tag, open)
			t.Log(tag, rest)
			t.Log(tag, closing)
			h.stop()
		}
	}
}

// TestLauncherKeystrokeFramesPaintOnlyThePanel is the regression gate
// of #479 for typing, at the owner's laptop size through the GPU path's
// recording: a key's frame and the frames of the panel springing to its
// new height repaint the panel, never the output it floats on (all of
// it painted on every frame before), allocate within budget, and a
// panel at rest paints nothing. It counts work, never time.
func TestLauncherKeystrokeFramesPaintOnlyThePanel(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector allocates")
	}
	size := treetest.ReportSizes[0]
	h, p, d := framePanel(t, treetest.DrawRecorded, size)
	output := d.DeviceRect(p.v.root.Bounds())
	_, first, after := typeKeys(t, h, p, d, "photo", 10)
	for _, s := range []treetest.Scenario{first, after} {
		painted := treetest.Median(s, func(f treetest.FrameStat) int { return f.Painted })
		if limit := output.W * output.H / 4; painted > limit {
			t.Errorf("%s paints %d device px at the median, want the panel's, at most %d of the output's %d", s.Name, painted, limit, output.W*output.H)
		}
		t.Logf("%s", s)
	}
	if got, limit := treetest.Median(first, func(f treetest.FrameStat) uint64 { return f.Allocs }), uint64(320); got > limit {
		t.Errorf("a keystroke's frame allocates %d times at the median, budget %d", got, limit)
	}
	if got, limit := treetest.Median(after, func(f treetest.FrameStat) uint64 { return f.Allocs }), uint64(8); got > limit {
		t.Errorf("a frame of the panel springing allocates %d times at the median, budget %d", got, limit)
	}
	settleFrames(t, h, p, d)
	rest := d.Run(t, "at rest", 10, func(int) { h.loop.pump() })
	if got := treetest.Max(rest, func(f treetest.FrameStat) int { return f.Painted }); got != 0 {
		t.Errorf("a panel at rest painted %d device px in a frame, want none", got)
	}
}

// lookFrames opens Quick Look on the selected copied file and draws its
// frames until it shows the text (open), the frames it draws at rest,
// and the frame of its key closing it.
func lookFrames(t *testing.T, h *panelHarness, p *panel, d *treetest.Frames, path string) (open, rest, closing treetest.Scenario) {
	t.Helper()
	q := p.v.ql
	open = treetest.Scenario{Name: "quick look, key to its content"}
	if !h.key("Shift+space") {
		t.Fatal("Shift+Space was not taken")
	}
	for range 120 {
		h.loop.pump()
		open.Frames = append(open.Frames, d.Frame(t))
		if q.shows == quickLookFile && q.file.Path() == path && q.file.Renderer() == preview.Text {
			break
		}
	}
	rest = d.Run(t, "quick look, open at rest", 20, func(int) { h.loop.pump() })
	if !h.key("Shift+space") {
		t.Fatal("the second Shift+Space was not taken")
	}
	closing = d.Run(t, "quick look, its close", 1, nil)
	return open, rest, closing
}

// quickLookFrames is a search panel over a copied file, hosted in a
// frame driver and settled with the file selected.
func quickLookFrames(t *testing.T, mode treetest.DrawMode, size treetest.ReportSize) (*panelHarness, *panel, *treetest.Frames, string) {
	t.Helper()
	h, path := quickLookHarness(t)
	h.s.d.Sheet = apptheme.New(h.cfg).Sheet
	d := hostFrames(t, h, mode, size)
	p, _ := h.openPanel()
	h.typed(p, "notes.txt")
	h.until("the copied file selected", func() bool { return isFile(path)(p.selectedRow()) })
	settleFrames(t, h, p, d)
	return h, p, d, path
}

// TestLauncherQuickLookRepaintsOnlyItsCard is the regression gate of
// #479 for the launcher's Quick Look, at the owner's laptop size
// through the GPU path's recording: opening and closing it repaint the
// card's rect, never the output the panel floats on (all of it before),
// and open at rest only the field's caret blinks. It counts work, never
// time.
func TestLauncherQuickLookRepaintsOnlyItsCard(t *testing.T) {
	size := treetest.ReportSizes[0]
	h, p, d, path := quickLookFrames(t, treetest.DrawRecorded, size)
	output := d.DeviceRect(p.v.root.Bounds())
	open, rest, closing := lookFrames(t, h, p, d, path)
	card := d.DeviceRect(widget.InkOutset(p.v.ql.card).Grow(p.v.ql.card.Bounds()))
	for _, s := range []treetest.Scenario{open, closing} {
		if got, limit := treetest.Max(s, func(f treetest.FrameStat) int { return f.Painted }), 2*card.W*card.H; got > limit {
			t.Errorf("%s painted %d device px in a frame, want the card's, at most %d of the output's %d", s.Name, got, limit, output.W*output.H)
		}
	}
	caret := d.DeviceRect(p.v.Entry.Bounds()).H * 4
	if got := treetest.Max(rest, func(f treetest.FrameStat) int { return f.Painted }); got > caret {
		t.Errorf("Quick Look open at rest painted %d device px in a frame, want at most the field's blinking caret, %d", got, caret)
	}
}
