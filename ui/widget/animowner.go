package widget

import (
	"slices"
	"sync"
	"weak"

	"github.com/stubbedev/lys/ui/anim"
)

type hosting struct{ clock *anim.Clock }

type treeTop struct {
	key     any
	host    *hosting
	waiting []weak.Pointer[node]
}

var (
	nodelessTops = struct {
		mu   sync.Mutex
		tops map[any]*treeTop
	}{tops: map[any]*treeTop{}}
	hostedTops = struct {
		mu      sync.Mutex
		byClock map[*anim.Clock][]*treeTop
	}{byClock: map[*anim.Clock][]*treeTop{}}
)

// Host marks root as the top of a tree a host shows on screen, a
// window's, a popover's or a toast's, whose animations run on clock,
// and returns the call that ends it. Only widgets under a hosted root
// animate: a subtree removed from its tree, a tree whose window closed,
// or one a test arranged and dropped schedules no frames, and what it
// launches waits until its tree is hosted again. Hosting a root again
// moves it to the later host; ending the earlier one leaves it there.
func Host(root Widget, clock *anim.Clock) (unhost func()) {
	checkLoop("Host")
	if clock == nil {
		panic("widget: Host: nil clock")
	}
	t := ensureTop(hostKey(root))
	if t.host != nil {
		unlistHosted(t)
	}
	h := &hosting{clock: clock}
	t.host = h
	hostedTops.mu.Lock()
	hostedTops.byClock[clock] = append(hostedTops.byClock[clock], t)
	hostedTops.mu.Unlock()
	return func() {
		if t.host != h {
			return
		}
		unlistHosted(t)
		t.host = nil
		dropIfIdle(t)
	}
}

func unlistHosted(t *treeTop) {
	hostedTops.mu.Lock()
	defer hostedTops.mu.Unlock()
	c := t.host.clock
	tops := slices.DeleteFunc(hostedTops.byClock[c], func(o *treeTop) bool { return o == t })
	if len(tops) == 0 {
		delete(hostedTops.byClock, c)
		return
	}
	hostedTops.byClock[c] = tops
}

func hostKey(w Widget) any {
	if n := nodeOf(w); n != nil {
		return n
	}
	return w
}

func topKey(n *node) any {
	for {
		p := n.parent
		if p == nil {
			return n
		}
		for nodeOf(p) == nil {
			up := parentOf(p)
			if up == nil {
				return p
			}
			p = up
		}
		n = nodeOf(p)
	}
}

func lookupTop(key any) *treeTop {
	if n, ok := key.(*node); ok {
		return n.top
	}
	nodelessTops.mu.Lock()
	defer nodelessTops.mu.Unlock()
	return nodelessTops.tops[key]
}

func ensureTop(key any) *treeTop {
	if n, ok := key.(*node); ok {
		if n.top == nil {
			n.top = &treeTop{key: n}
		}
		return n.top
	}
	nodelessTops.mu.Lock()
	defer nodelessTops.mu.Unlock()
	t := nodelessTops.tops[key]
	if t == nil {
		t = &treeTop{key: key}
		nodelessTops.tops[key] = t
	}
	return t
}

func dropIfIdle(t *treeTop) {
	if t.host != nil || len(t.waiting) > 0 {
		return
	}
	if n, ok := t.key.(*node); ok {
		if n.top == t {
			n.top = nil
		}
		return
	}
	nodelessTops.mu.Lock()
	defer nodelessTops.mu.Unlock()
	if nodelessTops.tops[t.key] == t {
		delete(nodelessTops.tops, t.key)
	}
}

func (n *node) hostClock() *anim.Clock {
	if t := lookupTop(topKey(n)); t != nil && t.host != nil {
		return t.host.clock
	}
	return nil
}

func onScreen(w Widget) bool {
	n := nodeOf(w)
	if n == nil || n.bounds.Empty() {
		return false
	}
	for a := w; a != nil; a = parentOf(a) {
		if !ownVisible(a) {
			return false
		}
	}
	return n.hostClock() != nil
}

// ScheduleAnimation starts l on the clock of the host whose tree holds
// the widget, now or, while no hosted tree holds it, once one does
// (AttachAnimations): anim.Owner.
func (n *node) ScheduleAnimation(l *anim.Launch) {
	key := topKey(n)
	t := lookupTop(key)
	if t != nil && t.host != nil {
		l.Start(t.host.clock)
		return
	}
	if len(n.awaiting) == 0 {
		t = ensureTop(key)
		t.waiting = append(t.waiting, weak.Make(n))
	}
	n.awaiting = append(slices.DeleteFunc(n.awaiting, (*anim.Launch).Cancelled), l)
}

// AttachAnimations starts what widgets launched while no hosted tree held
// them, for each widget a tree hosted on clock holds now, on clock. Each
// application's loop calls it with its own clock once its windows have
// arranged, so it touches only the trees that application hosts; a test
// that hosts a tree and arranges it calls it before it ticks.
func AttachAnimations(clock *anim.Clock) {
	checkLoop("AttachAnimations")
	hostedTops.mu.Lock()
	tops := hostedTops.byClock[clock]
	hostedTops.mu.Unlock()
	for _, t := range tops {
		t.settle(clock)
	}
}

func (t *treeTop) settle(startOn *anim.Clock) {
	kept := t.waiting[:0]
	for _, wp := range t.waiting {
		n := wp.Value()
		if n == nil {
			continue
		}
		n.awaiting = slices.DeleteFunc(n.awaiting, (*anim.Launch).Cancelled)
		if len(n.awaiting) == 0 {
			continue
		}
		if key := topKey(n); key != t.key {
			moved := ensureTop(key)
			moved.waiting = append(moved.waiting, wp)
			continue
		}
		if startOn == nil {
			kept = append(kept, wp)
			continue
		}
		for _, l := range n.awaiting {
			l.Start(startOn)
		}
		n.awaiting = nil
	}
	clear(t.waiting[len(kept):])
	t.waiting = kept
}

func waitingTopMoving(w, parent Widget) *treeTop {
	n := nodeOf(w)
	if n == nil || n.parent == parent {
		return nil
	}
	if t := lookupTop(topKey(n)); t != nil && len(t.waiting) > 0 {
		return t
	}
	return nil
}

func refileWaiting(t *treeTop) {
	if t == nil {
		return
	}
	t.settle(nil)
	dropIfIdle(t)
}
