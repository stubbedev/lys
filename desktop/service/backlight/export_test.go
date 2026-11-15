package backlight

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/logind"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/desktop/uevent"
)

func Inject(t testing.TB, listen func() (uevent.Reader, error), drift time.Duration) {
	t.Helper()
	origListen, origDrift := listenUevents, driftInterval
	listenUevents, driftInterval = listen, drift
	t.Cleanup(func() { listenUevents, driftInterval = origListen, origDrift })
}

func LogindWriter(conn *dbus.Conn) func(ctx context.Context, name string, value uint32) error {
	b := &logindBacklight{manager: logind.NewManager(conn)}
	return b.set
}

type Probe struct{ levelProbe }

func OpenProbe(entries []os.DirEntry) *Probe {
	p := &Probe{}
	p.open(entries)
	return p
}

func (p *Probe) Moved() (bool, error) { return p.moved() }
func (p *Probe) Close()               { p.close() }
func (p *Probe) CloseFirstFile()      { _ = p.levels[0].file.Close() }
