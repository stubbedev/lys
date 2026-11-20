package notifyview

import (
	"strings"

	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/identity"
	"github.com/stubbedev/lys/desktop/service/notifications/notifymodel"
)

// AppLabel is the name a notification goes by on screen. notify-send and
// the shell's own name say who delivered it, not what it is about, so
// those show as the system. Everything else is the app's own name.
func AppLabel(appName string) string {
	if appName == notifySendName || appName == identity.Name {
		return i18n.T("notification-source-system")
	}
	return appName
}

// notifySendName is what notify-send calls itself without -a.
const notifySendName = "notify-send"

// BodyMarkup is the sanitized body: markup
// lys-ui's parser accepts passes through with its tags intact, anything
// else escapes to the literal characters.
func BodyMarkup(body string) string {
	if _, ok := widget.ParseMarkup(body); ok {
		return body
	}
	repl := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return repl.Replace(body)
}

// UrgencyClass is the CSS class of an urgency: low, normal, or critical.
func UrgencyClass(u notifymodel.Urgency) string {
	switch u {
	case notifymodel.UrgencyLow:
		return "low"
	case notifymodel.UrgencyCritical:
		return "critical"
	}
	return "normal"
}

// UrgencyBarVisible reports whether an urgency reaches the bar's threshold.
func UrgencyBarVisible(u notifymodel.Urgency, threshold config.UrgencyBarThreshold) bool {
	switch threshold {
	case config.UrgencyBarThresholdLow:
		return true
	case config.UrgencyBarThresholdNormal:
		return u >= notifymodel.UrgencyNormal
	case config.UrgencyBarThresholdCritical:
		return u >= notifymodel.UrgencyCritical
	}
	return false
}

// PurposeClass is the style class a button purpose gives an action
// button: accepting a call is the suggested action, declining or hanging
// up the destructive one; "" for any other.
func PurposeClass(purpose string) string {
	switch purpose {
	case "call.accept":
		return css.SuggestedAction
	case "call.decline", "call.hang-up":
		return css.DestructiveAction
	}
	return ""
}

// ActionsPerRow is how many action buttons share a row.
const ActionsPerRow = 3
