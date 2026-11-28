// Package fprintd is fprintd's D-Bus interface as lys-login and
// lys-settings speak it: its names and its errors, and the reader it
// offers.
package fprintd

import (
	"context"
	"errors"
	"fmt"
	"slices"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const (
	BusName      = "net.reactivated.Fprint"
	ManagerPath  = dbus.ObjectPath("/net/reactivated/Fprint/Manager")
	ManagerIface = BusName + ".Manager"
	DeviceIface  = BusName + ".Device"
	// ErrorNoSuchDevice is the manager's answer on a machine without a
	// reader, ErrorNoEnrolledPrints a device's for a user without a
	// finger enrolled, and ErrorPermissionDenied its answer to a caller
	// polkit refused.
	ErrorNoSuchDevice     = BusName + ".Error.NoSuchDevice"
	ErrorNoEnrolledPrints = BusName + ".Error.NoEnrolledPrints"
	ErrorPermissionDenied = BusName + ".Error.PermissionDenied"
)

// ErrNoReader is a machine without a fingerprint reader, or without
// fprintd to drive one.
var ErrNoReader = errors.New("fprintd: no fingerprint reader")

// noReader are the answers to GetDefaultDevice of a machine without a
// reader: fprintd's own, or the bus's for a daemon that is not there.
var noReader = []string{ErrorNoSuchDevice, "org.freedesktop.DBus.Error.ServiceUnknown", "org.freedesktop.DBus.Error.NameHasNoOwner"}

// DefaultDevice is the path of the reader fprintd offers on conn; an
// error matching ErrNoReader without one.
func DefaultDevice(ctx context.Context, conn *dbus.Conn) (dbus.ObjectPath, error) {
	var dev dbus.ObjectPath
	err := conn.Object(BusName, ManagerPath).CallWithContext(ctx, ManagerIface+".GetDefaultDevice", 0).Store(&dev)
	if IsError(err, noReader...) {
		return "", fmt.Errorf("fprintd: find a fingerprint reader: %w: %w", ErrNoReader, err)
	}
	if err != nil {
		return "", fmt.Errorf("fprintd: find a fingerprint reader: %w", err)
	}
	return dev, nil
}

// IsError reports whether err is a D-Bus error with one of names.
func IsError(err error, names ...string) bool {
	de, ok := errors.AsType[dbus.Error](err)
	return ok && slices.Contains(names, de.Name)
}
