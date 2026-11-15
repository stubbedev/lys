package backlight

import (
	"context"
	"os"
	"path/filepath"
	"strconv"

	"github.com/stubbedev/lys/lib/logind"
)

func writeSysfs(name string, value uint32) error {
	return os.WriteFile(filepath.Join(Dir, name, "brightness"), //nolint:gosec // sysfs ignores the mode; the kernel file has its own permissions
		[]byte(strconv.FormatUint(uint64(value), 10)), 0o644)
}

// logindBacklight drives logind's SetBrightness for the user's session,
// resolving it once, on the first write that can: "auto" is the display
// session for a process outside any session, a service of the systemd
// user manager, where "self" is none. Its caller serialises set.
type logindBacklight struct {
	manager logind.Manager
	session *logind.Session
}

func (l *logindBacklight) set(ctx context.Context, name string, value uint32) error {
	if l.session == nil {
		s, err := l.manager.GetSession(ctx, logind.AutoSession)
		if err != nil {
			return err
		}
		l.session = &s
	}
	return l.session.SetBrightness(ctx, logind.Backlight, name, value)
}
