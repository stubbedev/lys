package notifications

import (
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx/dbusxtest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

func TestServeRefusesATakenNameAndExportsNothing(t *testing.T) {
	svc := newTestService(t)
	dbusxtest.RequireTakenNameRefused(t, dbustest.Start(t), Interface, ObjPath, Interface, func(conn *dbus.Conn) error {
		_, err := Serve(conn, svc)
		return err
	})
}
