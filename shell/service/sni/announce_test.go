package sni

import (
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/dbustest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// announcedWatcher is a watcher on a host of its own over conn, whose
// reports it collects; its signal loop is stopped, so nothing reads
// what it tracks.
func announcedWatcher(t *testing.T, conn *dbus.Conn) (*watcher, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var reports []string
	h := idleHost(conn, NewStore(), time.Second, func(err error) {
		mu.Lock()
		defer mu.Unlock()
		reports = append(reports, err.Error())
	})
	h.cancel()
	t.Cleanup(func() { h.work.Wait() })
	w := newWatcher(h)
	w.ownerOf = func(string) (dbus.Sequence, error) { return 1, nil }
	return w, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(reports)
	}
}

// churn registers an item and a second host, then has both leave the
// bus with the watcher's own host, so every watcher signal is sent.
func churn(t *testing.T, w *watcher) {
	t.Helper()
	own := w.host.conn.Names()[0]
	if err := w.RegisterStatusNotifierHost(":1.77"); err != nil {
		t.Fatal(err)
	}
	w.register(":1.77/StatusNotifierItem")
	w.nameVanished(nameLoss{name: ":1.77"})
	w.nameVanished(nameLoss{name: own})
}

// A watcher signal the bus does not take is reported, naming the signal
// and the item it announced, never dropped.
func TestAWatcherSignalThatCannotBeSentIsReported(t *testing.T) {
	bus := dbustest.Start(t)
	conn := bus.Conn(t)
	w, reports := announcedWatcher(t, conn)
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	churn(t, w)
	got := reports()
	want := []string{
		"announce StatusNotifierHostRegistered: ",
		"announce StatusNotifierItemRegistered :1.77/StatusNotifierItem: ",
		"announce StatusNotifierItemUnregistered :1.77/StatusNotifierItem: ",
		"announce StatusNotifierHostUnregistered: ",
	}
	if len(got) != len(want) {
		t.Fatalf("reported %q, want one report per signal", got)
	}
	for i, prefix := range want {
		if !strings.HasPrefix(got[i], prefix) || len(got[i]) == len(prefix) {
			t.Errorf("report %d is %q, want %q and the emit's error", i, got[i], prefix)
		}
	}
}

// A watcher whose signals are sent reports nothing.
func TestAWatcherSignalThatIsSentReportsNothing(t *testing.T) {
	bus := dbustest.Start(t)
	w, reports := announcedWatcher(t, bus.Conn(t))
	churn(t, w)
	if got := reports(); len(got) != 0 {
		t.Fatalf("reported %q", got)
	}
}
