package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/internal/waittest"
	"github.com/stubbedev/lys/ui/internal/wlsession"
	"github.com/stubbedev/lys/ui/media"
	"github.com/stubbedev/lys/ui/widget"
)

type widgetHooks struct {
	invoker, stylesheetWatcher, gpuHost, videoHost, textureHost bool
}

var allWidgetHooks = widgetHooks{true, true, true, true, true}

func (s *stopSet) added() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.next
}

func hooksOf(t *testing.T, a *Application) widgetHooks {
	t.Helper()
	var got widgetHooks
	box := render.Rect{W: 64, H: 48}

	icon := widget.NewSVGIcon([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><rect width="16" height="16" fill="#e04020"/></svg>`), 16)
	icon.Arrange(render.Rect{W: 16, H: 16})
	pixels := make([]byte, render.Stride(16)*16)
	icon.Paint(render.New(pixels, render.Stride(16), 16, 16))
	widget.WaitAssetLoads()
	got.invoker = slices.Max(pixels) == 0

	path := filepath.Join(t.TempDir(), "hooks.css")
	if err := os.WriteFile(path, []byte("label { color: red; }"), 0o600); err != nil {
		t.Fatal(err)
	}
	watched := a.watchers.added()
	sheet, err := widget.AddStylesheetFile(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	got.stylesheetWatcher = a.watchers.added() == watched+1
	sheet.Remove()

	area := widget.NewGPUArea()
	area.Arrange(box)
	got.gpuHost = area.Err() != nil

	yuv, err := media.NewY4M(strings.NewReader("YUV4MPEG2 W2 H2 F25:1 C420jpeg\nFRAME\n\x10\x10\x10\x10\x80\x80"))
	if err != nil {
		t.Fatal(err)
	}
	frames := media.NewStream(a, yuv, "hooks")
	t.Cleanup(frames.Close)
	painted := widget.NewVideo(testFace(t), 13, frames)
	painted.Arrange(box)
	got.videoHost = painted.OffloadErr() == nil

	textures := media.NewStream(a, UndecodedTextures{}, "hooks")
	t.Cleanup(textures.Close)
	widget.NewVideo(testFace(t), 13, textures).Arrange(box)
	got.textureHost = textures.Err() == nil
	return got
}

func TestCloseTakesOutTheWidgetHooksOfItsApplicationAndNoOther(t *testing.T) {
	first := NewTestApplication(t, &wlsession.Session{})
	second := NewTestApplication(t, &wlsession.Session{})
	if got := hooksOf(t, second); got != allWidgetHooks {
		t.Fatalf("with two applications the widgets reach the second through %+v, want every hook", got)
	}

	first.Close()
	if got := hooksOf(t, second); got != allWidgetHooks {
		t.Errorf("closing the first application left the second's hooks as %+v, want every one in place", got)
	}

	second.Close()
	if got := hooksOf(t, second); got != (widgetHooks{}) {
		t.Errorf("closing the second application left the hooks %+v, want none", got)
	}
}

type appLoop struct {
	work   chan func()
	exited chan struct{}
}

func newAppLoop(t *testing.T) *appLoop {
	t.Helper()
	l := &appLoop{work: make(chan func()), exited: make(chan struct{})}
	go func() {
		defer close(l.exited)
		for fn := range l.work {
			fn()
		}
	}()
	t.Cleanup(func() {
		close(l.work)
		waittest.For(t, l.exited, "an application loop to exit")
	})
	return l
}

func (l *appLoop) do(t *testing.T, fn func()) {
	t.Helper()
	ran := make(chan struct{})
	l.work <- func() {
		defer close(ran)
		fn()
	}
	waittest.For(t, ran, "work on an application loop")
}

type hookedApp struct {
	loop   *appLoop
	app    *Application
	box    *widget.Box
	target *tipTarget
	router *widget.Router
}

func newHookedApp(t *testing.T) *hookedApp {
	t.Helper()
	s := &hookedApp{loop: newAppLoop(t)}
	s.loop.do(t, func() {
		s.app = NewTestApplication(t, &wlsession.Session{})
		s.target = newTipTarget("")
		s.box = widget.NewBox(widget.Row, 0, 0)
		s.box.Append(s.target, false)
		h := newPaintHarness(s.box, 100, 20)
		s.app.windows = append(s.app.windows, h.wnd)
		widget.SizeWithin(s.box, widget.Size{W: 100, H: 20})
		s.box.Arrange(render.Rect{W: 100, H: 20})
		s.router = h.wnd.router
		s.router.Move(widget.Point{X: 5, Y: 5})
	})
	if s.router.Hovered() != s.target {
		t.Fatal("the pointer over the window's widget hovers nothing")
	}
	return s
}

func TestTwoApplicationsOnTwoLoopsEachHearTheirOwnWidgets(t *testing.T) {
	first, second := newHookedApp(t), newHookedApp(t)

	first.loop.do(t, func() { first.box.Remove(first.target) })
	if first.router.Hovered() != nil {
		t.Error("removing a widget on the first application's loop left its window hovering the removed widget")
	}
	if second.router.Hovered() != second.target {
		t.Error("removing a widget on the first application's loop dropped the hover of the second's window")
	}

	watchedByLatest := allWidgetHooks
	watchedByLatest.stylesheetWatcher = false
	for _, c := range []struct {
		s    *hookedApp
		want widgetHooks
	}{{first, watchedByLatest}, {second, allWidgetHooks}} {
		var got widgetHooks
		c.s.loop.do(t, func() { got = hooksOf(t, c.s.app) })
		if got != c.want {
			t.Errorf("with two applications on two loops a loop's widgets reach its application through %+v, want %+v", got, c.want)
		}
	}

	second.app.Close()
	var got widgetHooks
	first.loop.do(t, func() { got = hooksOf(t, first.app) })
	if got != allWidgetHooks {
		t.Errorf("closing the application installed last left the first's hooks as %+v, want every one in place", got)
	}
	second.loop.do(t, func() { got = hooksOf(t, second.app) })
	if got != (widgetHooks{}) {
		t.Errorf("the closed application's loop still reaches hooks %+v, want none", got)
	}
}
