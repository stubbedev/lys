package dbustest

import (
	"fmt"
	"os"
	"testing"

	"github.com/stubbedev/lys/lib/testhome"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const (
	activatedEnv   = "LYS_DBUSTEST_ACTIVATED"
	activatedName  = "com.example.Activated"
	activatedPath  = dbus.ObjectPath("/test")
	activatedIface = "com.example.Args"
)

func TestMain(m *testing.M) {
	os.Exit(testhome.Main(func() int {
		if os.Getenv(activatedEnv) != "" {
			return serveArgs()
		}
		return m.Run()
	}))
}

type args struct{}

func (args) Args() ([]string, *dbus.Error) { return os.Args[1:], nil }

func serveArgs() int {
	conn, err := StarterConn()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = conn.Close() }()
	if err := conn.Export(args{}, activatedPath, activatedIface); err != nil {
		fmt.Fprintln(os.Stderr, "dbustest: export the activated service:", err)
		return 1
	}
	if reply, err := conn.RequestName(activatedName, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		fmt.Fprintf(os.Stderr, "dbustest: own %s: %v %v\n", activatedName, reply, err)
		return 1
	}
	<-conn.Context().Done()
	return 0
}

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }
