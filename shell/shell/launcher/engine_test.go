package launcher

import (
	"context"
	"sync"
	"testing"

	"github.com/stubbedev/lys/desktop/launchermatch"
	"github.com/stubbedev/lys/lib/waittest"

	engine "github.com/stubbedev/lys/shell/service/launcher"
)

// slowQuerier answers a query with one row named after it, except "slow",
// which waits until a newer query ends it.
type slowQuerier struct {
	started chan string
	ended   chan string
}

func (*slowQuerier) Name() string                          { return "slow" }
func (*slowQuerier) Load(context.Context) engine.ModeState { return engine.ModeState{} }
func (*slowQuerier) Activate(context.Context, engine.Target, engine.ActivateKind, string) engine.Action {
	return engine.ActionNothing{}
}

func (q *slowQuerier) Query(ctx context.Context, query string) (engine.ModeState, bool) {
	q.started <- query
	if query == "slow" {
		<-ctx.Done()
		q.ended <- query
		return engine.ModeState{}, false
	}
	return engine.ModeState{Items: []engine.Item{engine.NewItem(query)}}, true
}

type matchSink struct {
	mu      sync.Mutex
	matches []engineMatches
}

func (*matchSink) onState(engineState)     {}
func (*matchSink) onAction(engine.Action)  {}
func (*matchSink) onPreview(enginePreview) {}
func (s *matchSink) onMatches(m engineMatches) {
	s.mu.Lock()
	s.matches = append(s.matches, m)
	s.mu.Unlock()
}

func (s *matchSink) rows() [][]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]string, len(s.matches))
	for i, m := range s.matches {
		for _, idx := range m.matched {
			out[i] = append(out[i], m.items[idx].Display)
		}
	}
	return out
}

func TestANewerQueryEndsTheOneInFlight(t *testing.T) {
	q := &slowQuerier{started: make(chan string, 4), ended: make(chan string, 1)}
	sess, err := engine.NewSession([]engine.Mode{q}, launchermatch.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	sink := &matchSink{}
	var wg sync.WaitGroup
	a := startActor(wg.Go, sess, 0, sink, func(fn func()) { fn() })
	defer func() {
		a.stop()
		wg.Wait()
	}()
	waittest.Until(t, func() bool { return len(sink.rows()) == 1 }, "the first frame")
	a.send(cmdSetQuery{query: "slow"})
	if got := waittest.For(t, q.started, "the slow query to start"); got != "slow" {
		t.Fatalf("started %q", got)
	}
	a.send(cmdSetQuery{query: "fast"})
	if got := waittest.For(t, q.ended, "the slow query to end"); got != "slow" {
		t.Fatalf("ended %q", got)
	}
	waittest.Until(t, func() bool { return len(sink.rows()) == 2 }, "the newer query's frame")
	if rows := sink.rows(); len(rows[1]) != 1 || rows[1][0] != "fast" {
		t.Errorf("frames %v, want the ended query to send none and the newer one its row", rows)
	}
}
