// Package sessionctl runs the power menu's built-in session actions in
// process: lock, suspend, reboot and power off through logind, and log
// out through lys-wm while it runs, else logind.
package sessionctl

import (
	"context"
	"errors"
	"fmt"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/lyswm"
	"github.com/stubbedev/lys/lib/logind"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// ErrNoSystemBus is a built-in that needs logind asked of a shell with
// no system bus.
var ErrNoSystemBus = errors.New("no system bus to reach logind on")

// SystemBus hands the connection logind is reached over, asked only
// when the action needs logind.
type SystemBus func() (*dbus.Conn, error)

// On is the SystemBus of an open connection; nil is ErrNoSystemBus.
func On(conn *dbus.Conn) SystemBus {
	return func() (*dbus.Conn, error) {
		if conn == nil {
			return nil, ErrNoSystemBus
		}
		return conn, nil
	}
}

var powerActions = map[config.PowerBuiltin]logind.PowerAction{
	config.PowerBuiltinSuspend:  logind.Suspend,
	config.PowerBuiltinReboot:   logind.Reboot,
	config.PowerBuiltinPowerOff: logind.PowerOff,
}

// Run runs b: Lock has logind lock the session "auto" resolves to, whose
// Lock signal the lock screen answers; Suspend, Reboot and PowerOff ask
// logind for them interactively, so where polkit needs authentication
// the session's agent prompts and the action waits for the answer, as
// long as ctx allows; Logout is Logout.
func Run(ctx context.Context, system SystemBus, b config.PowerBuiltin) error {
	if b == config.PowerBuiltinLogout {
		return Logout(ctx, system)
	}
	conn, err := system()
	if err != nil {
		return fmt.Errorf("%s: %w", b, err)
	}
	m := logind.NewManager(conn)
	if b == config.PowerBuiltinLock {
		err = m.LockSession(ctx, logind.AutoSession)
	} else if action, ok := powerActions[b]; ok {
		err = m.PowerInteractively(ctx, action)
	} else {
		err = fmt.Errorf("unknown built-in %d", uint8(b))
	}
	if err != nil {
		return fmt.Errorf("%s: %w", b, err)
	}
	return nil
}

// Logout ends the user's session: lys-wm's exit action while lys-wm
// runs, else logind's Terminate of the session "auto" resolves to.
func Logout(ctx context.Context, system SystemBus) error {
	if err := logout(ctx, system); err != nil {
		return fmt.Errorf("log out: %w", err)
	}
	return nil
}

func logout(ctx context.Context, system SystemBus) error {
	if lyswm.IsRunning() {
		return lyswm.Dispatch("exit")
	}
	conn, err := system()
	if err != nil {
		return err
	}
	session, err := logind.NewManager(conn).GetSession(ctx, logind.AutoSession)
	if err != nil {
		return err
	}
	return session.Terminate(ctx)
}
