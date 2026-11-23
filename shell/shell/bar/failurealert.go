package bar

import (
	"log"

	"github.com/stubbedev/lys/desktop/shell/widgets"
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
	if err != nil {
		log.Printf("%s: %s: %v", module, title, err)
	}
	showFailure(alert, title, err)
}

// showFailure shows err in alert under title, the summary in the
// shell's words, with the error's own text as its description; a nil
// err hides the alert. The one place the bar puts an error's text on
// screen, with failurePage and showFailureDetail.
func showFailure(alert *widget.Alert, title string, err error) {
	alert.SetVisible(err != nil)
	if err == nil {
		return
	}
	alert.SetTitle(title)
	alert.SetDescription(err.Error())
}

// failurePage is a page saying title, what failed in the shell's
// words, with err's own text as its description.
func failurePage(font render.Font, px float64, icon, title string, err error) *widget.StatusPage {
	return widgets.EmptyState(font, px, icon, title, err.Error())
}

// showFailureDetail puts err's own text in detail, the line a failure
// page shows under its title.
func showFailureDetail(detail *widget.Label, err error) { detail.SetText(err.Error()) }

// sameErrorText says a and b are both errors reading the same, so a
// failure that repeats is logged once.
func sameErrorText(a, b error) bool {
	return a != nil && b != nil && a.Error() == b.Error()
}
