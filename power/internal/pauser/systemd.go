package pauser

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/pause"
)

const (
	SystemdName    = "org.freedesktop.systemd1"
	SystemdPath    = dbus.ObjectPath("/org/freedesktop/systemd1")
	SystemdManager = SystemdName + ".Manager"
	SystemdUnit    = SystemdName + ".Unit"
	ErrNoSuchUnit  = SystemdName + ".NoSuchUnit"
)

var ErrNoManager = errors.New("no systemd manager runs for the user")

type Manager interface {
	Running(ctx context.Context, name string) (bool, error)
	Freeze(ctx context.Context, name string) error
	Thaw(ctx context.Context, name string) error
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
}

type Managers interface {
	System() Manager
	User(ctx context.Context, uid uint32) (Manager, func() error, error)
}

type Systemd struct{ Conn *dbus.Conn }

func (s Systemd) manager() dbus.BusObject { return s.Conn.Object(SystemdName, SystemdPath) }

func (s Systemd) Running(ctx context.Context, name string) (bool, error) {
	var path dbus.ObjectPath
	err := s.manager().CallWithContext(ctx, SystemdManager+".GetUnit", 0, name).Store(&path)
	if reply, ok := errors.AsType[dbus.Error](err); ok && reply.Name == ErrNoSuchUnit {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("find %s: %w", name, err)
	}
	unit := s.Conn.Object(SystemdName, path)
	active, err := stringProperty(ctx, unit, "ActiveState")
	if err != nil {
		return false, fmt.Errorf("read whether %s is active: %w", name, err)
	}
	freezer, err := stringProperty(ctx, unit, "FreezerState")
	if err != nil {
		return false, fmt.Errorf("read whether %s is frozen: %w", name, err)
	}
	return (active == "active" || active == "reloading") && freezer == "running", nil
}

func stringProperty(ctx context.Context, unit dbus.BusObject, name string) (string, error) {
	var v dbus.Variant
	if err := unit.CallWithContext(ctx, dbusx.PropertiesInterface+".Get", 0, SystemdUnit, name).Store(&v); err != nil {
		return "", err
	}
	text, ok := v.Value().(string)
	if !ok {
		return "", fmt.Errorf("%s is a %s, not a string", name, v.Signature())
	}
	return text, nil
}

func (s Systemd) Freeze(ctx context.Context, name string) error {
	return s.call(ctx, "FreezeUnit", name)
}

func (s Systemd) Thaw(ctx context.Context, name string) error {
	return s.call(ctx, "ThawUnit", name)
}

func (s Systemd) Start(ctx context.Context, name string) error {
	return s.call(ctx, "StartUnit", name, "replace")
}

func (s Systemd) Stop(ctx context.Context, name string) error {
	return s.call(ctx, "StopUnit", name, "replace")
}

func (s Systemd) call(ctx context.Context, method, name string, args ...any) error {
	if err := s.manager().CallWithContext(ctx, SystemdManager+"."+method, 0, append([]any{name}, args...)...).Err; err != nil {
		return fmt.Errorf("%s %s: %w", method, name, err)
	}
	return nil
}

type Bus struct {
	SystemConn     *dbus.Conn
	UserRuntimeDir string
}

func (b Bus) System() Manager { return Systemd{Conn: b.SystemConn} }

func (b Bus) User(ctx context.Context, uid uint32) (Manager, func() error, error) {
	path := filepath.Join(b.UserRuntimeDir, strconv.FormatUint(uint64(uid), 10), "systemd", "private")
	conn, err := dbus.Dial("unix:path="+path, dbus.WithContext(ctx))
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
		return nil, nil, fmt.Errorf("uid %d at %s: %w", uid, path, ErrNoManager)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("connect to the systemd manager of uid %d at %s: %w", uid, path, err)
	}
	if err := conn.Auth(nil); err != nil {
		return nil, nil, errors.Join(fmt.Errorf("authenticate to the systemd manager of uid %d at %s: %w", uid, path, err), conn.Close())
	}
	return Systemd{Conn: conn}, conn.Close, nil
}

func pauseUnit(ctx context.Context, m Manager, u pause.Unit) error {
	if u.Held() {
		return m.Stop(ctx, u.Name())
	}
	return m.Freeze(ctx, u.Name())
}

func resumeUnit(ctx context.Context, m Manager, u pause.Unit) error {
	if u.Held() {
		return m.Start(ctx, u.Name())
	}
	return m.Thaw(ctx, u.Name())
}
