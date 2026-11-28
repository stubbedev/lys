package app

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/internal/wlsession"
)

const (
	simRefresh = time.Second / 60
	simCost    = time.Millisecond
	simEvery   = 4 * time.Millisecond
	simWarm    = 240
	simMoves   = simWarm + 960
)

// vblankSim is a compositor on a simulated clock: it latches the
// window's commit lead before each vblank, presents it at the vblank with
// its presentation feedback and frame callback, and feeds the window an
// input every simEvery whose frame paints for simCost.
type vblankSim struct {
	t       *testing.T
	r       *pacingRig
	lead    time.Duration
	vblank  time.Time
	ms      uint32
	inputs  []time.Time
	painted int
	pending *simCommit
	commits int
	shown   int
	lat     []time.Duration
	idle    int
}

type simCommit struct {
	at      time.Time
	painted int
	wait    *presentWait
}

func newVblankSim(t *testing.T, feedback bool, lead time.Duration) *vblankSim {
	t.Helper()
	r := newPacingRig(t)
	r.h.surf.presentation = feedback
	s := &vblankSim{t: t, r: r, lead: lead, vblank: r.now.Add(simRefresh), painted: -1}
	r.area().OnDraw = func(*render.Canvas, render.Rect) {
		r.now = r.now.Add(simCost)
		s.painted = len(s.inputs) - 1
	}
	return s
}

func (s *vblankSim) run() {
	s.t.Helper()
	nextInput := s.r.now.Add(simEvery)
	for s.shown < simMoves {
		at, flip := s.vblank, true
		if len(s.inputs) < simMoves && nextInput.Before(at) {
			at, flip = nextInput, false
		}
		if s.r.armed.After(s.r.now) && s.r.armed.Before(at) {
			at, flip = s.r.armed, false
		}
		s.r.now = at
		switch {
		case flip:
			s.flip()
		case at.Equal(nextInput):
			s.inputs = append(s.inputs, at)
			s.r.area().QueueDraw()
			s.r.h.wnd.dirty = true
			nextInput = nextInput.Add(simEvery)
		}
		if err := s.r.a.tick(s.r.a.step, s.r.now); err != nil {
			s.t.Fatalf("tick: %v", err)
		}
		s.noteCommit()
	}
}

func (s *vblankSim) noteCommit() {
	s.t.Helper()
	if s.r.h.surf.commits == s.commits {
		return
	}
	if s.pending != nil || s.r.h.surf.commits != s.commits+1 {
		s.t.Fatalf("%d commits with a frame still waiting for its vblank", s.r.h.surf.commits-s.commits)
	}
	s.commits = s.r.h.surf.commits
	c := &simCommit{at: s.r.now, painted: s.painted}
	if n := len(s.r.h.surf.feedback); s.r.h.surf.presentation && n > 0 {
		c.wait = s.r.h.surf.feedback[n-1]
	}
	s.pending = c
}

func (s *vblankSim) flip() {
	at := s.vblank
	s.vblank = s.vblank.Add(simRefresh)
	s.ms += uint32(simRefresh / time.Millisecond)
	c := s.pending
	if c == nil || c.at.After(at.Add(-s.lead)) {
		if len(s.inputs) > simWarm && s.shown < len(s.inputs) {
			s.idle++
		}
		return
	}
	s.pending = nil
	for _, b := range s.r.h.bufs {
		if b.Busy() {
			b.Release()
		}
	}
	if c.wait != nil {
		c.wait.w.presented(c.wait, at, simRefresh, true)
	}
	s.r.h.wnd.frame = wlsession.FrameDone{Ready: true, MS: s.ms}
	for ; s.shown <= c.painted; s.shown++ {
		if s.shown >= simWarm {
			s.lat = append(s.lat, at.Sub(s.inputs[s.shown]))
		}
	}
}

func (s *vblankSim) meanLatency() time.Duration {
	var sum time.Duration
	for _, l := range s.lat {
		sum += l
	}
	return sum / time.Duration(len(s.lat))
}

// A window fed input four times a refresh, against a compositor that
// latches 3 ms before each vblank, shows each input sooner when it paces
// its frames by presentation feedback than when it draws as each frame
// callback returns, and still shows a new frame at every vblank but the
// ones the pacer missed learning the latch.
func TestPacedFramesShowInputSoonerAndKeepEveryVblank(t *testing.T) {
	const lead = 3 * time.Millisecond
	callbacks := newVblankSim(t, false, lead)
	callbacks.run()
	paced := newVblankSim(t, true, lead)
	paced.run()
	stats := paced.r.h.wnd.pace.Stats()
	t.Logf("frame callbacks: mean input to present %v, %d vblanks without a frame", callbacks.meanLatency(), callbacks.idle)
	t.Logf("paced: mean input to present %v, %d vblanks without a frame, %+v", paced.meanLatency(), paced.idle, stats)
	if callbacks.idle != 0 {
		t.Errorf("drawing at each frame callback left %d vblanks without a frame, want none", callbacks.idle)
	}
	if paced.idle > stats.Missed+stats.Overran || stats.Missed > 2 || stats.Overran != 0 || stats.Lagged != 0 {
		t.Errorf("paced frames left %d vblanks without a frame, %+v; want at most the 2 misses that learn the latch", paced.idle, stats)
	}
	if gain := callbacks.meanLatency() - paced.meanLatency(); gain < simRefresh/2 {
		t.Errorf("pacing brought input to present from %v to %v, want at least half a refresh sooner", callbacks.meanLatency(), paced.meanLatency())
	}
	if stats.Margin < lead {
		t.Errorf("the margin settled at %v, inside the compositor's %v latch", stats.Margin, lead)
	}
}

// A compositor latching earlier than half a refresh makes the first paced
// frames miss: the pacer raises its margin past the latch and keeps every
// later vblank.
func TestPacedFramesLearnAnEarlyLatch(t *testing.T) {
	lead := simRefresh/2 + 3*time.Millisecond
	s := newVblankSim(t, true, lead)
	s.run()
	stats := s.r.h.wnd.pace.Stats()
	t.Logf("mean input to present %v, %d vblanks without a frame, %+v", s.meanLatency(), s.idle, stats)
	if stats.Missed == 0 || stats.Missed > 3 {
		t.Errorf("%+v, want the first frames to miss the early latch, at most 3 of them", stats)
	}
	if stats.Margin < lead || stats.Margin >= simRefresh {
		t.Errorf("the margin settled at %v, want it past the compositor's %v latch and inside a refresh", stats.Margin, lead)
	}
}

// Without presentation feedback a window draws as its frame callback
// returns, as it always did: no frame waits.
func TestFramesWithoutFeedbackNeverWait(t *testing.T) {
	s := newVblankSim(t, false, 3*time.Millisecond)
	s.run()
	if stats := s.r.h.wnd.pace.Stats(); stats.Paced != 0 {
		t.Errorf("%+v, want no frame paced without feedback", stats)
	}
	if want := simRefresh; s.meanLatency() < want {
		t.Errorf("mean input to present %v, want the frame callback's at least %v", s.meanLatency(), want)
	}
}

// A window owed a frame right after its last one was presented holds it
// until the pacer's start before the next vblank, the loop armed for that
// start, and commits it then.
func TestAPacedFrameWaitsForItsStartAndTheLoopWakesForIt(t *testing.T) {
	r := newPacingRig(t)
	r.h.surf.presentation = true
	r.pass(time.Second)
	commits := len(r.h.surf.feedback)
	if commits != 1 {
		t.Fatalf("%d frames with feedback on the first pass, want 1", commits)
	}
	r.h.surf.feedback[0].w.presented(r.h.surf.feedback[0], r.now, simRefresh, true)
	r.callback(time.Millisecond, 1000)
	r.area().QueueDraw()
	r.h.wnd.dirty = true
	r.pass(time.Millisecond)
	if got := r.h.surf.commits; got != commits {
		t.Fatalf("%d commits right after the vblank, want the frame held", got)
	}
	start, ok := r.h.wnd.pace.Armed()
	if !ok || !r.armed.Equal(start) || !start.After(r.now) {
		t.Fatalf("pacer armed %v (%v), loop armed %v at %v, want the loop woken at the pacer's start ahead", start, ok, r.armed, r.now)
	}
	r.pass(start.Sub(r.now))
	if got := r.h.surf.commits; got != commits+1 {
		t.Errorf("%d commits at the pacer's start, want the held frame", got-commits)
	}
}
