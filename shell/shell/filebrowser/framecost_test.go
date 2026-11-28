package filebrowser

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/ui/widget"
)

// framePictures is how many pictures, and as many text files, the
// frame cost folder holds.
const framePictures = 200

// frameFolder writes the frame cost folder under root: pictures to
// thumbnail and text files beside them, interleaved by name.
func frameFolder(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, "many")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 320, 240))
	for y := range 240 {
		for x := range 320 {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	var pic bytes.Buffer
	if err := png.Encode(&pic, img); err != nil {
		t.Fatal(err)
	}
	for i := range framePictures {
		for name, data := range map[string][]byte{
			fmt.Sprintf("holiday %03d.png", i): pic.Bytes(),
			fmt.Sprintf("holiday %03d.txt", i): []byte("notes\n"),
		} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return dir
}

// frameBrowser is the browser's window over the frame cost folder in
// view (the list, or the grid), hosted in a frame driver at w x h and
// scale num/120, every thumbnail in view landed.
func frameBrowser(t *testing.T, mode treetest.DrawMode, size treetest.ReportSize, grid bool) (*harness, *treetest.Frames) {
	t.Helper()
	hs := newHarnessWith(t, filepath.Join(t.TempDir(), "view"), func(d *Deps) {
		d.Animations = func() config.AnimationsConfig { return config.Defaults().Animations }
	})
	hs.open(t, in(Options{}, frameFolder(t, hs.root)))
	d := treetest.NewFrames(mode)
	hs.run(func() {
		if grid {
			hs.c.toggleView()
		}
		hs.cfg.Root.(interface {
			AttachStylesheet(*widget.Stylesheet)
		}).AttachStylesheet(hs.theme.Sheet)
		d.Start(t, hs.cfg.Root, size.W/2, size.H*2/3, size.Num, hs.theme.RenderPalette().Surface)
	})
	hs.settle()
	settleThumbs(t, hs, d)
	return hs, d
}

// settleThumbs draws until the thumbnails in view landed.
func settleThumbs(t *testing.T, hs *harness, d *treetest.Frames) {
	t.Helper()
	for range 3 {
		hs.frames(t, d, 4)
		hs.waitThumbs(t)
	}
	hs.frames(t, d, 4)
}

func (hs *harness) frames(t *testing.T, d *treetest.Frames, n int) {
	t.Helper()
	for range n {
		hs.run(func() { d.Frame(t) })
	}
}

// waitThumbs waits until no thumbnail lookup is running.
func (hs *harness) waitThumbs(t *testing.T) {
	t.Helper()
	hs.settle()
	waittest.Until(t, func() bool {
		var waiting int
		hs.run(func() { waiting = len(hs.c.thumbs.waiting) })
		return waiting == 0
	}, "the thumbnails in view")
	hs.flush()
}

// scrollView scrolls the active view n times by step, a step every
// interval frames, and draws every frame.
func scrollView(t *testing.T, hs *harness, d *treetest.Frames, name string, n, interval int, step func(*widget.List)) treetest.Scenario {
	t.Helper()
	s := treetest.Scenario{Name: name}
	for i := range n * interval {
		hs.run(func() {
			if i%interval == 0 {
				step(hs.c.activeView())
			}
			s.Frames = append(s.Frames, d.Frame(t))
		})
	}
	return s
}

// openQuicklook selects name and opens Quick Look on it as Space does,
// drawing its frames from the key until the card has entered (load),
// the frames of its enter, and the frames it then draws at rest.
func openQuicklook(t *testing.T, hs *harness, d *treetest.Frames, name string) (load, enter, rest treetest.Scenario) {
	t.Helper()
	hs.pick(t, name)
	hs.frames(t, d, 4)
	load = treetest.Scenario{Name: "quick look, key to its content"}
	enter = treetest.Scenario{Name: "quick look, its enter"}
	rest = treetest.Scenario{Name: "quick look, open at rest"}
	hs.run(hs.c.toggleQuicklook)
	for range 240 {
		var loading, landed bool
		var st treetest.FrameStat
		hs.run(func() {
			loading = hs.c.ui.ql.Renderer() == preview.Loading
			st = d.Frame(t)
			landed = hs.c.ui.quicklook.rev.Progress() >= 1
		})
		if loading {
			load.Frames = append(load.Frames, st)
		} else {
			enter.Frames = append(enter.Frames, st)
		}
		if landed {
			break
		}
		hs.flush()
	}
	for range 30 {
		hs.run(func() { rest.Frames = append(rest.Frames, d.Frame(t)) })
	}
	return load, enter, rest
}

func wheelStep(l *widget.List)    { l.ScrollBy(0, 1) }
func touchpadStep(l *widget.List) { l.ScrollPixels(0, 7.3) }

// TestBrowserFrameCostReport measures scrolling the list and the grid
// of a folder of pictures at the owner's laptop size and at 1920x1080,
// on the CPU and on Vulkan when LYS_TEST_GPU names a device. It runs
// only when LYS_FRAMECOST is set: the times are for reading, not for
// gating.
func TestBrowserFrameCostReport(t *testing.T) {
	treetest.RequireFrameCost(t)
	for _, mode := range treetest.ReportModes() {
		for _, size := range treetest.ReportSizes {
			for _, grid := range []bool{false, true} {
				hs, d := frameBrowser(t, mode, size, grid)
				tag := size.Tag(mode)
				view := "list"
				if grid {
					view = "grid"
				}
				t.Log(tag, scrollView(t, hs, d, view+" wheel, a step every 2 frames", 20, 2, wheelStep))
				t.Log(tag, scrollView(t, hs, d, view+" touchpad 7.3 px", 60, 1, touchpadStep))
				if !grid {
					load, enter, rest := openQuicklook(t, hs, d, "holiday 003.png")
					t.Log(tag, load)
					t.Log(tag, enter)
					t.Log(tag, rest)
				}
			}
		}
	}
}

// TestQuicklookOpensRepaintingOnlyItsCard is the regression gate of
// #479 for Quick Look, at the owner's laptop size through the GPU
// path's recording: the key that opens it repaints the card's rect,
// never the views it lies over (all of them before), its enter paints
// at most the card, and open at rest it paints nothing. Closed, it
// lies over nothing: a touchpad step on the list under it copies the
// view and paints only the strip it exposed. It counts work, never
// time.
func TestQuicklookOpensRepaintingOnlyItsCard(t *testing.T) {
	hs, d := frameBrowser(t, treetest.DrawRecorded, treetest.ReportSizes[0], false)
	var view render.Rect
	hs.run(func() { view = d.DeviceRect(hs.c.activeView().Bounds()) })
	load, enter, rest := openQuicklook(t, hs, d, "holiday 003.png")
	var card render.Rect
	hs.run(func() { card = d.DeviceRect(hs.c.ui.quicklook.rev.Bounds()) })
	if got, limit := treetest.Max(load, func(f treetest.FrameStat) int { return f.Painted }), card.W*card.H; got > limit {
		t.Errorf("the key opening Quick Look painted %d device px, want at most its card's %d of the views' %d", got, limit, view.W*view.H)
	}
	if got, limit := treetest.Median(enter, func(f treetest.FrameStat) int { return f.Painted }), card.W*card.H; got > limit {
		t.Errorf("Quick Look's enter paints %d device px at the median, want at most its card's %d", got, limit)
	}
	if got := treetest.Max(rest, func(f treetest.FrameStat) int { return f.Painted }); got != 0 {
		t.Errorf("Quick Look open at rest painted %d device px in a frame, want none", got)
	}
	hs.run(hs.c.closeQuicklook)
	hs.frames(t, d, 60)
	hs.run(func() {
		if hs.c.ui.quicklook.shown {
			t.Fatal("Quick Look is still up after its exit")
		}
	})
	s := scrollView(t, hs, d, "touchpad under closed Quick Look", 12, 1, func(l *widget.List) { l.ScrollPixels(0, 8) })
	copied := 0
	for _, f := range s.Frames {
		copied += f.Copied
	}
	if copied == 0 {
		t.Error("no touchpad step under closed Quick Look copied the list")
	}
	if got, limit := treetest.Median(s, func(f treetest.FrameStat) int { return f.Painted }), view.W*view.H/6; got > limit {
		t.Errorf("touchpad steps under closed Quick Look paint %d device px at the median, want the exposed strip, at most %d", got, limit)
	}
}
