package app_test

import (
	"testing"

	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/wltest"

	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/internal/headlesstest/inproc"
	"github.com/stubbedev/lys/ui/widget"
)

func TestHeadlessADialogOpensWithoutAParentWindow(t *testing.T) {
	wltest.Require(t)
	answers := make(chan string, 2)
	inproc.Show(t, func(a *app.Application, out *app.Output) error {
		layer, err := a.NewLayer(app.LayerConfig{Output: out, Layer: app.LayerTop, Anchor: app.AnchorTop, Namespace: "lys-ui-dialog-parent", Root: widget.NewDrawingArea(100, 30)})
		if err != nil {
			return err
		}
		ok := []app.DialogButton{{Label: "OK", Response: "ok"}}
		modal, err := a.NewDialog(nil, app.DialogConfig{Title: "Alert", Description: "an application-level alert", Buttons: ok, Modal: true, OnResponse: func(r string) { answers <- r }})
		if err != nil {
			return err
		}
		beside, err := a.NewDialog(layer, app.DialogConfig{Title: "Beside", Description: "a dialog a layer surface asked for", Buttons: ok, OnResponse: func(r string) { answers <- r }})
		if err != nil {
			return err
		}
		modal.Respond("ok")
		beside.Respond("ok")
		return nil
	})
	for range 2 {
		if got := waittest.For(t, answers, "a dialog's answer"); got != "ok" {
			t.Fatalf("a dialog answered %q, want ok", got)
		}
	}
}
