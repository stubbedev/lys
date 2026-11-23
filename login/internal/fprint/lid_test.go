package fprint

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/logind/logindtest"
	"github.com/stubbedev/lys/lib/waittest"
)

type lidRig struct {
	*rig
	logind *logindtest.Logind
}

func newLidRig(t *testing.T, lidClosed bool) *lidRig {
	t.Helper()
	bus := dbustest.Start(t)
	l := logindtest.Serve(t, bus, logindtest.Config{})
	l.SetLidClosed(lidClosed)
	r := &lidRig{rig: newRigOn(t, bus, []string{"right-index-finger"}, false), logind: l}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	t.Cleanup(cancel)
	lid, err := followLid(ctx, r.client)
	if err != nil {
		t.Fatal(err)
	}
	go func() { r.ended <- lid.verify(ctx, r.client, "ada", func(res Result) { r.results <- res }) }()
	return r
}

func (r *lidRig) awaitCalls(t *testing.T, want []string) {
	t.Helper()
	waittest.Until(t, func() bool { return slices.Equal(r.dev.recorded(), want) }, fmt.Sprintf("the reader's calls %v", want))
}

func TestWithTheLidClosedTheReaderIsNeverClaimed(t *testing.T) {
	r := newLidRig(t, true)
	r.logind.SwitchLid(t, true)
	r.logind.SwitchLid(t, false)
	r.awaitStart(t)
	if got, want := r.dev.recorded(), []string{"list ada", "claim ada", "start any"}; !slices.Equal(got, want) {
		t.Fatalf("calls %v by the time the lid opened, want only %v after it", got, want)
	}
	r.cancel()
	if err := r.awaitEnd(t); err != nil {
		t.Fatal(err)
	}
}

func TestOpeningTheLidVerifiesAndClosingItReleasesTheReader(t *testing.T) {
	r := newLidRig(t, true)
	r.logind.SwitchLid(t, false)
	r.awaitStart(t)
	r.logind.SwitchLid(t, true)
	if got := r.awaitResult(t); got != Stopped {
		t.Fatalf("closing the lid reported %v, want Stopped", got)
	}
	r.awaitCalls(t, []string{"list ada", "claim ada", "start any", "stop", "release"})
	r.dev.emit(t, "verify-match", true)
	r.logind.SwitchLid(t, false)
	r.listened = false
	r.awaitStart(t)
	r.dev.emit(t, "verify-match", true)
	if got := r.awaitResult(t); got != Match {
		t.Fatalf("result %v, want Match", got)
	}
	if err := r.awaitEnd(t); err != nil {
		t.Fatal(err)
	}
	if len(r.results) != 0 {
		t.Fatal("a status sent while the lid was closed reached the caller")
	}
}

func TestWithTheLidOpenAMatchEndsTheVerification(t *testing.T) {
	r := newLidRig(t, false)
	r.awaitStart(t)
	r.dev.emit(t, "verify-match", true)
	if got := r.awaitResult(t); got != Match {
		t.Fatalf("result %v, want Match", got)
	}
	if err := r.awaitEnd(t); err != nil {
		t.Fatal(err)
	}
}

func TestWithoutLogindTheLidCannotBeFollowed(t *testing.T) {
	r := newRig(t, []string{"right-index-finger"}, false)
	err := VerifyWhileLidOpen(context.Background(), r.client, "ada", func(Result) { t.Error("a result without logind") })
	if err == nil {
		t.Fatal("VerifyWhileLidOpen without logind succeeded")
	}
	if got := r.dev.recorded(); len(got) != 0 {
		t.Fatalf("the reader was used without logind: %v", got)
	}
}
