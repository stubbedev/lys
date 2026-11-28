package framepace

import (
	"testing"
	"time"
)

const refresh = time.Second / 60

var origin = time.Unix(1000, 0)

func vblank(k int) time.Time { return origin.Add(time.Duration(k) * refresh) }

func presented(p *Pacer, k int) {
	p.Presented(Frame{}, vblank(k), refresh)
}

func frameAt(p *Pacer, now time.Time, cost time.Duration) Frame {
	f := p.Begin(now)
	p.Committed(&f, f.start.Add(cost))
	return f
}

func TestAPacerWithoutFeedbackNeverWaits(t *testing.T) {
	var p Pacer
	if at, wait := p.Wake(origin); wait {
		t.Fatalf("waits until %v with no presentation heard", at)
	}
	if f := p.Begin(origin); !f.target.IsZero() {
		t.Errorf("aims at %v with no presentation heard", f.target)
	}
	p.Presented(Frame{}, origin, 0)
	if at, wait := p.Wake(origin.Add(time.Millisecond)); wait {
		t.Errorf("waits until %v on an output of unknown refresh", at)
	}
}

func TestAFrameOwedAfterAVblankStartsBeforeTheNextByMarginAndCost(t *testing.T) {
	var p Pacer
	presented(&p, 0)
	now := vblank(0).Add(time.Millisecond)
	at, wait := p.Wake(now)
	want := vblank(1).Add(-p.margin - p.cost())
	if !wait || !at.Equal(want) {
		t.Fatalf("Wake = %v, %v, want to wait until %v", at, wait, want)
	}
	if armed, ok := p.Armed(); !ok || !armed.Equal(want) {
		t.Errorf("Armed = %v, %v, want %v", armed, ok, want)
	}
	if _, wait := p.Wake(want); wait {
		t.Fatal("still waits once its start came")
	}
	late := want.Add(2 * time.Millisecond)
	f := p.Begin(late)
	if !f.target.Equal(vblank(1)) || !f.start.Equal(want) {
		t.Errorf("a frame woken late aims at %v from %v, want the next vblank %v from its start %v", f.target, f.start, vblank(1), want)
	}
	if _, ok := p.Armed(); ok {
		t.Error("still armed once the frame began")
	}
	p.Committed(&f, late.Add(time.Millisecond))
	if got, want := p.cost(), 3*time.Millisecond+p.guard(); got != want {
		t.Errorf("cost %v, want the late wake counted in: %v", got, want)
	}
}

func TestAFrameOwedPastItsStartDrawsAtOnceForTheVblankItStillReaches(t *testing.T) {
	var p Pacer
	presented(&p, 0)
	now := vblank(1).Add(-time.Millisecond)
	if _, wait := p.Wake(now); wait {
		t.Fatal("waits though its start before the next vblank has passed")
	}
	if f := p.Begin(now); !f.target.Equal(vblank(2)) {
		t.Errorf("aims at %v, want the vblank after next %v", f.target, vblank(2))
	}
}

func TestAPresentationOlderThanThreeRefreshesStopsPacing(t *testing.T) {
	var p Pacer
	presented(&p, 0)
	if _, wait := p.Wake(vblank(3).Add(time.Millisecond)); wait {
		t.Error("waits on a vblank predicted from a stale presentation")
	}
	if _, wait := p.Wake(vblank(2).Add(time.Millisecond)); !wait {
		t.Error("does not wait on a presentation two refreshes old")
	}
}

func TestATightMissRaisesTheMarginPastIt(t *testing.T) {
	var p Pacer
	presented(&p, 0)
	at, _ := p.Wake(vblank(0))
	margin := p.margin
	f := frameAt(&p, at, 0)
	p.Presented(f, vblank(2), refresh)
	s := p.Stats()
	if s.Missed != 1 || s.Overran != 0 || s.Lagged != 0 {
		t.Fatalf("%+v, want one miss of the latch", s)
	}
	slack := vblank(1).Sub(f.committed)
	if p.margin <= margin || p.margin < slack+p.guard() || p.edge != slack {
		t.Errorf("margin %v edge %v after a miss with slack %v at margin %v, want above both", p.margin, p.edge, slack, margin)
	}
	if p.patience != 2*patienceStart {
		t.Errorf("patience %d, want it doubled to %d", p.patience, 2*patienceStart)
	}
}

func TestAnOverrunRaisesTheMarginAndALaggedFrameChangesNothing(t *testing.T) {
	var p Pacer
	presented(&p, 0)
	at, _ := p.Wake(vblank(0))
	margin := p.margin
	f := frameAt(&p, at, p.cost()+time.Millisecond)
	p.Presented(f, vblank(2), refresh)
	if s := p.Stats(); s.Overran != 1 || s.Missed != 0 {
		t.Fatalf("%+v, want one overrun", s)
	}
	if p.margin != margin+refresh/8 || p.edge != 0 {
		t.Errorf("margin %v edge %v, want %v and no edge", p.margin, p.edge, margin+refresh/8)
	}
	margin = p.margin
	early := p.Begin(vblank(2))
	p.Committed(&early, vblank(2))
	p.Presented(early, early.target.Add(refresh), refresh)
	if s := p.Stats(); s.Lagged != 1 || p.margin != margin {
		t.Errorf("%+v margin %v, want a lag that leaves the margin at %v", s, p.margin, margin)
	}
}

func TestHitsShrinkTheMarginToTheEdgeThenWaitForThePatience(t *testing.T) {
	var p Pacer
	presented(&p, 0)
	k := 0
	hit := func() {
		at, wait := p.Wake(vblank(k))
		if !wait {
			t.Fatalf("frame %d starts at once with margin %v", k, p.margin)
		}
		f := frameAt(&p, at, 0)
		k++
		p.Presented(f, vblank(k), refresh)
	}
	start := p.margin
	for range probeAfter {
		hit()
	}
	if p.margin != start-p.step() {
		t.Fatalf("margin %v after %d tight hits, want one step below %v", p.margin, probeAfter, start)
	}
	held := p.margin
	p.edge = held - p.guard()
	for range p.patience - 1 {
		hit()
	}
	if p.margin != held {
		t.Fatalf("margin %v, want it held a guard above the edge %v until the patience", p.margin, p.edge)
	}
	hit()
	if p.margin != held-p.step() || p.edge != 0 {
		t.Errorf("margin %v edge %v after the patience, want %v and the edge forgotten", p.margin, p.edge, held-p.step())
	}
}

func TestFramesShownBeforeTheirAimProbeTheMarginDown(t *testing.T) {
	var p Pacer
	presented(&p, 0)
	p.margin = refresh
	k := 0
	for range probeAfter {
		f := p.Begin(vblank(k))
		p.Committed(&f, vblank(k))
		k++
		p.Presented(f, vblank(k), refresh)
	}
	if p.margin != refresh-p.step() {
		t.Errorf("margin %v, want a whole-refresh margin stepped down by frames shown early", p.margin)
	}
}

func TestANewRefreshStartsOver(t *testing.T) {
	var p Pacer
	presented(&p, 0)
	p.margin, p.edge, p.patience = time.Millisecond, time.Millisecond, patienceCap
	p.Presented(Frame{}, vblank(1), refresh/2)
	if p.margin != refresh/4+refresh/64 || p.edge != 0 || p.patience != patienceStart {
		t.Errorf("margin %v edge %v patience %d at a doubled refresh rate, want the defaults", p.margin, p.edge, p.patience)
	}
}
