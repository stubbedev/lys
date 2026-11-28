package bar

import (
	"log"

	"github.com/stubbedev/lys/desktop/i18n"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/shell/internal/desktopnotify"
	"github.com/stubbedev/lys/shell/service/automount"
	"github.com/stubbedev/lys/shell/service/udisks"
)

// startAutomount mounts removable drives as they are plugged in, and
// those plugged in at start, for the session, while drives.automount
// is on; with no file manager window open as with one.
func (s *shellStart) startAutomount(system, session *dbus.Conn) func() {
	stop, err := automount.Start(automount.Deps{
		Drives:   udisks.New(system),
		Notifier: desktopnotify.NewSender(session),
		Enabled:  func() bool { return s.current.Load().Drives.Automount },
		Open:     s.openVolume,
	})
	if err != nil {
		log.Printf("automount: off: %v", err)
		return func() {}
	}
	return stop
}

// openVolume shows a mounted volume in the default app for folders,
// and toasts a failure.
func (s *shellStart) openVolume(dir string) {
	s.application.Invoke(func() {
		s.application.OpenPath(dir, func(err error) {
			if err != nil {
				toastFailure(s.ctx.Toast, "automount", i18n.T("volume-open-failed", i18n.Str("path", dir)), err)
			}
		})
	})
}
