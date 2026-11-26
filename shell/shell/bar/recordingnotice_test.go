package bar

import (
	"testing"

	"github.com/stubbedev/lys/shell/service/notifications"
)

func TestASavedRecordingsNotificationOpensTheFileOnAClickAndTheFolderOnItsButton(t *testing.T) {
	const path = "/home/u/Videos/rec.mp4"
	if got, ok := recordingTarget(notifications.DefaultActionID, path); !ok || got != path {
		t.Errorf("a click on the body opens %q (%v), want the recording", got, ok)
	}
	if got, ok := recordingTarget(recordingOpenFolder, path); !ok || got != "/home/u/Videos" {
		t.Errorf("the folder button opens %q (%v), want its folder", got, ok)
	}
	if got, ok := recordingTarget("delete", path); ok {
		t.Errorf("a key the notification never posts opened %q", got)
	}
}
