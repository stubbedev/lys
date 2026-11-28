package screenshot

import (
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx/dbusxtest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

func TestADaemonRefusesATakenNameAndExportsNothing(t *testing.T) {
	dbusxtest.RequireTakenNameRefused(t, dbustest.Start(t), ServiceName, ServicePath, iface, func(conn *dbus.Conn) error {
		_, err := NewDaemon(&Host{}).Export(conn)
		return err
	})
}
