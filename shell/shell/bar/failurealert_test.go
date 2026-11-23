package bar

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/logtest"
	"github.com/stubbedev/lys/ui/widget"
)

func TestReportWriteShowsAFailureAndHidesASuccess(t *testing.T) {
	alert := writeAlert(testFont(t), "x-failed")
	if alert.Visible() {
		t.Fatal("the alert shows before any write")
	}
	reportWrite(alert, "test", "It failed", errors.New("org.freedesktop.DBus.Error.AccessDenied"))
	if !alert.Visible() || alert.Title() != "It failed" || alert.Description() != "org.freedesktop.DBus.Error.AccessDenied" {
		t.Errorf("after a failure: visible %v title %q description %q", alert.Visible(), alert.Title(), alert.Description())
	}
	reportWrite(alert, "test", "It failed", nil)
	if alert.Visible() {
		t.Error("a write that went through left the alert up")
	}
}

// wantFailure requires alert to show title, a summary in the shell's
// words, with the error, holding errPart, only in its description.
func wantFailure(t *testing.T, what string, alert *widget.Alert, title, errPart string) {
	t.Helper()
	if !alert.Visible() || alert.Title() != title || strings.Contains(alert.Title(), errPart) || !strings.Contains(alert.Description(), errPart) {
		t.Errorf("%s: alert shown %v %q / %q, want %q with %q in the description alone", what, alert.Visible(), alert.Title(), alert.Description(), title, errPart)
	}
}

func TestToastFailureToastsTheSummaryAndLogsTheError(t *testing.T) {
	logs := logtest.Capture(t)
	var toasts []string
	toastFailure(func(label, _ string) { toasts = append(toasts, label) }, "dock", "Could not start Firefox", errors.New("exec: firefox: not found"))
	if !slices.Equal(toasts, []string{"Could not start Firefox"}) {
		t.Errorf("toasts %q, want the summary alone", toasts)
	}
	if got := logs.String(); !strings.Contains(got, "dock: Could not start Firefox: exec: firefox: not found") {
		t.Errorf("log %q, want the summary with its error", got)
	}
	toastFailure(nil, "dock", "Could not start Firefox", errors.New("no OSD"))
	if got := logs.String(); !strings.Contains(got, "no OSD") {
		t.Errorf("without an OSD the log %q lost the error", got)
	}
}
