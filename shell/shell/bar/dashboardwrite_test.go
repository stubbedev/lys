package bar

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
)

func TestAServiceWriteIsBoundedByTheOffLoopTimeout(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	type bound struct {
		deadline, earliest, latest time.Time
		ok                         bool
	}
	bounds := make(chan bound, 1)
	answered := make(chan error, 1)
	start := time.Now()
	serviceWrite(ctx, func(c context.Context) error {
		d, ok := c.Deadline()
		bounds <- bound{d, start.Add(offLoopTimeout), time.Now().Add(offLoopTimeout), ok}
		return nil
	}, func(err error) { answered <- err })
	b := <-bounds
	if !b.ok || b.deadline.Before(b.earliest) || b.deadline.After(b.latest) {
		t.Errorf("the write's deadline = %v (set %v), want %v from when it began", b.deadline, b.ok, offLoopTimeout)
	}
	if err := <-answered; err != nil {
		t.Errorf("a write that went through answered %v", err)
	}
}

func TestAHungServiceWriteEndsWithItsContext(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	life, cancel := context.WithCancel(context.Background())
	ctx.gen.life = life
	started := make(chan struct{})
	answered := make(chan error, 1)
	serviceWrite(ctx, func(c context.Context) error {
		close(started)
		<-c.Done()
		return c.Err()
	}, func(err error) { answered <- err })
	<-started
	cancel()
	select {
	case err := <-answered:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("the hung write answered %v, want its context's end", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the hung write outlived its context")
	}
}

func TestReportWriteShowsAFailureAndHidesASuccess(t *testing.T) {
	alert := writeAlert(testFont(t), "x-failed")
	if alert.Visible() {
		t.Fatal("the alert shows before any write")
	}
	reportWrite(alert, "test", "It failed", errors.New("org.freedesktop.DBus.Error.AccessDenied"))
	if !alert.Visible() || alert.Title() != "It failed" || alert.Description() != "org.freedesktop.DBus.Error.AccessDenied" {
		t.Errorf("after a failure: visible %v title %q description %q", alert.Visible(), alert.Title(), alert.Description())
	}
	reportWrite(alert, "test", "It failed", nil)
	if alert.Visible() {
		t.Error("a write that went through left the alert up")
	}
}
