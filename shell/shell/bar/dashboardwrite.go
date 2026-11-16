package bar

import (
	"log"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// writeAlert is where a section says a write failed: lys-ui's
// destructive alert with the warning icon, hidden until a write fails.
func writeAlert(font render.Font, class string) *widget.Alert {
	a := widget.NewAlert(font, "", "")
	a.SetVariant(widget.AlertDestructive)
	a.SetIcon(widget.NewThemeIcon(alertIcon, itemIconPx))
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
