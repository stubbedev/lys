package launcher

import (
	"errors"
	"fmt"
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"

	"github.com/stubbedev/lys/shell/internal/launcheripc"
)

type fakeWalls struct {
	images   []string
	shown    map[string]string
	previews []string
	ended    int
	commits  [][2]string
	loads    []string
	// commitErr refuses every commit.
	commitErr error
	saves     []func(error)
}

func (f *fakeWalls) Images() ([]string, error)    { return f.images, nil }
func (f *fakeWalls) Shown(monitor string) string  { return f.shown[monitor] }
func (f *fakeWalls) EndPreview()                  { f.ended++ }
func (f *fakeWalls) Preview(path, _ string) error { f.previews = append(f.previews, path); return nil }

func (f *fakeWalls) Commit(path, mon string, saved func(error)) error {
	f.commits = append(f.commits, [2]string{path, mon})
	if f.commitErr != nil {
		return f.commitErr
	}
	f.saves = append(f.saves, saved)
	return nil
}

func (f *fakeWalls) save(t *testing.T, err error) {
	t.Helper()
	if len(f.saves) == 0 {
		t.Fatal("no commit waits on its save")
	}
	saved := f.saves[len(f.saves)-1]
	f.saves = f.saves[:len(f.saves)-1]
	saved(err)
}

func (f *fakeWalls) LoadThumbnail(path string) (image.Image, error) {
	f.loads = append(f.loads, path)
	return image.NewRGBA(image.Rect(0, 0, 4, 3)), nil
}

type pickerHarness struct {
	*harness
	walls  *fakeWalls
	queued []func()
	reply  <-chan launcheripc.ServerFrame
}

func newPickerHarness(t *testing.T, images ...string) *pickerHarness {
	t.Helper()
	h := &pickerHarness{harness: newHarness(t), walls: &fakeWalls{images: images, shown: map[string]string{}}}
	h.s.d.Wallpapers = h.walls
	h.s.d.FocusedOutput = func() (string, bool) { return "DP-1", true }
	h.s.spawn = func(fn func()) { h.queued = append(h.queued, fn) }
	return h
}

func (h *pickerHarness) openPicker() *picker {
	h.t.Helper()
	mode := WallpaperMode
	_, h.reply = h.open(launcheripc.SessionOptions{Mode: &mode}, false)
	h.until("the picker", func() bool { return h.s.picker != nil })
	return h.s.picker
}

func (h *pickerHarness) press(spec string) {
	h.t.Helper()
	accel, err := app.ParseAccel(spec)
	if err != nil {
		h.t.Fatal(err)
	}
	if !h.windows[len(h.windows)-1].cfg.KeyCapture(accel) {
		h.t.Fatalf("%s went to the field", spec)
	}
}

func (h *pickerHarness) layout(p *picker) {
	p.grid.Arrange(render.Rect{W: pickerColumns * p.cellPx[0], H: pickerLines * p.cellPx[1]})
}

func (h *pickerHarness) runWork() {
	for len(h.queued) > 0 {
		fn := h.queued[0]
		h.queued = h.queued[1:]
		fn()
	}
	h.loop.pump()
}

func (h *pickerHarness) settle() {
	h.t.Helper()
	for _, tm := range h.timers {
		if tm.d == wallpaperSettle && !tm.stopped {
			tm.stopped = true
			tm.fn()
		}
	}
}

func names(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("/walls/img%04d.png", i)
	}
	return out
}

func TestPickerListsEveryImageAndTheFilterNarrowsByName(t *testing.T) {
	h := newPickerHarness(t, "/w/beach.png", "/w/beach-night.png", "/w/forest.jpg")
	p := h.openPicker()
	if len(p.shown) != 3 {
		t.Fatalf("%d images shown, want 3", len(p.shown))
	}
	p.filter("beach")
	if got := len(p.shown); got != 2 {
		t.Errorf("beach shows %d images, want 2", got)
	}
	p.filter("forest")
	if path, _ := p.path(0); len(p.shown) != 1 || path != "/w/forest.jpg" {
		t.Errorf("forest shows %v", p.shown)
	}
	p.filter("nothing-like-it")
	if len(p.shown) != 0 || !p.chrome.Message.Visible() {
		t.Errorf("an unmatched filter shows %d images, message %t", len(p.shown), p.chrome.Message.Visible())
	}
}

func TestPickerThumbnailsOnlyTheCellsInView(t *testing.T) {
	h := newPickerHarness(t, names(1000)...)
	p := h.openPicker()
	h.layout(p)
	for range 300 {
		h.press("Down")
		h.layout(p)
	}
	h.runWork()
	if limit := pickerColumns * (pickerLines + 1); len(h.walls.loads) > limit {
		t.Fatalf("arrowing through 1,000 images made %d thumbnails, want at most %d", len(h.walls.loads), limit)
	}
	t.Logf("%d thumbnails made", len(h.walls.loads))
	selected, _ := p.path(p.grid.Selected())
	if !slices.Contains(h.walls.loads, selected) {
		t.Errorf("the selected image %s has no thumbnail; made %v", selected, h.walls.loads)
	}
	loads := len(h.walls.loads)
	for range 2 {
		h.layout(p)
	}
	h.runWork()
	if len(h.walls.loads) != loads {
		t.Errorf("laying the same cells out again made %d more thumbnails", len(h.walls.loads)-loads)
	}
}

func TestPickerEscapeAfterAPreviewChangesNothing(t *testing.T) {
	h := newPickerHarness(t, "/w/a.png", "/w/b.png", "/w/c.png")
	h.walls.shown["DP-1"] = "/w/a.png"
	p := h.openPicker()
	if p.grid.Selected() != 0 {
		t.Fatalf("the picker opened on %d, want the shown image", p.grid.Selected())
	}
	h.settle()
	if len(h.walls.previews) != 0 {
		t.Fatalf("opening previewed %v", h.walls.previews)
	}
	h.press("Right")
	h.settle()
	if !slices.Equal(h.walls.previews, []string{"/w/b.png"}) {
		t.Fatalf("previews %v, want b once the selection rested", h.walls.previews)
	}
	h.press("Escape")
	if h.walls.ended != 1 || len(h.walls.commits) != 0 {
		t.Errorf("escape: %d preview ends, commits %v; want the preview ended and nothing set", h.walls.ended, h.walls.commits)
	}
	if f := <-h.reply; f.Type != launcheripc.FrameCancelled {
		t.Errorf("escape answered %+v", f)
	}
}

func TestPickerArrowingPastAnImagePreviewsOnlyWhereItRests(t *testing.T) {
	h := newPickerHarness(t, names(12)...)
	h.layout(h.openPicker())
	h.press("Right")
	h.press("Right")
	h.press("Down")
	h.settle()
	if !slices.Equal(h.walls.previews, []string{"/walls/img0006.png"}) {
		t.Errorf("previews %v, want only where the selection rested", h.walls.previews)
	}
}

func TestPickerEnterSetsTheFocusedMonitorAndControlEnterEveryOne(t *testing.T) {
	h := newPickerHarness(t, "/w/a.png", "/w/b.png")
	h.openPicker()
	h.press("Right")
	h.press("Return")
	if h.walls.ended != 0 || h.s.picker == nil {
		t.Errorf("enter closed the picker before the wallpaper was saved: %d preview ends", h.walls.ended)
	}
	h.press("Return")
	if len(h.walls.saves) != 1 {
		t.Errorf("enter while saving committed again: %v", h.walls.commits)
	}
	h.walls.save(t, nil)
	if !slices.Equal(h.walls.commits, [][2]string{{"/w/b.png", "DP-1"}}) || h.walls.ended != 1 {
		t.Errorf("enter committed %v, %d preview ends", h.walls.commits, h.walls.ended)
	}
	if f := <-h.reply; f.Type != launcheripc.FrameResult || len(f.Selected) != 1 || f.Selected[0].Text != "/w/b.png" {
		t.Errorf("enter answered %+v", f)
	}

	all := newPickerHarness(t, "/w/a.png", "/w/b.png")
	all.cfg.Launcher.Monitor = "HDMI-1"
	p := all.openPicker()
	if p.monitor != "HDMI-1" {
		t.Errorf("launcher.monitor HDMI-1 set the picker on %q", p.monitor)
	}
	all.press("Control+Return")
	all.walls.save(t, nil)
	if !slices.Equal(all.walls.commits, [][2]string{{"/w/a.png", ""}}) {
		t.Errorf("control+enter committed %v, want every monitor", all.walls.commits)
	}
}

// A wallpaper that cannot be set says why as text, what the error
// holds of markup escaped, and the picker stays open.
func TestPickerAFailedCommitSaysTheErrorAsText(t *testing.T) {
	h := newPickerHarness(t, "/w/a.png")
	h.walls.commitErr = errors.New("write <b>a.png</b>: disk full")
	p := h.openPicker()
	h.press("Return")
	markup := p.chrome.Message.Markup()
	if !p.chrome.Message.Visible() || !strings.Contains(markup, "&lt;b&gt;a.png&lt;/b&gt;: disk full") || strings.Contains(markup, "<b>") {
		t.Errorf("the failed commit shows %q (visible %t), want the error as text", markup, p.chrome.Message.Visible())
	}
	if h.s.picker != p {
		t.Error("the failed commit closed the picker")
	}
}

func TestPickerAFailedSaveSaysTheErrorAndStaysOpen(t *testing.T) {
	h := newPickerHarness(t, "/w/a.png")
	p := h.openPicker()
	h.press("Return")
	h.walls.save(t, errors.New("config: save: write <runtime.yaml>: read-only file system"))
	markup := p.chrome.Message.Markup()
	if !p.chrome.Message.Visible() || !strings.Contains(markup, "&lt;runtime.yaml&gt;: read-only file system") {
		t.Errorf("the failed save shows %q (visible %t), want the error as text", markup, p.chrome.Message.Visible())
	}
	if h.s.picker != p || h.walls.ended != 0 {
		t.Errorf("the failed save closed the picker: %d preview ends", h.walls.ended)
	}
	h.press("Return")
	h.walls.save(t, nil)
	if h.s.picker != nil || h.walls.ended != 1 {
		t.Errorf("a save that worked left the picker open: %d preview ends", h.walls.ended)
	}
	if f := <-h.reply; f.Type != launcheripc.FrameResult || len(f.Selected) != 1 || f.Selected[0].Text != "/w/a.png" {
		t.Errorf("the saved wallpaper answered %+v", f)
	}
}

func TestPickerASaveAfterEscapeAnswersNothingMore(t *testing.T) {
	h := newPickerHarness(t, "/w/a.png")
	h.openPicker()
	h.press("Return")
	h.press("Escape")
	if f := <-h.reply; f.Type != launcheripc.FrameCancelled {
		t.Errorf("escape while saving answered %+v", f)
	}
	h.walls.save(t, errors.New("config: save: disk full"))
	select {
	case f := <-h.reply:
		t.Errorf("the late save answered %+v", f)
	default:
	}
}

func TestPickerWithoutTheWallpaperServiceCancels(t *testing.T) {
	h := newHarness(t)
	mode := WallpaperMode
	_, reply := h.open(launcheripc.SessionOptions{Mode: &mode}, false)
	h.loop.pump()
	if f := <-reply; f.Type != launcheripc.FrameCancelled || h.s.picker != nil || len(h.windows) != 0 {
		t.Errorf("no wallpaper service: %+v, picker %v, %d windows", f, h.s.picker, len(h.windows))
	}
}

func TestLauncherSurfacesAskForBlurOnlyWhileTranslucent(t *testing.T) {
	h := newPickerHarness(t, "/w/a.png")
	h.openPicker()
	if !h.windows[0].cfg.BackgroundBlur {
		t.Fatal("a translucent launcher asks for no blur")
	}
	h.press("Escape")
	h.cfg.Styling.ReduceTransparency = true
	h.openPicker()
	if h.windows[1].cfg.BackgroundBlur {
		t.Error("reduce-transparency still asks for blur")
	}
}
