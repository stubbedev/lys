package nap_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/dbustest"

	"github.com/stubbedev/lys/power/nap"
)

type call struct {
	uid   uint32
	unit  nap.Unit
	slack time.Duration
}

type recorder struct {
	mu    sync.Mutex
	calls []call
	fail  error
}

func (r *recorder) SetTimerSlack(uid uint32, unit nap.Unit, slack time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call{uid, unit, slack})
	return r.fail
}

func (r *recorder) taken() []call {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]call(nil), r.calls...)
}

func TestTheCallersUserAsksForItsUnitsSlack(t *testing.T) {
	bus := dbustest.Start(t)
	r := &recorder{}
	release, err := nap.Serve(bus.Conn(t), r)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	client := nap.NewClient(bus.Conn(t))
	ctx := context.Background()
	if err := client.SetTimerSlack(ctx, "app-chrome-10.scope", nap.NappingSlack); err != nil {
		t.Fatal(err)
	}
	want := call{uint32(os.Getuid()), "app-chrome-10.scope", nap.NappingSlack}
	if got := r.taken(); len(got) != 1 || got[0] != want {
		t.Fatalf("calls %+v, want %+v", got, want)
	}
	for _, bad := range []struct {
		unit  nap.Unit
		slack time.Duration
	}{
		{"../user-0.slice", 0},
		{"app.slice/app-x.scope", 0},
		{"app-x", 0},
		{"app-x.scope", 2 * nap.MaxSlack},
	} {
		if err := client.SetTimerSlack(ctx, bad.unit, bad.slack); err == nil {
			t.Errorf("unit %q with slack %v was accepted", bad.unit, bad.slack)
		}
	}
	if got := len(r.taken()); got != 1 {
		t.Fatalf("%d calls reached the slacker, want only the valid one", got)
	}
	r.mu.Lock()
	r.fail = errors.New("no such unit")
	r.mu.Unlock()
	if err := client.SetTimerSlack(ctx, "app-gone-1.scope", 0); err == nil || !strings.Contains(err.Error(), "no such unit") {
		t.Fatalf("a failing write answered %v, want its error", err)
	}
}
