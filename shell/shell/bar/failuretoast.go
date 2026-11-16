package bar

import "log"

// alertIcon is the warning icon a failure is shown with.
const alertIcon = "ld-alert-triangle-symbolic"

// toastFailure logs message, which says what failed and why, under module
// and shows it in an alert toast; toast is nil without an OSD, and then
// the log alone has it.
func toastFailure(toast func(label, icon string), module, message string) {
	log.Printf("%s: %s", module, message)
	if toast != nil {
		toast(message, alertIcon)
	}
}
