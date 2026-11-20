package attention_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stubbedev/lys/desktop/service/idle/idlepolicy"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/testhome"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/attention"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

func serve(t *testing.T, bus *dbustest.Bus, check attention.Check) {
	t.Helper()
	release, err := attention.Serve(bus.Conn(t), check)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
}

func TestEveryPresenceCrossesTheBus(t *testing.T) {
	bus := dbustest.Start(t)
	var mu sync.Mutex
	var answer idlepolicy.Presence
	serve(t, bus, func(ctx context.Context) (idlepolicy.Presence, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("the check runs without a deadline")
		}
		mu.Lock()
		defer mu.Unlock()
		return answer, nil
	})
	client := attention.NewClient(bus.Conn(t))
	for _, want := range []idlepolicy.Presence{idlepolicy.Present, idlepolicy.Absent, idlepolicy.Unseen} {
		mu.Lock()
		answer = want
		mu.Unlock()
		got, err := client.Check(t.Context())
		if err != nil || got != want {
			t.Errorf("Check = %v, %v, want %v", got, err, want)
		}
	}
}

func TestAFailedOrInvalidCheckIsAnErrorNotAPresence(t *testing.T) {
	bus := dbustest.Start(t)
	fail := errors.New("the camera vanished")
	var mu sync.Mutex
	var answer error
	serve(t, bus, func(context.Context) (idlepolicy.Presence, error) {
		mu.Lock()
		defer mu.Unlock()
		if answer != nil {
			return 0, answer
		}
		return idlepolicy.Presence(9), nil
	})
	client := attention.NewClient(bus.Conn(t))
	for _, a := range []error{fail, nil} {
		mu.Lock()
		answer = a
		mu.Unlock()
		_, err := client.Check(t.Context())
		var derr dbus.Error
		if !errors.As(err, &derr) || derr.Name != dbusx.ErrFailed {
			t.Errorf("answer %v: %v, want %s", a, err, dbusx.ErrFailed)
		}
	}
}

func TestACheckWithNoServiceFails(t *testing.T) {
	bus := dbustest.Start(t)
	if _, err := attention.NewClient(bus.Conn(t)).Check(t.Context()); err == nil {
		t.Error("a check without the service answered")
	}
}

func TestASecondServerFindsTheNameTaken(t *testing.T) {
	bus := dbustest.Start(t)
	check := func(context.Context) (idlepolicy.Presence, error) { return idlepolicy.Unseen, nil }
	serve(t, bus, check)
	if _, err := attention.Serve(bus.Conn(t), check); !errors.Is(err, dbusx.ErrNameTaken) {
		t.Errorf("a second server: %v, want ErrNameTaken", err)
	}
}
