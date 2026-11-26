package bar

import (
	"log"
	"path/filepath"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"

	"github.com/stubbedev/lys/shell/internal/desktopnotify"
	"github.com/stubbedev/lys/shell/service/notifications"
)

const (
	recordingOpenFolder = "open-folder"
	recordingNoticeWait = 30 * time.Second
)

// announceRecording posts the notification for a recording saved at
// path: a click on it opens the file and its button the folder, through
// open, off the caller's goroutine.
func announceRecording(open func(target string), summary, path, icon string) {
	go func() {
		key, ok, err := desktopnotify.AskOnce(i18n.T("notification-source-recorder"), summary, path, icon, []desktopnotify.Action{
			{Key: notifications.DefaultActionID, Label: i18n.T("recorder-action-open")},
			{Key: recordingOpenFolder, Label: i18n.T("recorder-action-open-folder")},
		}, recordingNoticeWait)
		if err != nil {
			log.Printf("recorder: notification: %v", err)
			return
		}
		if !ok {
			return
		}
		if target, ok := recordingTarget(key, path); ok {
			open(target)
		}
	}()
}

// recordingTarget is what the action key of a saved recording's
// notification opens: the file for a click on the body, its folder for
// the button, nothing for a key it does not post.
func recordingTarget(key, path string) (string, bool) {
	switch key {
	case notifications.DefaultActionID:
		return path, true
	case recordingOpenFolder:
		return filepath.Dir(path), true
	}
	return "", false
}

// openRecording shows a saved recording, or its folder, in the default
// app for it, and toasts a failure.
func (s *shellStart) openRecording(target string) {
	s.application.Invoke(func() {
		s.application.OpenPath(target, func(err error) {
			if err != nil {
				toastFailure(s.ctx.Toast, "recorder", i18n.T("recorder-open-failed"), err)
			}
		})
	})
}
