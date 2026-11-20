package bar

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/service/powerprofiles"
	"github.com/stubbedev/lys/lib/waittest"
)

type profileDaemon struct {
	mu     sync.Mutex
	active string
	ticks  chan struct{}
}

func (p *profileDaemon) Read(context.Context) (powerprofiles.Snapshot, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return powerprofiles.Snapshot{Available: true, Active: p.active}, nil
}

func (*profileDaemon) SetActive(context.Context, string) error {
	return errors.New("not in this test")
}

func (p *profileDaemon) Subscribe(context.Context) (<-chan struct{}, func() error, error) {
	return p.ticks, func() error { return nil }, nil
}

func (p *profileDaemon) switchTo(profile string) {
	p.mu.Lock()
	p.active = profile
	p.mu.Unlock()
	p.ticks <- struct{}{}
}

func TestTheBarsPollsFollowLowPower(t *testing.T) {
	daemon := &profileDaemon{active: powerprofiles.ProfileBalanced, ticks: make(chan struct{})}
	h := testPollHub(t)
	var mu sync.Mutex
	var seen []bool
	stop := followLowPower(daemon, h.setLowPower, func(on bool) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, on)
	})
	defer stop()
	stretched := powerprofiles.LowPowerStretch * time.Second
	pace := func() time.Duration { return h.pace(pauseIdleOrLocked, time.Second) }
	waittest.Until(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(seen) == 1
	}, "the profile at start")
	if got := pace(); got != time.Second {
		t.Fatalf("under balanced a 1s poll waits %v", got)
	}
	daemon.switchTo(powerprofiles.ProfilePowerSaver)
	waittest.Until(t, func() bool { return pace() == stretched }, "the polls stretched under power-saver")
	daemon.switchTo(powerprofiles.ProfilePerformance)
	waittest.Until(t, func() bool { return pace() == time.Second }, "the polls back at their pace on performance")
	mu.Lock()
	defer mu.Unlock()
	if want := []bool{false, true, false}; len(seen) != len(want) || seen[0] || !seen[1] || seen[2] {
		t.Errorf("every follower saw %v, want %v", seen, want)
	}
}
