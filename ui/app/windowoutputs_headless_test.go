package app_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stubbedev/lys/lib/wltest"

	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/internal/headlesstest/inproc"
	"github.com/stubbedev/lys/ui/widget"
)

func TestHeadlessWindowListsTheOutputItIsOn(t *testing.T) {
	wltest.Require(t)
	var (
		a    *app.Application
		win  *app.Window
		name string
	)
	heard := make(chan struct{}, 16)
	inproc.Show(t, func(application *app.Application, out *app.Output) error {
		a, name = application, out.Name
		if name == "" {
			return errors.New("the compositor names no output")
		}
		var err error
		win, err = a.NewWindow(app.WindowConfig{
			Title: "outputs", Root: widget.NewDrawingArea(64, 64),
			OnOutputs: func() {
				select {
				case heard <- struct{}{}:
				default:
				}
			},
		})
		if err != nil {
			return err
		}
		win.FullscreenOn(out)
		return nil
	})
	wltest.Eventually(t, "the window on its output", func() error {
		names := make(chan []string, 1)
		a.Invoke(func() {
			var got []string
			for _, o := range win.Outputs() {
				got = append(got, o.Name)
			}
			names <- got
		})
		if got := <-names; len(got) != 1 || got[0] != name {
			return fmt.Errorf("on %v, want [%s]", got, name)
		}
		return nil
	})
	if len(heard) == 0 {
		t.Error("OnOutputs heard nothing while the window entered its output")
	}
}
