package daemon_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/power/internal/daemon"
)

func runOnPrivateBus(t *testing.T, ready func() error) (stop context.CancelFunc, done <-chan error) {
	t.Helper()
	return runFixtureOn(t, newFixture(t), dbustest.Start(t), ready)
}

func runFixtureOn(t *testing.T, f *fixture, bus *dbustest.Bus, ready func() error) (stop context.CancelFunc, done <-chan error) {
	t.Helper()
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", bus.Address)
	ctx, cancel := context.WithCancel(context.Background())
	ended, finished := make(chan error, 1), make(chan struct{})
	go func() {
		defer close(finished)
		ended <- daemon.Run(ctx, f.cfg, f.log, ready)
	}()
	t.Cleanup(func() {
		cancel()
		<-finished
	})
	return cancel, ended
}

func TestRunReportsReadyOnceAfterItsFirstEvaluation(t *testing.T) {
	readies := make(chan struct{}, 2)
	stop, done := runOnPrivateBus(t, func() error {
		readies <- struct{}{}
		return nil
	})
	waittest.For(t, readies, "lys-power to report ready")
	stop()
	if err := waittest.For(t, done, "Run to end with its context"); err != nil {
		t.Fatalf("Run ended with %v", err)
	}
	if n := len(readies); n != 0 {
		t.Errorf("Run reported ready %d more times", n)
	}
}

func TestRunEndsWhenItCannotReportReady(t *testing.T) {
	refused := errors.New("no supervisor")
	_, done := runOnPrivateBus(t, func() error { return refused })
	if err := waittest.For(t, done, "Run to end"); !errors.Is(err, refused) {
		t.Errorf("Run ended with %v, want the refused report", err)
	}
}
