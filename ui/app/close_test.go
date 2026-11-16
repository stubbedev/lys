package app

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/ui/internal/wlsession"
)

func settleGoroutines(t *testing.T, atMost int, after string) {
	t.Helper()
	waittest.Until(t, func() bool { return runtime.NumGoroutine() <= atMost }, fmt.Sprintf("the goroutines to fall to %d after %s", atMost, after))
}

func awaitBusPeer(t *testing.T, signals <-chan *dbus.Signal, what string, match func(name, newOwner string) bool) string {
	t.Helper()
	for {
		sig := waittest.For(t, signals, what)
		if len(sig.Body) != 3 {
			t.Fatalf("NameOwnerChanged carried %v", sig.Body)
		}
		name, _ := sig.Body[0].(string)
		newOwner, _ := sig.Body[2].(string)
		if match(name, newOwner) {
			return name
		}
	}
}

func TestCloseStopsWhatAnApplicationThatNeverRanStarted(t *testing.T) {
	bus := dbustest.Session(t)
	watch := bus.Conn(t)
	if err := watch.AddMatchSignal(dbus.WithMatchInterface("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged")); err != nil {
		t.Fatal(err)
	}
	signals := make(chan *dbus.Signal, 16)
	watch.Signal(signals)
	entered, returned := holdFontWarm(t)
	baseline := runtime.NumGoroutine()

	a := NewTestApplication(t, &wlsession.Session{})
	stops := 0
	a.OnStop(func() { stops++ })
	<-entered
	peer := awaitBusPeer(t, signals, "the application's settings monitor to join the bus", func(_, newOwner string) bool { return newOwner != "" })

	a.Close()
	select {
	case <-returned:
	default:
		t.Error("Close returned with the font warm-up still scanning")
	}
	if stops != 1 {
		t.Errorf("Close ran the OnStop hook %d times, want once", stops)
	}
	awaitBusPeer(t, signals, "the settings monitor's connection to leave the bus", func(name, newOwner string) bool { return name == peer && newOwner == "" })
	ran := false
	a.Invoke(func() { ran = true })
	for _, fn := range a.queues.drain() {
		fn()
	}
	if ran {
		t.Error("a closed application queued an Invoke")
	}
	settleGoroutines(t, baseline, "Close")

	a.Close()
	if stops != 1 {
		t.Errorf("closing again ran the OnStop hook, %d times in all", stops)
	}
}
