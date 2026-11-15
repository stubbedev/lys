package autobright

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/shell/service/brightness"

	"github.com/stubbedev/lys/power/autobrightness"
	"github.com/stubbedev/lys/power/internal/ambient"
)

const (
	hang     = 5 * time.Second
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
	case <-time.After(hang):
		return 0, errors.New("the test gave no reading")
	}
}

type backlight struct {
	mu     sync.Mutex
	levels map[string]uint32
	ticks  chan struct{}
	writes chan float64
}

func newBacklight(names ...string) *backlight {
	b := &backlight{levels: map[string]uint32{}, ticks: make(chan struct{}), writes: make(chan float64, 64)}
	for _, name := range names {
		b.levels[name] = 400
	}
	return b
}

func (b *backlight) Devices(context.Context) ([]brightness.Device, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []brightness.Device
	for _, name := range []string{"acpi_video0", "intel_backlight"} {
		if raw, ok := b.levels[name]; ok {
			out = append(out, brightness.Device{Name: name, Brightness: raw, Max: maxRaw})
		}
	}
	return out, nil
}

func (b *backlight) Set(_ context.Context, name string, percent float64) error {
	b.mu.Lock()
	b.levels[name] = uint32(math.Round(percent / 100 * maxRaw))
	b.mu.Unlock()
	b.writes <- percent
	return nil
}

func (b *backlight) Subscribe(context.Context) (<-chan struct{}, func(), error) {
	return b.ticks, func() {}, nil
}

func (b *backlight) setAll(raw uint32) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for name := range b.levels {
		b.levels[name] = raw
	}
}

func (b *backlight) level(name string) uint32 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.levels[name]
}

type session struct {
	mu             sync.Mutex
	active, locked bool
}

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

func (s *session) set(active, locked bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active, s.locked = active, locked
}

type publisher struct{ states chan autobrightness.State }

func (p *publisher) Publish(s autobrightness.State) error {
	p.states <- s
	return nil
}

type harness struct {
	t       *testing.T
	clock   *clock
	light   *light
	back    *backlight
	session *session
	pub     *publisher
	daemon  *Daemon
	curve   ambient.Curve
	done    chan error
	cancel  context.CancelFunc
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
		session: &session{active: true},
		pub:     &publisher{states: make(chan autobrightness.State, 8)},
		curve:   curve,
		done:    make(chan error, 1),
	}
	h.daemon = New(h.light, h.back, h.session, h.clock, slog.New(slog.NewTextHandler(io.Discard, nil)), Params{
		Curve: curve, Smoother: smoother, Band: 6, Steps: steps, Step: stepGap, Interval: interval,
	})
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go func() { h.done <- h.daemon.Run(ctx, h.pub) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-h.done:
			if err != nil {
				t.Errorf("the daemon stopped with %v", err)
			}
		case <-time.After(hang):
			t.Error("the daemon did not stop")
		}
	})
	return h
}

func (h *harness) read(lux ambient.Lux) {
	h.t.Helper()
	select {
	case h.light.readings <- reading{lux: lux}:
	case <-time.After(hang):
		h.t.Fatal("the daemon took no sample")
	}
}

func (h *harness) readErr(err error) {
	h.t.Helper()
	select {
	case h.light.readings <- reading{err: err}:
	case <-time.After(hang):
		h.t.Fatal("the daemon took no sample")
	}
}

func (h *harness) armed() chan time.Time {
	h.t.Helper()
	select {
	case ch := <-h.clock.samples:
		return ch
	case <-time.After(hang):
		h.t.Fatal("the daemon armed no next sample")
	}
	return nil
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
		select {
		case ch := <-h.clock.steps:
			ch <- h.clock.now
		case <-time.After(hang):
			h.t.Fatalf("the transition stopped after %v", levels)
		}
		for range devices {
			select {
			case p := <-h.back.writes:
				levels = append(levels, p)
			case <-time.After(hang):
				h.t.Fatalf("a step wrote nothing after %v", levels)
			}
		}
	}
	return levels
}

func (h *harness) changed() {
	h.t.Helper()
	for range 2 {
		select {
		case h.back.ticks <- struct{}{}:
		case <-time.After(hang):
			h.t.Fatal("the daemon does not watch the backlight")
		}
	}
}

func (h *harness) published(want autobrightness.State) {
	h.t.Helper()
	select {
	case got := <-h.pub.states:
		if got != want {
			h.t.Fatalf("published %v, want %v", got, want)
		}
	case <-time.After(hang):
		h.t.Fatalf("published nothing, want %v", want)
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
	ctx, cancel := context.WithTimeout(context.Background(), hang)
	defer cancel()
	if err := h.daemon.Resume(ctx); err != nil {
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
	h.back.setAll(ours * 30 / 100)
	h.changed()
	h.published(autobrightness.Paused)
	h.back.setAll(ours)
	h.changed()
	h.published(autobrightness.Following)

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
	select {
	case err := <-h.done:
		if !errors.Is(err, fail) {
			t.Errorf("the daemon stopped with %v, want %v", err, fail)
		}
		h.done <- nil
	case <-time.After(hang):
		t.Fatal("the daemon went on without its light")
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
	d := New(&light{}, newBacklight(), &session{active: true}, newClock(), slog.New(slog.NewTextHandler(io.Discard, nil)), Params{Steps: steps})
	if err := d.Run(context.Background(), &publisher{}); err == nil {
		t.Fatal("the daemon ran without a backlight")
	}
}
