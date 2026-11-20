package autobright

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/service/backlight"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/power/autobrightness"
	"github.com/stubbedev/lys/power/internal/ambient"
)

const (
	interval = time.Minute
	stepGap  = 100 * time.Millisecond
	steps    = 4
	maxRaw   = 1000
)

type clock struct {
	samples chan chan time.Time
	steps   chan chan time.Time
	now     time.Time
}

func newClock() *clock {
	return &clock{
		samples: make(chan chan time.Time, 4),
		steps:   make(chan chan time.Time, 4),
		now:     time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC),
	}
}

func (c *clock) Now() time.Time { return c.now }

func (c *clock) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	switch d {
	case 0:
		ch <- c.now
	case interval:
		c.samples <- ch
	case stepGap:
		c.steps <- ch
	default:
		panic(fmt.Sprintf("a timer of %v", d))
	}
	return ch
}

type reading struct {
	lux ambient.Lux
	err error
}

type light struct{ readings chan reading }

func (l *light) Sample(ctx context.Context) (ambient.Lux, error) {
	select {
	case r := <-l.readings:
		return r.lux, r.err
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func deliver[T any](t *testing.T, ch chan<- T, v T, what string) {
	t.Helper()
	ctx := t.Context()
	sent := make(chan struct{})
	go func() {
		select {
		case ch <- v:
			close(sent)
		case <-ctx.Done():
		}
	}()
	waittest.For(t, sent, what)
}

type fakeBacklight struct {
	mu     sync.Mutex
	levels map[string]uint32
	unread []float64
	ticks  chan struct{}
	writes chan float64
}

func newBacklight(names ...string) *fakeBacklight {
	b := &fakeBacklight{levels: map[string]uint32{}, ticks: make(chan struct{}), writes: make(chan float64, 64)}
	for _, name := range names {
		b.levels[name] = 400
	}
	return b
}

func (b *fakeBacklight) Devices(context.Context) ([]backlight.Device, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []backlight.Device
	for _, name := range []string{"acpi_video0", "intel_backlight"} {
		if raw, ok := b.levels[name]; ok {
			out = append(out, backlight.Device{Name: name, Brightness: raw, Max: maxRaw})
		}
	}
	for _, percent := range b.unread {
		b.writes <- percent
	}
	b.unread = nil
	return out, nil
}

func (b *fakeBacklight) Set(_ context.Context, name string, percent float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.levels[name] = uint32(math.Round(percent / 100 * maxRaw))
	b.unread = append(b.unread, percent)
	return nil
}

func (b *fakeBacklight) Subscribe(context.Context) (<-chan struct{}, func(), error) {
	return b.ticks, func() {}, nil
}

func (b *fakeBacklight) setAll(raw uint32) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for name := range b.levels {
		b.levels[name] = raw
	}
}

func (b *fakeBacklight) level(name string) uint32 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.levels[name]
}

type session struct {
	mu                   sync.Mutex
	path                 dbus.ObjectPath
	active, locked, idle bool
	idleErr              error
}

func (s *session) Path() dbus.ObjectPath { return s.path }

func (s *session) Active(context.Context) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active, nil
}

func (s *session) LockedHint(context.Context) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.locked, nil
}

func (s *session) IdleHint(context.Context) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.idle, s.idleErr
}

func (s *session) setIdle(idle bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.idle = idle
}

func (s *session) failIdleHint(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.idleErr = err
}

func (s *session) set(active, locked bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active, s.locked = active, locked
}

type sessions struct {
	mu      sync.Mutex
	display Session
	err     error
}

func (s *sessions) Display(context.Context) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.display, s.err
}

func (s *sessions) set(display Session, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.display, s.err = display, err
}

type profile struct{ saver chan bool }

func newProfile() *profile { return &profile{saver: make(chan bool)} }

func (p *profile) Saver(context.Context) (<-chan bool, error) { return p.saver, nil }

type publisher struct {
	states chan autobrightness.State
	mu     sync.Mutex
	rooms  []autobrightness.Room
}

func (p *publisher) Publish(s autobrightness.State) error {
	p.states <- s
	return nil
}

func (p *publisher) PublishRoom(r autobrightness.Room) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rooms = append(p.rooms, r)
	return nil
}

func (p *publisher) roomsSeen() []autobrightness.Room {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.rooms)
}

type harness struct {
	t        *testing.T
	clock    *clock
	light    *light
	back     *fakeBacklight
	session  *session
	sessions *sessions
	profile  *profile
	pub      *publisher
	daemon   *Daemon
	curve    ambient.Curve
	done     chan error
	cancel   context.CancelFunc
}

func start(t *testing.T, names ...string) *harness {
	t.Helper()
	curve, err := ambient.NewCurve(1, 10000, 5, 100)
	if err != nil {
		t.Fatal(err)
	}
	smoother, err := ambient.NewSmoother(30*time.Second, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		names = []string{"intel_backlight"}
	}
	h := &harness{
		t:       t,
		clock:   newClock(),
		light:   &light{readings: make(chan reading)},
		back:    newBacklight(names...),
		session: &session{path: "/org/freedesktop/login1/session/_32", active: true},
		profile: newProfile(),
		pub:     &publisher{states: make(chan autobrightness.State, 8)},
		curve:   curve,
		done:    make(chan error, 1),
	}
	h.sessions = &sessions{display: h.session}
	dim, err := ambient.NewDim(0.5)
	if err != nil {
		t.Fatal(err)
	}
	room, err := ambient.NewRoomBand(250, 500)
	if err != nil {
		t.Fatal(err)
	}
	h.daemon = New(h.light, h.back, h.sessions, h.profile, h.clock, slog.New(slog.NewTextHandler(io.Discard, nil)), Params{
		Curve: curve, LowPower: dim, Room: room, Smoother: smoother, Band: 6, Steps: steps, Step: stepGap, Interval: interval,
	})
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go func() { h.done <- h.daemon.Run(ctx, h.pub) }()
	t.Cleanup(func() {
		cancel()
		if err := waittest.For(t, h.done, "the daemon to stop"); err != nil {
			t.Errorf("the daemon stopped with %v", err)
		}
	})
	return h
}

func (h *harness) read(lux ambient.Lux) {
	h.t.Helper()
	deliver(h.t, h.light.readings, reading{lux: lux}, "the daemon to take a sample")
}

func (h *harness) readErr(err error) {
	h.t.Helper()
	deliver(h.t, h.light.readings, reading{err: err}, "the daemon to take a sample")
}

func (h *harness) armed() chan time.Time {
	h.t.Helper()
	return waittest.For(h.t, h.clock.samples, "the daemon to arm the next sample")
}

func (h *harness) tick(next chan time.Time) chan time.Time {
	h.t.Helper()
	h.clock.now = h.clock.now.Add(interval)
	next <- h.clock.now
	return h.armed()
}

func (h *harness) sampleWith(next chan time.Time, lux ambient.Lux) chan time.Time {
	h.t.Helper()
	h.clock.now = h.clock.now.Add(interval)
	next <- h.clock.now
	h.read(lux)
	return h.armed()
}

func (h *harness) stepping() bool {
	select {
	case ch := <-h.clock.steps:
		h.clock.steps <- ch
		return true
	default:
		return false
	}
}

func (h *harness) transition(devices int) []float64 {
	h.t.Helper()
	var levels []float64
	for range steps {
		waittest.For(h.t, h.clock.steps, "the next step of the transition") <- h.clock.now
		for range devices {
			levels = append(levels, waittest.For(h.t, h.back.writes, "a step's write to be read back"))
		}
	}
	return levels
}

func (h *harness) changed() {
	h.t.Helper()
	for range 2 {
		deliver(h.t, h.back.ticks, struct{}{}, "the daemon to watch the backlight")
	}
}

func (h *harness) published(want autobrightness.State) {
	h.t.Helper()
	if got := waittest.For(h.t, h.pub.states, "the daemon to publish "+want.String()); got != want {
		h.t.Fatalf("published %v, want %v", got, want)
	}
}

func (h *harness) quiet() {
	h.t.Helper()
	select {
	case got := <-h.pub.states:
		h.t.Fatalf("published %v, want nothing", got)
	default:
	}
	select {
	case p := <-h.back.writes:
		h.t.Fatalf("wrote %v%%, want nothing", p)
	default:
	}
	if h.stepping() {
		h.t.Fatal("a transition started")
	}
}

func (h *harness) resume() {
	h.t.Helper()
	done := make(chan error, 1)
	go func() { done <- h.daemon.Resume(h.t.Context()) }()
	if err := waittest.For(h.t, done, "Resume to return"); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) follow(lux ambient.Lux) chan time.Time {
	h.t.Helper()
	h.read(lux)
	next := h.armed()
	levels := h.transition(1)
	want := h.curve.Percent(lux)
	if math.Abs(levels[len(levels)-1]-want) > 1e-9 {
		h.t.Fatalf("the transition ended at %v%%, want %v%%", levels[len(levels)-1], want)
	}
	return next
}

func TestTheBacklightFollowsTheRoomInSteps(t *testing.T) {
	h := start(t)
	h.read(1000)
	h.armed()
	levels := h.transition(1)
	want := h.curve.Percent(1000)
	for i := 1; i < len(levels); i++ {
		if levels[i] <= levels[i-1] {
			t.Fatalf("the steps %v do not rise", levels)
		}
	}
	if levels[0] <= 40 || levels[0] >= want || math.Abs(levels[len(levels)-1]-want) > 1e-9 {
		t.Fatalf("the steps %v do not go from 40%% to %v%%", levels, want)
	}
	if h.stepping() {
		t.Fatal("the transition went on past its target")
	}
	h.quiet()
}

func TestTheBacklightHoldsInsideTheBand(t *testing.T) {
	h := start(t)
	next := h.follow(100)
	h.sampleWith(next, 130)
	h.quiet()
}

func TestTheBacklightMovesWhenTheRoomChangesBeyondTheBand(t *testing.T) {
	h := start(t)
	next := h.follow(100)
	h.clock.now = h.clock.now.Add(time.Hour)
	h.sampleWith(next, 10000)
	levels := h.transition(1)
	if math.Abs(levels[len(levels)-1]-100) > 1e-9 {
		t.Fatalf("the transition ended at %v, want 100%%", levels)
	}
}

func TestAManualChangePausesUntilResume(t *testing.T) {
	h := start(t)
	next := h.follow(100)
	h.back.setAll(900)
	h.changed()
	h.published(autobrightness.Paused)

	h.tick(next)
	h.quiet()
	if got := h.back.level("intel_backlight"); got != 900 {
		t.Fatalf("a paused daemon moved the backlight to %d", got)
	}

	h.resume()
	h.published(autobrightness.Following)
	if !h.stepping() {
		t.Fatal("resume did not apply the current target at once")
	}
	h.read(100)
	h.armed()
	levels := h.transition(1)
	if want := h.curve.Percent(100); levels[0] >= 90 || math.Abs(levels[len(levels)-1]-want) > 1e-9 {
		t.Fatalf("resume went from 90%% along %v, want the current target %v%%", levels, want)
	}
	h.quiet()
}

func TestAnIdleDimAndItsRestoreKeepFollowing(t *testing.T) {
	h := start(t)
	next := h.follow(100)
	ours := h.back.level("intel_backlight")
	h.session.setIdle(true)
	h.back.setAll(ours * 30 / 100)
	h.changed()
	h.published(autobrightness.Paused)
	h.back.setAll(ours)
	h.changed()
	h.published(autobrightness.Following)
	h.session.setIdle(false)

	h.clock.now = h.clock.now.Add(time.Hour)
	h.sampleWith(next, 10000)
	levels := h.transition(1)
	if math.Abs(levels[len(levels)-1]-100) > 1e-9 {
		t.Fatalf("after the restore the daemon went to %v, want 100%%", levels)
	}
}

func TestADimThatMovesAgainHoldsEvenBackAtOurLevel(t *testing.T) {
	h := start(t)
	next := h.follow(100)
	ours := h.back.level("intel_backlight")
	h.session.setIdle(true)
	h.back.setAll(ours / 2)
	h.changed()
	h.published(autobrightness.Paused)
	h.back.setAll(ours / 3)
	h.changed()
	h.back.setAll(ours)
	h.changed()
	h.quiet()
	h.tick(next)
	h.quiet()
}

func TestAManualLowerAndRaiseBackToOurLevelHoldsThePause(t *testing.T) {
	h := start(t)
	next := h.follow(100)
	ours := h.back.level("intel_backlight")
	h.back.setAll(ours - 50)
	h.changed()
	h.published(autobrightness.Paused)
	h.back.setAll(ours)
	h.changed()
	h.quiet()
	h.tick(next)
	h.quiet()
}

func TestAnUnreadableIdleHintStopsTheDaemon(t *testing.T) {
	h := start(t)
	h.follow(100)
	fail := errors.New("org.freedesktop.DBus.Error.NoReply")
	h.session.failIdleHint(fail)
	h.back.setAll(900)
	h.back.ticks <- struct{}{}
	err := h.stopped()
	if !errors.Is(err, fail) || !strings.Contains(err.Error(), "read whether the session is idle") {
		t.Fatalf("the daemon stopped with %v, want %v saying what it read", err, fail)
	}
	h.quiet()
}

func TestNothingIsSampledOrWrittenInTheBackgroundOrLocked(t *testing.T) {
	for name, s := range map[string]struct{ active, locked bool }{
		"inactive": {false, false},
		"locked":   {true, true},
	} {
		t.Run(name, func(t *testing.T) {
			h := start(t)
			next := h.follow(100)
			h.session.set(s.active, s.locked)
			next = h.tick(next)
			h.back.setAll(900)
			h.changed()
			h.quiet()

			h.session.set(true, false)
			h.tick(next)
			h.published(autobrightness.Paused)
			h.quiet()
		})
	}
}

func TestABusyCameraSkipsTheSampleAndKeepsTheLevel(t *testing.T) {
	h := start(t)
	next := h.follow(100)
	h.clock.now = h.clock.now.Add(time.Hour)
	next <- h.clock.now
	h.readErr(fmt.Errorf("%w: /dev/video0 is in use", ambient.ErrNoSample))
	h.armed()
	h.quiet()
}

func TestAFailingLightStopsTheDaemon(t *testing.T) {
	h := start(t)
	fail := errors.New("the sensor went away")
	h.readErr(fail)
	if err := h.stopped(); !errors.Is(err, fail) {
		t.Errorf("the daemon stopped with %v, want %v", err, fail)
	}
}

func TestEveryBacklightIsDriven(t *testing.T) {
	h := start(t, "acpi_video0", "intel_backlight")
	h.read(1000)
	h.armed()
	levels := h.transition(2)
	want := h.curve.Percent(1000)
	if math.Abs(levels[len(levels)-1]-want) > 1e-9 || math.Abs(levels[len(levels)-2]-want) > 1e-9 {
		t.Fatalf("the backlights ended at %v, want both at %v%%", levels, want)
	}
}

func TestNoBacklightIsAnError(t *testing.T) {
	d := New(&light{}, newBacklight(), &sessions{display: &session{active: true}}, newProfile(), newClock(), slog.New(slog.NewTextHandler(io.Discard, nil)), Params{Steps: steps})
	if err := d.Run(context.Background(), &publisher{}); err == nil {
		t.Fatal("the daemon ran without a backlight")
	}
}

func (h *harness) stopped() error {
	h.t.Helper()
	err := waittest.For(h.t, h.done, "the daemon to stop")
	h.done <- nil
	return err
}

func TestTheSameDisplaySessionKeepsFollowing(t *testing.T) {
	h := start(t)
	next := h.follow(100)
	h.sessions.set(&session{path: h.session.path, active: true}, nil)
	h.clock.now = h.clock.now.Add(time.Hour)
	h.sampleWith(next, 10000)
	levels := h.transition(1)
	if math.Abs(levels[len(levels)-1]-100) > 1e-9 {
		t.Fatalf("the daemon went to %v, want 100%%", levels)
	}
}

func TestAnotherDisplaySessionStopsTheDaemon(t *testing.T) {
	h := start(t)
	next := h.follow(100)
	h.sessions.set(&session{path: "/org/freedesktop/login1/session/_33", active: true}, nil)
	h.clock.now = h.clock.now.Add(interval)
	next <- h.clock.now
	err := h.stopped()
	if !errors.Is(err, ErrSessionChanged) || !strings.Contains(err.Error(), "_33") {
		t.Fatalf("the daemon stopped with %v, want %v naming the new session", err, ErrSessionChanged)
	}
	h.quiet()
}

func TestAGoneDisplaySessionStopsTheDaemon(t *testing.T) {
	h := start(t)
	h.follow(100)
	gone := errors.New("org.freedesktop.login1.NoSuchSession")
	h.sessions.set(nil, gone)
	h.back.setAll(900)
	h.back.ticks <- struct{}{}
	err := h.stopped()
	if !errors.Is(err, ErrSessionChanged) || !errors.Is(err, gone) {
		t.Fatalf("the daemon stopped with %v, want %v wrapping %v", err, ErrSessionChanged, gone)
	}
	h.quiet()
}

func TestNoDisplaySessionAtStartIsAnError(t *testing.T) {
	gone := errors.New("org.freedesktop.login1.NoSuchSession")
	d := New(&light{}, newBacklight("intel_backlight"), &sessions{err: gone}, newProfile(), newClock(), slog.New(slog.NewTextHandler(io.Discard, nil)), Params{Steps: steps})
	if err := d.Run(context.Background(), &publisher{}); !errors.Is(err, gone) {
		t.Fatalf("the daemon ran with %v, want %v", err, gone)
	}
}

func (h *harness) powerSaver(on bool) {
	h.t.Helper()
	deliver(h.t, h.profile.saver, on, "the daemon to take the power profile")
}

func TestPowerSaverDimsTheCurveAndItsEndRestoresIt(t *testing.T) {
	h := start(t)
	h.follow(1000)
	full := h.curve.Percent(1000)
	h.powerSaver(true)
	if levels := h.transition(1); math.Abs(levels[len(levels)-1]-full/2) > 1e-9 {
		t.Fatalf("under power saver the backlight went along %v, want to %v%%", levels, full/2)
	}
	h.quiet()
	h.powerSaver(true)
	h.quiet()
	h.powerSaver(false)
	if levels := h.transition(1); math.Abs(levels[len(levels)-1]-full) > 1e-9 {
		t.Fatalf("after power saver the backlight went along %v, want back to %v%%", levels, full)
	}
	h.quiet()
}

func TestPowerSaverLeavesAPausedBacklightAlone(t *testing.T) {
	h := start(t)
	h.follow(1000)
	h.back.setAll(900)
	h.changed()
	h.published(autobrightness.Paused)
	h.powerSaver(true)
	h.quiet()
	if got := h.back.level("intel_backlight"); got != 900 {
		t.Fatalf("power saver moved a paused backlight to %d", got)
	}
}

func TestPowerSaverDimmingDoesNotPauseAutomaticBrightness(t *testing.T) {
	h := start(t)
	next := h.follow(1000)
	h.powerSaver(true)
	h.transition(1)
	h.changed()
	h.sampleWith(next, 1000)
	h.quiet()
}

func TestAnEndedPowerProfileWatchStopsTheDaemon(t *testing.T) {
	h := start(t)
	h.follow(1000)
	close(h.profile.saver)
	err := waittest.For(t, h.done, "the daemon to stop")
	h.done <- nil
	if err == nil || !strings.Contains(err.Error(), "power profile") {
		t.Fatalf("the daemon stopped with %v, want the ended power profile watch", err)
	}
}

func TestPowerSaverStartedDuringAnIdleDimDimsOnceItIsRestored(t *testing.T) {
	h := start(t)
	h.follow(1000)
	full := h.curve.Percent(1000)
	ours := h.back.level("intel_backlight")
	h.session.setIdle(true)
	h.back.setAll(ours * 30 / 100)
	h.changed()
	h.published(autobrightness.Paused)
	h.powerSaver(true)
	h.quiet()
	h.back.setAll(ours)
	h.changed()
	h.published(autobrightness.Following)
	h.session.setIdle(false)
	if levels := h.transition(1); math.Abs(levels[len(levels)-1]-full/2) > 1e-9 {
		t.Fatalf("after the restore the backlight went along %v, want to the dimmed %v%%", levels, full/2)
	}
	h.quiet()
}

func TestARestoreWithTheSameProfileStartsNoTransition(t *testing.T) {
	h := start(t)
	h.follow(1000)
	ours := h.back.level("intel_backlight")
	h.session.setIdle(true)
	h.back.setAll(ours * 30 / 100)
	h.changed()
	h.published(autobrightness.Paused)
	h.back.setAll(ours)
	h.changed()
	h.published(autobrightness.Following)
	h.quiet()
}

func TestTheRoomIsAnnouncedFromTheEstimateAndUnknownWhilePaused(t *testing.T) {
	h := start(t)
	next := h.follow(1000)
	h.clock.now = h.clock.now.Add(time.Hour)
	next = h.sampleWith(next, 10)
	h.transition(1)
	h.back.setAll(900)
	h.changed()
	h.published(autobrightness.Paused)
	want := []autobrightness.Room{autobrightness.RoomBright, autobrightness.RoomDim, autobrightness.RoomUnknown}
	if got := h.pub.roomsSeen(); !slices.Equal(got, want) {
		t.Fatalf("rooms %v, want %v", got, want)
	}
	h.tick(next)
	if got := h.pub.roomsSeen(); len(got) != len(want) {
		t.Errorf("a paused daemon announced the rooms %v", got[len(want):])
	}
}
