package dbusxtest

import (
	"errors"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// RequireTakenNameRefused owns name on another connection of bus, then
// has serve export iface at path under name on a connection of its own:
// serve must fail with dbusx.ErrNameTaken and leave iface unexported.
func RequireTakenNameRefused(t testing.TB, bus *dbustest.Bus, name string, path dbus.ObjectPath, iface string, serve func(*dbus.Conn) error) {
	t.Helper()
	if _, err := bus.Conn(t).RequestName(name, dbus.NameFlagDoNotQueue); err != nil {
		t.Fatalf("take %s first: %v", name, err)
	}
	conn := bus.Conn(t)
	if err := serve(conn); !errors.Is(err, dbusx.ErrNameTaken) {
		t.Fatalf("serving under the taken %s = %v, want dbusx.ErrNameTaken", name, err)
	}
	var xml string
	err := bus.Conn(t).Object(conn.Names()[0], path).Call(dbusx.IntrospectableInterface+".Introspect", 0).Store(&xml)
	if err == nil && strings.Contains(xml, `"`+iface+`"`) {
		t.Fatalf("the refused service left %s exported at %s:\n%s", iface, path, xml)
	}
}
