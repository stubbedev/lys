// Package framepace places a window's frames against its output's
// refresh. It learns the vblank cadence from presentation feedback, the
// frame's cost from the window's own draws, and how long before a vblank
// the compositor latches from the frames that missed one, and starts each
// frame as late before the next vblank as those allow: input that arrives
// in the meantime is in the frame instead of waiting a refresh for the
// next one. Without fresh feedback a frame starts at once.
package framepace

import "time"

const (
	costSamples    = 16
	staleRefreshes = 3
	probeAfter     = 8
	patienceStart  = 16
	patienceCap    = 1024
)

// Pacer is one surface's frame placement. Its zero value starts every
// frame at once until a presentation arrives.
type Pacer struct {
	refresh  time.Duration
	vblank   time.Time
	margin   time.Duration
	edge     time.Duration
	hits     int
	patience int
	costs    [costSamples]time.Duration
	nextCost int
	armed    time.Time
	aim      time.Time
	stats    Stats
}

// Stats counts the frames a Pacer aimed at a vblank. A frame presented
// after the vblank it aimed at counts in one of Missed, Overran and
// Lagged.
type Stats struct {
	// Paced is the frames presented that aimed at a vblank.
	Paced int
	// Missed is those committed close to their deadline: the compositor
	// latched earlier than the margin allowed.
	Missed int
	// Overran is those committed after their deadline: their cost was
	// underestimated.
	Overran int
	// Lagged is those committed with time to spare: the compositor
	// itself was late.
	Lagged int
	// Margin is how long before the vblank frames are committed now.
	Margin time.Duration
}

// Frame is one frame's plan, from Begin to its presentation.
type Frame struct {
	target    time.Time
	margin    time.Duration
	start     time.Time
	committed time.Time
}

// Wake reports when a frame owed at now should start: the deadline and
// true to wait for it, false to start now. A frame waits only while the
// next vblank is predictable from a recent presentation and its start
// before that vblank is still ahead.
func (p *Pacer) Wake(now time.Time) (time.Time, bool) {
	if !p.live(now) {
		p.armed = time.Time{}
		return time.Time{}, false
	}
	aim := p.vblankAfter(now)
	start := aim.Add(-p.margin - p.cost())
	if start.After(now) {
		p.armed, p.aim = start, aim
		return start, true
	}
	if !p.armed.Equal(start) {
		p.armed = time.Time{}
	}
	return time.Time{}, false
}

// Armed is the start Wake asked the frame to wait for, until the frame
// begins.
func (p *Pacer) Armed() (time.Time, bool) { return p.armed, !p.armed.IsZero() }

// Begin plans the frame starting at now. A frame that waited for Wake's
// start aims at the vblank Wake chose and counts its cost from that
// start, so a late wake-up is part of the cost the next frames budget
// for; any other frame aims at the first vblank its cost and the margin
// still reach.
func (p *Pacer) Begin(now time.Time) Frame {
	f := Frame{start: now, margin: p.margin}
	switch {
	case !p.armed.IsZero() && !p.armed.After(now):
		f.start, f.target = p.armed, p.aim
	case p.live(now):
		f.target = p.vblankFrom(now.Add(p.cost() + p.margin))
	}
	p.armed = time.Time{}
	return f
}

// Committed records that f went to the compositor at at: its cost. The
// next frames budget for the dearest of the last costSamples, plus a
// sixteenth of a refresh for a wake-up that comes late.
func (p *Pacer) Committed(f *Frame, at time.Time) {
	f.committed = at
	p.costs[p.nextCost] = max(at.Sub(f.start), 0)
	p.nextCost = (p.nextCost + 1) % costSamples
}

// Presented takes f's presentation feedback: shown at at on an output
// refreshing every refresh, zero when the compositor does not know.
//
// A frame shown after the vblank it aimed at, though committed close to
// its deadline, says the compositor latches earlier than the margin
// allowed: that slack becomes the edge, and the margin rises three
// sixteenths of a refresh past it. One that overran its deadline had its
// cost underestimated, and raises the margin an eighth of a refresh.
// Either doubles the patience. One committed with time to spare says the
// compositor itself was late, and changes nothing.
//
// Frames shown in time though committed close to their deadline, and
// frames shown a vblank before the one they aimed at, shrink the margin
// a thirty-second of a refresh every probeAfter of them, down to a
// sixteenth of a refresh above the edge; the step below that waits for
// the patience, and forgets the edge.
func (p *Pacer) Presented(f Frame, at time.Time, refresh time.Duration) {
	if refresh != p.refresh {
		p.retune(refresh)
	}
	if at.After(p.vblank) {
		p.vblank = at
	}
	if f.target.IsZero() || refresh <= 0 {
		return
	}
	p.stats.Paced++
	switch off := at.Sub(f.target); {
	case off < -refresh/2:
		p.hit()
		return
	case off <= refresh/2:
		if p.tight(f) {
			p.hit()
		}
		return
	}
	switch slack := f.target.Sub(f.committed); {
	case slack < f.margin:
		p.stats.Overran++
		p.margin += refresh / 8
	case p.tight(f):
		p.stats.Missed++
		p.edge = slack
		p.margin = max(p.margin, slack+p.guard()+refresh/8)
	default:
		p.stats.Lagged++
		return
	}
	p.margin = min(p.margin, refresh)
	p.hits = 0
	p.patience = min(2*p.patience, patienceCap)
}

// Stats reports the frames presented so far.
func (p *Pacer) Stats() Stats {
	s := p.stats
	s.Margin = p.margin
	return s
}

func (p *Pacer) hit() {
	p.hits++
	next := p.margin - p.step()
	need := probeAfter
	below := p.edge > 0 && next < p.edge+p.guard()
	if below {
		need = p.patience
	}
	if p.hits < need || next <= 0 {
		return
	}
	p.hits = 0
	p.margin = next
	if below {
		p.edge = 0
	}
}

func (p *Pacer) tight(f Frame) bool {
	return f.target.Sub(f.committed) <= f.margin+p.refresh/4
}

func (p *Pacer) retune(refresh time.Duration) {
	*p = Pacer{
		refresh: refresh, vblank: p.vblank,
		margin: refresh/2 + refresh/32, patience: patienceStart,
		costs: p.costs, nextCost: p.nextCost, stats: p.stats,
	}
}

func (p *Pacer) step() time.Duration { return p.refresh / 32 }

func (p *Pacer) guard() time.Duration { return p.refresh / 16 }

func (p *Pacer) live(now time.Time) bool {
	return p.refresh > 0 && !p.vblank.IsZero() && now.Sub(p.vblank) <= staleRefreshes*p.refresh
}

func (p *Pacer) cost() time.Duration {
	c := p.guard()
	for _, s := range p.costs {
		c = max(c, s+p.guard())
	}
	return c
}

func (p *Pacer) vblankAfter(t time.Time) time.Time {
	d := t.Sub(p.vblank)
	if d < 0 {
		return p.vblank
	}
	return p.vblank.Add((d/p.refresh + 1) * p.refresh)
}

func (p *Pacer) vblankFrom(t time.Time) time.Time {
	d := max(t.Sub(p.vblank), 1)
	return p.vblank.Add((d + p.refresh - 1) / p.refresh * p.refresh)
}
