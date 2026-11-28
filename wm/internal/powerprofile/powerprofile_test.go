package powerprofile

import (
	"sync/atomic"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	lysprofile "github.com/stubbedev/lys/power/profile"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/wm/internal/loop"
	"github.com/stubbedev/lys/wm/internal/looptest"
)

type daemon struct {
	t       *testing.T
	conn    *dbus.Conn
	profile atomic.Value
	stop    func()
}

func runDaemon(t *testing.T, address string, profile string) *daemon {
	t.Helper()
	conn, err := dbus.Connect(address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	d := &daemon{t: t, conn: conn}
	d.profile.Store(profile)
	if d.stop, err = dbusx.Serve(conn, dbusx.Service{Name: lysprofile.ServiceName, Path: lysprofile.ServicePath, Interface: lysprofile.Interface, Methods: struct{}{}, Properties: dbusx.Getters{lysprofile.Property: func() any { return d.profile.Load().(string) }}}); err != nil {
		t.Fatal(err)
	}
	return d
}

func (d *daemon) set(profile string) {
	d.t.Helper()
	d.profile.Store(profile)
	d.emit(map[string]dbus.Variant{lysprofile.Property: dbus.MakeVariant(profile)})
}

func (d *daemon) emit(changed map[string]dbus.Variant, extra ...any) {
	d.t.Helper()
	if err := d.conn.Emit(lysprofile.ServicePath, dbusx.PropertiesInterface+".PropertiesChanged", append([]any{lysprofile.Interface, changed, []string{}}, extra...)...); err != nil {
		d.t.Fatal(err)
	}
}

type run struct {
	t      *testing.T
	l      *loop.Loop
	w      *Watcher
	saver  []bool
	errors []error
}

func (r *run) until(what string, done func() bool) {
	r.t.Helper()
	met := false
	defer func() {
		if !met {
			r.t.Logf("changes %v, errors %v", r.saver, r.errors)
		}
	}()
	looptest.RunUntil(r.t, r.l, what, done)
	met = true
}

func watch(t *testing.T, address string) *run {
	t.Helper()
	l, err := loop.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	r := &run{t: t, l: l}
	r.w, err = Watch(l, address, func(saver bool) error {
		r.saver = append(r.saver, saver)
		return nil
	}, func(err error) { r.errors = append(r.errors, err) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.w.Close(); err != nil {
			t.Error(err)
		}
	})
	return r
}

func (r *run) changes(n int) func() bool {
	return func() bool { return len(r.saver) >= n }
}

func (r *run) holds() bool { return r.w.last.owned }

func TestTheWatcherFollowsTheActiveProfile(t *testing.T) {
	bus := dbustest.Start(t)
	d := runDaemon(t, bus.Address, "power-saver")
	r := watch(t, bus.Address)
	r.until("lys-power's power-saver", r.changes(1))
	d.set("balanced")
	r.until("balanced", r.changes(2))
	d.set("balanced")
	d.set("power-saver")
	r.until("power-saver again", r.changes(3))
	d.stop()
	r.until("lys-power gone", r.changes(4))
	runDaemon(t, bus.Address, "power-saver")
	r.until("lys-power back", r.changes(5))
	if want := []bool{true, false, true, false, true}; len(r.saver) != len(want) || len(r.errors) != 0 {
		t.Fatalf("changes %v and errors %v, want %v and none", r.saver, r.errors, want)
	} else {
		for i := range want {
			if r.saver[i] != want[i] {
				t.Fatalf("changes %v, want %v", r.saver, want)
			}
		}
	}
}

func TestNoDaemonIsNoPowerSaver(t *testing.T) {
	bus := dbustest.Start(t)
	r := watch(t, bus.Address)
	stranger, err := dbus.Connect(bus.Address)
	if err != nil {
		t.Fatal(err)
	}
	defer stranger.Close()
	if err := stranger.Emit(lysprofile.ServicePath, dbusx.PropertiesInterface+".PropertiesChanged", "org.example.Other", map[string]dbus.Variant{lysprofile.Property: dbus.MakeVariant("power-saver")}, []string{}); err != nil {
		t.Fatal(err)
	}
	<-r.w.read
	d := runDaemon(t, bus.Address, "balanced")
	r.until("the daemon's balanced profile read", r.holds)
	d.set("power-saver")
	r.until("the daemon's power-saver", r.changes(1))
	if len(r.saver) != 1 || !r.saver[0] || len(r.errors) != 0 {
		t.Fatalf("changes %v and errors %v, want one change to power-saver: another interface's signal and a balanced daemon change nothing", r.saver, r.errors)
	}
}

func TestOnlyTheDaemonChangesTheProfile(t *testing.T) {
	bus := dbustest.Start(t)
	d := runDaemon(t, bus.Address, "balanced")
	r := watch(t, bus.Address)
	stranger, err := dbus.Connect(bus.Address)
	if err != nil {
		t.Fatal(err)
	}
	defer stranger.Close()
	if err := stranger.Emit(lysprofile.ServicePath, dbusx.PropertiesInterface+".PropertiesChanged", lysprofile.Interface, map[string]dbus.Variant{lysprofile.Property: dbus.MakeVariant("power-saver")}, []string{}); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := stranger.BusObject().Call("org.freedesktop.DBus.GetId", 0).Store(&id); err != nil {
		t.Fatal(err)
	}
	d.emit(map[string]dbus.Variant{lysprofile.Property: dbus.MakeVariant(int32(1))})
	d.emit(map[string]dbus.Variant{lysprofile.Property: dbus.MakeVariant("turbo")})
	r.until("the daemon's profiles that are no profile", func() bool { return len(r.errors) >= 2 })
	if len(r.saver) != 0 || len(r.errors) != 2 {
		t.Fatalf("changes %v and errors %v, want no change from another peer's PropertiesChanged in the daemon's name, and an error for each of the daemon's", r.saver, r.errors)
	}
}

func TestTheDescriptorsOfAPropertiesChangedAreClosed(t *testing.T) {
	bus := dbustest.Start(t)
	d := runDaemon(t, bus.Address, "balanced")
	r := watch(t, bus.Address)
	r.until("the daemon's balanced profile read", r.holds)
	var p [2]int
	if err := unix.Pipe2(p[:], unix.O_CLOEXEC); err != nil {
		t.Fatal(err)
	}
	read, write := p[0], p[1]
	defer unix.Close(read)
	fd := dbus.UnixFD(write)
	d.emit(map[string]dbus.Variant{"Handle": dbus.MakeVariant(fd)}, fd)
	d.profile.Store("power-saver")
	d.emit(map[string]dbus.Variant{lysprofile.Property: dbus.MakeVariant("power-saver"), "Handle": dbus.MakeVariant(fd)}, fd)
	r.until("power-saver", r.changes(1))
	if err := unix.Close(write); err != nil {
		t.Fatal(err)
	}
	fds := []unix.PollFd{{Fd: int32(read), Events: unix.POLLIN}}
	if n, err := unix.Poll(fds, 10_000); err != nil || n != 1 || fds[0].Revents&unix.POLLHUP == 0 {
		t.Fatalf("poll the pipe = %d, %v, events %#x; want a hangup, every descriptor the watcher was handed for its write end closed", n, err, fds[0].Revents)
	}
	if len(r.saver) != 1 || !r.saver[0] || len(r.errors) != 0 {
		t.Fatalf("changes %v and errors %v, want one change to power-saver", r.saver, r.errors)
	}
}

// Read answers the profile once: power-saver is power saver, any other
// profile and no lys-power at all are not.
func TestReadAsksTheProfileOnce(t *testing.T) {
	bus := dbustest.Start(t)
	if saver, err := Read(bus.Address); err != nil || saver {
		t.Fatalf("with no lys-power Read = %v, %v; want false and no error", saver, err)
	}
	d := runDaemon(t, bus.Address, "balanced")
	if saver, err := Read(bus.Address); err != nil || saver {
		t.Fatalf("under balanced Read = %v, %v; want false", saver, err)
	}
	d.set("power-saver")
	if saver, err := Read(bus.Address); err != nil || !saver {
		t.Fatalf("under power-saver Read = %v, %v; want true", saver, err)
	}
	if _, err := Read("unix:path=/nonexistent/bus"); err == nil {
		t.Fatal("Read of a bus that is not there returned no error")
	}
}
