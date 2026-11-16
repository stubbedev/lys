package bar

import (
	"errors"
	"testing"
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
