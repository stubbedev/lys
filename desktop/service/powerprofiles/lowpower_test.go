package powerprofiles

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/waittest"
)

type fakeDaemon struct {
	mu      sync.Mutex
	snap    Snapshot
	readErr error
	ticks   chan struct{}
}

func (f *fakeDaemon) Read(context.Context) (Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.snap, f.readErr
}

func (f *fakeDaemon) SetActive(context.Context, string) error { return errors.New("not in this test") }

func (f *fakeDaemon) Subscribe(context.Context) (<-chan struct{}, func() error, error) {
	return f.ticks, func() error { return nil }, nil
}

func (f *fakeDaemon) set(snap Snapshot, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snap, f.readErr = snap, err
}

func TestThePowerProfileIsReadAtStartAndOnEachChange(t *testing.T) {
	fake := &fakeDaemon{snap: Snapshot{Available: true, Active: ProfilePowerSaver}, ticks: make(chan struct{})}
	saver, err := FollowSaver(t.Context(), fake, time.Second, func(error) {})
	if err != nil {
		t.Fatal(err)
	}
	for i, step := range []struct {
		snap Snapshot
		want bool
	}{
		{Snapshot{Available: true, Active: ProfileBalanced}, false},
		{Snapshot{Available: true, Active: ProfilePowerSaver}, true},
		{Snapshot{Active: ProfilePowerSaver}, false},
		{Snapshot{Available: true, Active: ProfilePerformance}, false},
	} {
		if i == 0 {
			if got := waittest.For(t, saver, "the profile at start"); !got {
				t.Fatal("power-saver at start read as not power-saver")
			}
		}
		fake.set(step.snap, nil)
		fake.ticks <- struct{}{}
		if got := waittest.For(t, saver, "the changed profile"); got != step.want {
			t.Errorf("%+v read as power-saver %v, want %v", step.snap, got, step.want)
		}
	}
	close(fake.ticks)
	if _, open := <-saver; open {
		t.Error("the profile feed stayed open after the daemon's watch ended")
	}
}

func TestAnUnreadablePowerProfileEndsTheFeed(t *testing.T) {
	noReply := errors.New("org.freedesktop.DBus.Error.NoReply")
	fake := &fakeDaemon{readErr: noReply, ticks: make(chan struct{})}
	heard := make(chan error, 2)
	saver, err := FollowSaver(t.Context(), fake, time.Second, func(err error) { heard <- err })
	if err != nil {
		t.Fatal(err)
	}
	if _, open := <-saver; open {
		t.Error("an unreadable profile was reported as an answer")
	}
	if err := <-heard; !errors.Is(err, noReply) {
		t.Errorf("failed heard %v, want the read's %v", err, noReply)
	}
}

func TestLowPowerStretchesOnlyUnderLowPower(t *testing.T) {
	if got := Paced(time.Second, false); got != time.Second {
		t.Errorf("Paced(1s) outside Low Power = %v, want 1s", got)
	}
	if got, want := Paced(time.Second, true), LowPowerStretch*time.Second; got != want {
		t.Errorf("Paced(1s) under Low Power = %v, want %v", got, want)
	}
}
