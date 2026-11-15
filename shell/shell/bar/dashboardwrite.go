package bar

import (
	"context"
	"log"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// serviceWrite is a control's write to a service: write runs off the
// loop, bounded as boundedWrite bounds it, and its error, nil on
// success, comes back to done on the loop.
func serviceWrite(ctx ModuleContext, write func(context.Context) error, done func(error)) {
	ctx.Go(func() {
		err := boundedWrite(ctx, write)
		ctx.Invoke(func() { done(err) })
	})
}

// boundedWrite runs write, off the loop, under offLoopTimeout within
// the module's life.
func boundedWrite(ctx ModuleContext, write func(context.Context) error) error {
	c, cancel := context.WithTimeout(ctx.Life(), offLoopTimeout)
	defer cancel()
	return write(c)
}

// writeAlert is where a section says a write failed: lys-ui's
// destructive alert with the warning icon, hidden until a write fails.
func writeAlert(font render.Font, class string) *widget.Alert {
	a := widget.NewAlert(font, "", "")
	a.SetVariant(widget.AlertDestructive)
	a.SetIcon(widget.NewThemeIcon("ld-alert-triangle-symbolic", itemIconPx))
	a.AddClass(class)
	a.SetVisible(false)
	return a
}

// reportWrite shows a failed write in alert under title, with the
// error as its description, and logs it under module; a write that
// went through hides the alert.
func reportWrite(alert *widget.Alert, module, title string, err error) {
	alert.SetVisible(err != nil)
	if err == nil {
		return
	}
	log.Printf("%s: %s: %v", module, title, err)
	alert.SetTitle(title)
	alert.SetDescription(err.Error())
}
