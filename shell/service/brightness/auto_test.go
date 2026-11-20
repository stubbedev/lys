package brightness

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/power/autobrightness"
)

type fakeAutoDaemon struct {
	conn    *dbus.Conn
	state   dbustest.Var[any]
	room    dbustest.Var[string]
	resumes dbustest.Var[int]
	release func()
}

func (f *fakeAutoDaemon) Resume() *dbus.Error {
	f.resumes.Update(func(n int) int { return n + 1 })
	return nil
}

func serveAutoDaemon(t *testing.T, bus *dbustest.Bus, state any) *fakeAutoDaemon {
	t.Helper()
	f := &fakeAutoDaemon{conn: bus.Conn(t)}
	f.state.Store(state)
	f.room.Store(autobrightness.RoomUnknown.String())
	release, err := dbusx.Serve(f.conn, dbusx.Service{
		Name:      autobrightness.ServiceName,
		Path:      autobrightness.ServicePath,
		Interface: autobrightness.Interface,
		Methods:   f,
		Properties: dbusx.Getters{
			autobrightness.Property:     func() any { return f.state.Load() },
			autobrightness.RoomProperty: func() any { return f.room.Load() },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	f.release = release
	t.Cleanup(release)
	return f
}

func (f *fakeAutoDaemon) announce(t *testing.T, state any) {
	t.Helper()
	f.state.Store(state)
	err := f.conn.Emit(autobrightness.ServicePath, dbusx.PropertiesInterface+".PropertiesChanged",
		autobrightness.Interface, map[string]dbus.Variant{autobrightness.Property: dbus.MakeVariant(state)}, []string{})
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fakeAutoDaemon) announceRoom(t *testing.T, room autobrightness.Room) {
	t.Helper()
	f.room.Store(room.String())
	err := f.conn.Emit(autobrightness.ServicePath, dbusx.PropertiesInterface+".PropertiesChanged",
		autobrightness.Interface, map[string]dbus.Variant{autobrightness.RoomProperty: dbus.MakeVariant(room.String())}, []string{})
	if err != nil {
		t.Fatal(err)
	}
}

func (f autoFollower) awaitRoom(t *testing.T, want autobrightness.Room) {
	t.Helper()
	for f.Room() != want {
		waittest.For(t, f.ticks, fmt.Sprintf("the room to change from %v to %v", f.Room(), want))
	}
}

func TestAutoFollowsTheRoomAndForgetsItWhenAutomaticBrightnessLeaves(t *testing.T) {
	bus := dbustest.Start(t)
	f := followAuto(t, bus.Conn(t))
	if got := f.Room(); got != autobrightness.RoomUnknown {
		t.Fatalf("no automatic brightness: room %v, want unknown", got)
	}
	daemon := serveAutoDaemon(t, bus, "following")
	f.await(t, ControlFollowing)
	daemon.announceRoom(t, autobrightness.RoomBright)
	f.awaitRoom(t, autobrightness.RoomBright)
	if got := f.Control(); got != ControlFollowing {
		t.Errorf("a room change moved the control to %v", got)
	}
	daemon.announceRoom(t, autobrightness.RoomDim)
	f.awaitRoom(t, autobrightness.RoomDim)
	daemon.announceRoom(t, autobrightness.RoomBright)
	f.awaitRoom(t, autobrightness.RoomBright)
	daemon.release()
	f.awaitRoom(t, autobrightness.RoomUnknown)
}

type autoFollower struct {
	*Auto
	ticks  <-chan struct{}
	failed chan error
}

func followAuto(t *testing.T, conn *dbus.Conn) autoFollower {
	t.Helper()
	failed := make(chan error, 16)
	a, err := FollowAuto(conn, func(err error) { failed <- err })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Error(err)
		}
	})
	ticks, stop := a.Subscribe()
	t.Cleanup(stop)
	return autoFollower{Auto: a, ticks: ticks, failed: failed}
}

func (f autoFollower) await(t *testing.T, want Control) {
	t.Helper()
	for f.Control() != want {
		select {
		case err := <-f.failed:
			t.Fatalf("waiting for %v: %v", want, err)
		default:
		}
		waittest.For(t, f.ticks, fmt.Sprintf("the control to change from %v to %v", f.Control(), want))
	}
}

func (f autoFollower) awaitFailure(t *testing.T, want string) {
	t.Helper()
	if err := waittest.For(t, f.failed, fmt.Sprintf("a failure naming %q", want)); !strings.Contains(err.Error(), want) {
		t.Errorf("reported %q, want it to name %q", err, want)
	}
}

func TestAutoFollowsTheNameAndItsState(t *testing.T) {
	bus := dbustest.Start(t)
	f := followAuto(t, bus.Conn(t))
	if got := f.Control(); got != ControlManual {
		t.Fatalf("no automatic brightness: control %v, want manual", got)
	}

	daemon := serveAutoDaemon(t, bus, "following")
	f.await(t, ControlFollowing)

	daemon.announce(t, "paused")
	f.await(t, ControlPaused)

	daemon.announce(t, "following")
	f.await(t, ControlFollowing)

	daemon.release()
	f.await(t, ControlManual)

	serveAutoDaemon(t, bus, "paused")
	f.await(t, ControlPaused)
}

func TestAutoReadsAStateAlreadyOnTheBus(t *testing.T) {
	bus := dbustest.Start(t)
	serveAutoDaemon(t, bus, "paused")
	if got := followAuto(t, bus.Conn(t)).Control(); got != ControlPaused {
		t.Errorf("control %v, want paused at once", got)
	}
}

func TestAutoReportsAMalformedStateAndKeepsTheLastOne(t *testing.T) {
	bus := dbustest.Start(t)
	daemon := serveAutoDaemon(t, bus, "following")
	f := followAuto(t, bus.Conn(t))
	f.await(t, ControlFollowing)

	daemon.announce(t, "dimming")
	f.awaitFailure(t, `"dimming"`)
	daemon.announce(t, uint32(2))
	f.awaitFailure(t, "uint32")
	if got := f.Control(); got != ControlFollowing {
		t.Errorf("after malformed states: control %v, want following kept", got)
	}

	daemon.release()
	f.await(t, ControlManual)
	serveAutoDaemon(t, bus, "dimming")
	f.awaitFailure(t, `"dimming"`)
	if got := f.Control(); got != ControlManual {
		t.Errorf("a daemon answering a malformed state: control %v, want manual kept", got)
	}
}

func TestAutoMalformedStateAtStartIsReported(t *testing.T) {
	bus := dbustest.Start(t)
	serveAutoDaemon(t, bus, "dimming")
	f := followAuto(t, bus.Conn(t))
	f.awaitFailure(t, `"dimming"`)
	if got := f.Control(); got != ControlManual {
		t.Errorf("control %v, want manual", got)
	}
}

func TestAutoCloseLeavesTheConnectionAsItFoundIt(t *testing.T) {
	bus := dbustest.Start(t)
	conn := bus.Conn(t)
	before := dbustest.MatchRules(t, conn)
	a, err := FollowAuto(conn, func(err error) { t.Error(err) })
	if err != nil {
		t.Fatal(err)
	}
	if during := dbustest.MatchRules(t, conn); len(during) <= len(before) {
		t.Fatalf("following added no match rule: %v", during)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if after := dbustest.MatchRules(t, conn); !slices.Equal(after, before) {
		t.Errorf("match rules after Close = %v, want %v", after, before)
	}
}

func TestResumeAutoHandsTheBacklightBack(t *testing.T) {
	bus := dbustest.Start(t)
	conn := bus.Conn(t)
	err := ResumeAuto(t.Context(), conn)
	if !errors.Is(err, ErrAutoNotRunning) {
		t.Fatalf("without automatic brightness: %v, want ErrAutoNotRunning", err)
	}
	daemon := serveAutoDaemon(t, bus, "paused")
	if err := ResumeAuto(t.Context(), conn); err != nil {
		t.Fatal(err)
	}
	if got := daemon.resumes.Load(); got != 1 {
		t.Errorf("Resume reached the daemon %d times, want once", got)
	}
	f := followAuto(t, conn)
	if err := f.Resume(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := daemon.resumes.Load(); got != 2 {
		t.Errorf("Auto.Resume reached the daemon %d times in all, want twice", got)
	}
}

func TestControlOfRejectsAnUnknownState(t *testing.T) {
	for state, want := range map[autobrightness.State]Control{autobrightness.Following: ControlFollowing, autobrightness.Paused: ControlPaused} {
		if got, err := controlOf(state); err != nil || got != want {
			t.Errorf("controlOf(%v) = %v, %v; want %v", state, got, err, want)
		}
	}
	if _, err := controlOf(0); err == nil || !strings.Contains(err.Error(), "State(0)") {
		t.Errorf("controlOf(0) = %v, want an error naming State(0)", err)
	}
}
