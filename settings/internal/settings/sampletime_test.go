package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/barview"
)

// A clock format's preview read 15:09 local beside the bar preview's
// 09:41 UTC, two different times for one sample. A format previews at
// the bar preview's moment, whatever the local time zone.
func TestAFormatPreviewsTheBarPreviewsMoment(t *testing.T) {
	text, err := strftimeFormat().render("%Y-%m-%d %H:%M")
	if err != nil {
		t.Fatal(err)
	}
	if want := barview.SampleTime.Format("2006-01-02 15:04"); text != want {
		t.Errorf("the format previews %q, the bar preview %q", text, want)
	}
}
