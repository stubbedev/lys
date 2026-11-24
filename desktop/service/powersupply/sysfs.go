package powersupply

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/udevmonitor"
)

// Refresh is how often a subscription to batteries reads them again: a
// battery's firmware announces a change of state, but not every change
// of its level.
const Refresh = 30 * time.Second

var subsystems = []string{"power_supply"}

// Sysfs is the power supplies under one directory as a Source.
type Sysfs struct {
	dir     string
	listen  func([]string) (*udevmonitor.Listener, error)
	refresh time.Duration
}

// New reads the supplies under dir, Dir outside tests.
func New(dir string) *Sysfs {
	return &Sysfs{dir: dir, listen: udevmonitor.Listen, refresh: Refresh}
}

// Read aggregates the machine's batteries; a Device that is not
// IsPresent without one.
func (s *Sysfs) Read(context.Context) (Device, error) {
	snap, err := read(s.dir)
	if err != nil {
		return Device{}, err
	}
	return aggregate(snap.cells), nil
}

// Peripherals lists the peripherals reporting a level, by name.
func (s *Sysfs) Peripherals(context.Context) ([]Peripheral, error) {
	snap, err := read(s.dir)
	return snap.peripherals, err
}

// OnBattery reports whether a battery powers the machine with no mains
// or USB source online.
func (s *Sysfs) OnBattery(context.Context) (bool, error) {
	snap, err := read(s.dir)
	return len(snap.cells) > 0 && !snap.onAC, err
}

// Subscribe ticks per change to what Read returns.
func (s *Sysfs) Subscribe(ctx context.Context) (<-chan struct{}, func() error, error) {
	return watch(ctx, s, s.refresh, func(snap snapshot) Device { return aggregate(snap.cells) }, func(a, b Device) bool { return a == b })
}

// SubscribePeripherals ticks per change to what Peripherals returns.
func (s *Sysfs) SubscribePeripherals(ctx context.Context) (<-chan struct{}, func() error, error) {
	return watch(ctx, s, s.refresh, func(snap snapshot) []Peripheral { return snap.peripherals }, slices.Equal[[]Peripheral])
}

// SubscribeOnBattery ticks per change to what OnBattery returns. Every
// change of a power source is announced, so it reads only on events.
func (s *Sysfs) SubscribeOnBattery(ctx context.Context) (<-chan struct{}, func() error, error) {
	return watch(ctx, s, 0, func(snap snapshot) bool { return len(snap.cells) > 0 && !snap.onAC }, func(a, b bool) bool { return a == b })
}

type reading[T any] struct {
	value T
	err   error
}

// watch reads the supplies at every power_supply event, after a burst
// lost some, and every refresh when it is not zero, and ticks when the
// part of them that pick takes changed or could not be read. The
// channel closes once the watch has ended: at stop, which returns then,
// or at ctx's end.
func watch[T any](ctx context.Context, s *Sysfs, refresh time.Duration, pick func(snapshot) T, equal func(a, b T) bool) (<-chan struct{}, func() error, error) {
	events, err := s.listen(subsystems)
	if err != nil {
		return nil, nil, fmt.Errorf("powersupply: follow %s: %w", s.dir, err)
	}
	current := func() reading[T] {
		snap, err := read(s.dir)
		return reading[T]{pick(snap), err}
	}
	last := current()
	ticks := make(chan struct{}, 1)
	triggers := make(chan struct{}, 1)
	stopping := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() { forward(events, triggers, s.dir) })
	wg.Go(func() {
		defer close(ticks)
		var timer <-chan time.Time
		if refresh > 0 {
			t := time.NewTicker(refresh)
			defer t.Stop()
			timer = t.C
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopping:
				return
			case _, open := <-triggers:
				if !open {
					return
				}
			case <-timer:
			}
			next := current()
			if next.err == nil && last.err == nil && equal(next.value, last.value) {
				continue
			}
			last = next
			select {
			case ticks <- struct{}{}:
			default:
			}
		}
	})
	var once sync.Once
	var closeErr error
	end := func() {
		once.Do(func() {
			close(stopping)
			if err := events.Close(); err != nil {
				closeErr = fmt.Errorf("powersupply: stop following %s: %w", s.dir, err)
			}
			wg.Wait()
		})
	}
	go func() {
		select {
		case <-ctx.Done():
			end()
		case <-stopping:
		}
	}()
	return ticks, func() error {
		end()
		return closeErr
	}, nil
}

// forward turns the listener's events into coalesced triggers until it
// is closed, closing triggers then.
func forward(events *udevmonitor.Listener, triggers chan<- struct{}, dir string) {
	defer close(triggers)
	for {
		_, err := events.Next()
		switch {
		case errors.Is(err, os.ErrClosed):
			return
		case errors.Is(err, udevmonitor.ErrLost):
		case err != nil:
			log.Printf("powersupply: follow %s: %v", dir, err)
			return
		}
		select {
		case triggers <- struct{}{}:
		default:
		}
	}
}
