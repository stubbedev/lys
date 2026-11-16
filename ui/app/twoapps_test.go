package app

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/stubbedev/lys/ui/internal/wlsession"
	"github.com/stubbedev/lys/ui/widget"
)

type twoAppWindow struct {
	h     *paintHarness
	sc    *widget.Scroll
	last  *widget.Label
	sheet *widget.Stylesheet
	pads  [2]int
}

func twoAppSheet(pad int) string {
	return fmt.Sprintf(`.pad { padding: %dpx; container-type: inline-size; } .tall { min-height: 60px; background-color: #3a6; } @container (min-width: 1px) { .tall { color: #123; } }`, pad)
}

func newTwoAppWindow(t *testing.T, pads [2]int) *twoAppWindow {
	t.Helper()
	a := NewTestApplication(t, &wlsession.Session{})
	face, err := render.LoadFont(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	col := widget.NewBox(widget.Column, 2, 4)
	var last *widget.Label
	for i := range 300 {
		last = widget.NewLabel(face, 14, fmt.Sprintf("row %d of the long list", i), 0)
		col.Append(last, false)
	}
	sc := widget.NewScroll(col)
	root := widget.NewBox(widget.Column, 0, 0).Append(sc, true)
	root.AddClass("pad")
	sheet := widget.NewStylesheet(twoAppSheet(pads[0]), widget.StylePriorityApplication)
	root.AttachStylesheet(sheet)
	h := newPaintHarness(root, 400, 300)
	h.wnd.anim = &a.anim
	a.windows = append(a.windows, h.wnd)
	h.frame()
	return &twoAppWindow{h: h, sc: sc, last: last, sheet: sheet, pads: pads}
}

func (w *twoAppWindow) frame(i int) error {
	pad := w.pads[i%2]
	w.sheet.Load(twoAppSheet(pad))
	if i%3 == 0 {
		w.last.AddClass("tall")
	} else {
		w.last.RemoveClass("tall")
	}
	w.sc.SetOffset(0, (i%9)*37)
	w.h.wnd.dirty = true
	if !w.h.wnd.draw() {
		return fmt.Errorf("frame %d: draw: %w", i, w.h.wnd.drawErr)
	}
	for _, b := range w.h.bufs {
		b.Release()
	}
	if got := w.sc.Bounds().X; got != pad {
		return fmt.Errorf("frame %d: the scroll sits at x %d under its window's %dpx padding", i, got, pad)
	}
	if front, whole := w.h.wnd.pool.Front(), wholePaint(w.h); !bytes.Equal(front.Data[:len(whole)], whole) {
		return fmt.Errorf("frame %d: the window's buffer differs from a whole paint", i)
	}
	return nil
}

func TestTwoApplicationsDrawTheirWindowsAtOnce(t *testing.T) {
	windows := [2]*twoAppWindow{
		newTwoAppWindow(t, [2]int{3, 7}),
		newTwoAppWindow(t, [2]int{11, 13}),
	}
	const frames = 40
	errs := make(chan error, 2*frames)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, w := range windows {
		wg.Go(func() {
			<-start
			for i := range frames {
				if err := w.frame(i); err != nil {
					errs <- err
				}
			}
		})
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
