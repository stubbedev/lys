package bar

import "log"

// alertIcon is the warning icon a failure is shown with.
const alertIcon = "ld-alert-triangle-symbolic"

// toastFailure shows summary, what failed in the shell's words, in an
// alert toast and logs it with err under module: a toast is one line
// with no room for a detail, so the error's own text stays in the log.
// toast is nil without an OSD, and then the log alone has it.
func toastFailure(toast func(label, icon string), module, summary string, err error) {
	log.Printf("%s: %s: %v", module, summary, err)
	if toast != nil {
		toast(summary, alertIcon)
	}
}
