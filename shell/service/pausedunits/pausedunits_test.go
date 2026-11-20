package pausedunits_test

import (
	"context"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/testhome"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/power/pause"

	"github.com/stubbedev/lys/shell/internal/desktopnotify"
	"github.com/stubbedev/lys/shell/service/pausedunits"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

type handler struct {
	mu      sync.Mutex
	system  [][]string
	user    [][]string
	resumed int
	calls   chan struct{}
}

func (h *handler) record(do func()) error {
	h.mu.Lock()
	do()
	h.mu.Unlock()
	h.calls <- struct{}{}
	return nil
}

func names(units []pause.Unit) []string {
	out := []string{}
	for _, u := range units {
		out = append(out, u.Name())
	}
	return out
}

func (h *handler) PickSystem(_ context.Context, _ pause.Caller, units []pause.Unit) error {
	return h.record(func() { h.system = append(h.system, names(units)) })
}

func (h *handler) PickUser(_ context.Context, _ pause.Caller, units []pause.Unit) error {
	return h.record(func() { h.user = append(h.user, names(units)) })
}

func (h *handler) Resume(context.Context, pause.Caller) error {
	return h.record(func() { h.resumed++ })
}

type asked struct {
	summary, body string
	keys          []string
}

type notifier struct {
	answer string
	asks   chan asked
}

func (n *notifier) Ask(_ context.Context, _, summary, body, _ string, actions []desktopnotify.Action, _ time.Duration) (string, bool, error) {
	keys := make([]string, len(actions))
	for i, a := range actions {
		keys[i] = a.Key
	}
	n.asks <- asked{summary, body, keys}
	return n.answer, n.answer != "", nil
}

type rig struct {
	bus     *dbustest.Bus
	server  *pause.Server
	release func()
	h       *handler
	n       *notifier
	m       *pausedunits.Monitor
}

func me() uint32 { return intconv.Must[uint32](os.Getuid()) }

func start(t *testing.T, answer string, initial pause.State) *rig {
	t.Helper()
	r := &rig{bus: dbustest.Start(t), h: &handler{calls: make(chan struct{}, 8)}, n: &notifier{answer: answer, asks: make(chan asked, 8)}}
	r.serve(t, initial)
	m, err := pausedunits.Follow(r.bus.Conn(t), me(), func(err error) { t.Error(err) })
	if err != nil {
		t.Fatal(err)
	}
	r.m = m
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	})
	r.until(t, "the first state", func(_ pause.State, running bool) bool { return running })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		pausedunits.Announce(ctx, m, r.n)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return r
}

func (r *rig) serve(t *testing.T, st pause.State) {
	t.Helper()
	s, release, err := pause.Serve(r.bus.Conn(t), r.h)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(st); err != nil {
		t.Fatal(err)
	}
	r.server, r.release = s, release
	t.Cleanup(release)
}

func (r *rig) until(t *testing.T, what string, ok func(pause.State, bool) bool) {
	t.Helper()
	ticks, stop := r.m.Subscribe()
	defer stop()
	for !ok(r.m.Value()) {
		waittest.For(t, ticks, what)
	}
}

func (r *rig) publish(t *testing.T, st pause.State) {
	t.Helper()
	if err := r.server.Publish(st); err != nil {
		t.Fatal(err)
	}
	r.until(t, "the published state", func(got pause.State, _ bool) bool { return len(got.Entries) == len(st.VisibleTo(me()).Entries) })
}

func units(t *testing.T) (docker, timer, mine, theirs pause.Unit) {
	t.Helper()
	var errs [4]error
	docker, errs[0] = pause.System("docker.service")
	timer, errs[1] = pause.System("nix-gc.timer")
	mine, errs[2] = pause.User(me(), "syncthing.service")
	theirs, errs[3] = pause.User(me()+1, "mpd.service")
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	return docker, timer, mine, theirs
}

func paused(us ...pause.Unit) []pause.Entry {
	out := make([]pause.Entry, len(us))
	for i, u := range us {
		out[i] = pause.Entry{Unit: u, Status: pause.StatusPaused}
	}
	return out
}

func noAsk(t *testing.T, n *notifier) {
	t.Helper()
	select {
	case a := <-n.asks:
		t.Errorf("asked %+v", a)
	default:
	}
}

func TestANewlyPausedUnitIsToldOfAndResumedForTheSessionOnRequest(t *testing.T) {
	docker, timer, mine, theirs := units(t)
	picks := []pause.Unit{docker, timer, mine, theirs}
	r := start(t, pausedunits.ActionResume, pause.State{Picks: picks, Entries: paused(timer)})

	r.publish(t, pause.State{Picks: picks, Entries: paused(docker, timer, mine, theirs)})
	a := waittest.For(t, r.n.asks, "the notification")
	if !slices.Equal(a.keys, []string{pausedunits.ActionResume, pausedunits.ActionKeep}) {
		t.Errorf("actions %q", a.keys)
	}
	if want := i18n.T("battery-paused-body", i18n.Str("units", "docker.service, syncthing.service")); a.body != want || a.summary != i18n.T("battery-paused") {
		t.Errorf("the body %q names more or less than what was newly paused of the system's and yours", a.body)
	}
	waittest.For(t, r.h.calls, "the resume")
	r.h.mu.Lock()
	defer r.h.mu.Unlock()
	if r.h.resumed != 1 || len(r.h.system)+len(r.h.user) != 0 {
		t.Errorf("resumed %d, picks %q %q", r.h.resumed, r.h.system, r.h.user)
	}
}

func TestKeepRunningUnpicksJustTheNewlyPausedUnitsOfEachScope(t *testing.T) {
	docker, timer, mine, _ := units(t)
	picks := []pause.Unit{docker, timer, mine}
	r := start(t, pausedunits.ActionKeep, pause.State{Picks: picks, Entries: paused(timer)})
	r.publish(t, pause.State{Picks: picks, Entries: paused(docker, timer, mine)})
	waittest.For(t, r.n.asks, "the notification")
	waittest.For(t, r.h.calls, "the first pick")
	waittest.For(t, r.h.calls, "the second pick")
	r.h.mu.Lock()
	defer r.h.mu.Unlock()
	if !slices.EqualFunc(r.h.user, [][]string{{}}, slices.Equal) || !slices.EqualFunc(r.h.system, [][]string{{"nix-gc.timer"}}, slices.Equal) {
		t.Errorf("user picks %q, system picks %q", r.h.user, r.h.system)
	}
}

func TestNothingIsToldOfWhenNothingNewIsPausedOrLysPowerRestarts(t *testing.T) {
	docker, timer, mine, theirs := units(t)
	picks := []pause.Unit{docker, timer, mine, theirs}
	st := pause.State{Picks: picks, Entries: paused(docker, mine)}
	r := start(t, "", st)

	r.publish(t, pause.State{Picks: picks, Entries: append(paused(docker, mine), pause.Entry{Unit: timer, Status: pause.StatusPauseFailed, Err: "no"})})
	r.publish(t, pause.State{Picks: picks, Entries: paused(docker, mine, theirs)})
	r.release()
	r.until(t, "lys-power gone", func(_ pause.State, running bool) bool { return !running })
	r.serve(t, st)
	r.until(t, "lys-power back", func(_ pause.State, running bool) bool { return running })
	r.publish(t, pause.State{Picks: picks, Entries: paused(docker, mine, timer)})
	a := waittest.For(t, r.n.asks, "the one notification")
	if want := i18n.T("battery-paused-body", i18n.Str("units", "nix-gc.timer")); a.body != want {
		t.Errorf("the first notification reads %q, want only the timer paused last", a.body)
	}
	noAsk(t, r.n)
}

func TestRemainingIsThePicksOfAScopeWithoutTheUnits(t *testing.T) {
	docker, timer, mine, _ := units(t)
	st := pause.State{Picks: []pause.Unit{docker, timer, mine}}
	if got, ok := pausedunits.Remaining(st, []pause.Unit{docker}, pause.ScopeSystem); !ok || !slices.Equal(got, []string{"nix-gc.timer"}) {
		t.Errorf("system: %q %v", got, ok)
	}
	if got, ok := pausedunits.Remaining(st, []pause.Unit{docker}, pause.ScopeUser); ok {
		t.Errorf("no user unit was given, yet %q", got)
	}
	if got, ok := pausedunits.Remaining(st, []pause.Unit{mine}, pause.ScopeUser); !ok || got == nil || len(got) != 0 {
		t.Errorf("user: %#v %v, want an empty list that clears the picks", got, ok)
	}
}
