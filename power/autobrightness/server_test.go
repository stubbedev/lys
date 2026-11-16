package autobrightness_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/power/autobrightness"
)

func serve(t *testing.T, bus *dbustest.Bus, initial autobrightness.State, resume func(context.Context) error) *autobrightness.Server {
	t.Helper()
	s, release, err := autobrightness.Serve(bus.Conn(t), initial, resume)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	return s
}

func watch(t *testing.T, conn *dbus.Conn) <-chan *dbus.Signal {
	t.Helper()
	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(autobrightness.ServicePath),
		dbus.WithMatchInterface(dbusx.PropertiesInterface),
		dbus.WithMatchMember("PropertiesChanged"),
	); err != nil {
		t.Fatal(err)
	}
	ch := make(chan *dbus.Signal, 8)
	conn.Signal(ch)
	return ch
}

func next(t *testing.T, signals <-chan *dbus.Signal) autobrightness.State {
	t.Helper()
	for {
		state, ok, err := autobrightness.Changed(waittest.For(t, signals, "a State change"))
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			return state
		}
	}
}

func TestStateIsReadAndEveryChangeAnnounced(t *testing.T) {
	bus := dbustest.Start(t)
	s := serve(t, bus, autobrightness.Following, func(context.Context) error { return nil })
	conn := bus.Conn(t)
	client := autobrightness.NewClient(conn)
	signals := watch(t, conn)
	ctx := context.Background()

	if got, err := client.State(ctx); err != nil || got != autobrightness.Following {
		t.Fatalf("State = %v, %v, want following", got, err)
	}
	if err := s.Publish(autobrightness.Paused); err != nil {
		t.Fatal(err)
	}
	if got := next(t, signals); got != autobrightness.Paused {
		t.Errorf("announced %v, want paused", got)
	}
	if got, err := client.State(ctx); err != nil || got != autobrightness.Paused {
		t.Errorf("State = %v, %v, want paused", got, err)
	}
	if err := s.Publish(autobrightness.Paused); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(autobrightness.Following); err != nil {
		t.Fatal(err)
	}
	if got := next(t, signals); got != autobrightness.Following {
		t.Errorf("after a repeated paused, announced %v, want following: a state that did not change is not announced", got)
	}
}

func TestPublishRefusesAnInvalidState(t *testing.T) {
	bus := dbustest.Start(t)
	s := serve(t, bus, autobrightness.Following, func(context.Context) error { return nil })
	if err := s.Publish(autobrightness.State(0)); err == nil {
		t.Error("an invalid state published")
	}
	if _, _, err := autobrightness.Serve(bus.Conn(t), autobrightness.State(9), nil); err == nil {
		t.Error("served with an invalid state")
	}
}

func TestResumeReachesTheDaemonAndCarriesItsFailure(t *testing.T) {
	bus := dbustest.Start(t)
	fail := errors.New("no backlight to hand back")
	var mu sync.Mutex
	var calls int
	var answer error
	serve(t, bus, autobrightness.Paused, func(ctx context.Context) error {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("Resume runs without a deadline")
		}
		mu.Lock()
		defer mu.Unlock()
		calls++
		return answer
	})
	client := autobrightness.NewClient(bus.Conn(t))
	if err := client.Resume(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	answer = fail
	mu.Unlock()
	err := client.Resume(context.Background())
	var derr dbus.Error
	if !errors.As(err, &derr) || derr.Name != dbusx.ErrFailed {
		t.Errorf("a failing resume: %v, want %s", err, dbusx.ErrFailed)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Errorf("the daemon saw %d resumes, want 2", calls)
	}
}

func TestASecondServerFindsTheNameTaken(t *testing.T) {
	bus := dbustest.Start(t)
	serve(t, bus, autobrightness.Following, func(context.Context) error { return nil })
	if _, _, err := autobrightness.Serve(bus.Conn(t), autobrightness.Following, func(context.Context) error { return nil }); !errors.Is(err, dbusx.ErrNameTaken) {
		t.Errorf("a second server: %v, want ErrNameTaken", err)
	}
}
