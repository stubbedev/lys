// Package systemd1 names systemd's D-Bus API, org.freedesktop.systemd1,
// once for every lys process that asks a system or user manager about
// its units.
package systemd1

import dbus "github.com/stubbedev/lys/lib/third_party/godbus"

// The manager's well-known name, object and interfaces, and the error
// it answers for a unit it does not know.
const (
	BusName          = "org.freedesktop.systemd1"
	Path             = dbus.ObjectPath("/org/freedesktop/systemd1")
	ManagerInterface = BusName + ".Manager"
	UnitInterface    = BusName + ".Unit"
	ErrNoSuchUnit    = BusName + ".NoSuchUnit"
)

// Manager is the manager object on conn.
func Manager(conn *dbus.Conn) dbus.BusObject { return conn.Object(BusName, Path) }

// ListedUnit is one unit as ListUnits and ListUnitsByNames list it.
type ListedUnit struct {
	Name, Description, Load, Active, Sub, Following string
	Path                                            dbus.ObjectPath
	JobID                                           uint32
	JobType                                         string
	JobPath                                         dbus.ObjectPath
}

// IsActive reports whether an ActiveState is a unit that is up or on its
// way up: active, activating or reloading.
func IsActive(state string) bool {
	switch state {
	case "active", "activating", "reloading":
		return true
	}
	return false
}
