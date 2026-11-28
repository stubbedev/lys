package portal

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/dbustest"
)

func TestALockdownChangeThatCannotBeAnnouncedIsLogged(t *testing.T) {
	cfg := config.Load(t.TempDir(), config.DiscardDiagnostics)
	conn := dbustest.Start(t).Conn(t)
	stop := lockdown{cfg}.watch(conn)
	t.Cleanup(stop)
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	if err := cfg.SetByPath("portal.lockdown.disable-camera", true); err != nil {
		t.Fatal(err)
	}
	if got := logged.String(); !strings.Contains(got, "lockdown: announce [disable-camera]") {
		t.Fatalf("logged %q, want the failed announcement", got)
	}
}
