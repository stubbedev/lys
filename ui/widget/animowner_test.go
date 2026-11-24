package widget

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/anim"
)

func TestAWidgetsAnimationWaitsForAHostedTreeAndRunsOnItsClock(t *testing.T) {
	c := pinAnimClock(t)
	root := NewBox(Column, 0, 0)
	c.host(t, root)
	late := NewBox(Row, 0, 0)
	root.Append(late, false)
	landed := 0.0
	anim.Play(late, anim.Animate(100*time.Millisecond, func(p float64) { landed = p }).Easing(anim.Linear))
	if c.active() {
		t.Fatal("a widget no hosted tree holds yet launched onto the host's clock")
	}
	SizeWithin(root, Size{W: 100, H: 100})
	root.Arrange(render.Rect{W: 100, H: 100})
	AttachAnimations(&c.clock)
	if !c.active() {
		t.Fatal("the widget's animation did not start once its tree was hosted and laid out")
	}
	c.drive()
	if landed != 1 {
		t.Errorf("the attached animation stands at %v, want it landed", landed)
	}
}

func TestAnUnhostedTreesLaunchNeverRunsAndACancelledOneNeverStarts(t *testing.T) {
	c := pinAnimClock(t)
	root := NewBox(Column, 0, 0)
	leaf := NewBox(Row, 0, 0)
	root.Append(leaf, false)
	unhost := Host(root, &c.clock)
	SizeWithin(root, Size{W: 100, H: 100})
	root.Arrange(render.Rect{W: 100, H: 100})
	unhost()
	waited, cancelled := 0.0, 0
	anim.Start(leaf, time.Second, func(p float64) { waited = p })
	cancel := anim.Start(root, time.Second, func(float64) { cancelled++ })
	cancel()
	AttachAnimations(&c.clock)
	if c.active() {
		t.Fatal("a tree whose host ended launched onto the clock")
	}
	c.host(t, root)
	AttachAnimations(&c.clock)
	c.drive()
	if waited != 1 {
		t.Errorf("the launch waiting on the unhosted tree stands at %v once it was hosted again, want it landed", waited)
	}
	if cancelled != 0 {
		t.Errorf("the launch cancelled while it waited ran %d frames", cancelled)
	}
}

func TestAWidgetUnderANodelessRootRunsOnThatRootsHost(t *testing.T) {
	c := pinAnimClock(t)
	leaf := NewBox(Row, 0, 0)
	root := &nodelessRoot{child: leaf}
	SetParents(root, leaf)
	c.host(t, root)
	anim.Start(leaf, time.Second, func(float64) {})
	if !c.active() {
		t.Error("a widget under a hosted root without a node did not launch onto its clock")
	}
}

type nodelessRoot struct {
	Nodeless
	child Widget
}

func (r *nodelessRoot) Measure(axis Axis, forSize int) Sizes { return r.child.Measure(axis, forSize) }
func (r *nodelessRoot) Arrange(rect render.Rect)             { r.child.Arrange(rect) }
func (r *nodelessRoot) Paint(cv *render.Canvas)              { r.child.Paint(cv) }
func (r *nodelessRoot) HitTest(p Point) Widget               { return r.child.HitTest(p) }

func TestAnAttachOnOneClockLeavesAnotherClocksTreesWaiting(t *testing.T) {
	c := pinAnimClock(t)
	var other anim.Clock
	roots := [2]*Box{NewBox(Column, 0, 0), NewBox(Column, 0, 0)}
	landed := [2]float64{}
	for i, root := range roots {
		anim.Start(root, time.Second, func(p float64) { landed[i] = p })
	}
	c.host(t, roots[0])
	t.Cleanup(Host(roots[1], &other))
	AttachAnimations(&c.clock)
	if !c.active() {
		t.Fatal("attaching the first clock started nothing on it")
	}
	if other.Active() {
		t.Fatal("attaching the first clock started the launch of a tree hosted on another clock")
	}
	c.drive()
	if landed != [2]float64{1, 0} {
		t.Fatalf("after driving the first clock the launches stand at %v, want only the first landed", landed)
	}
	AttachAnimations(&other)
	if !other.Active() {
		t.Error("attaching the second clock did not start its tree's waiting launch")
	}
}

func TestALaunchWaitingInAMovedSubtreeFollowsItIntoAHostedTree(t *testing.T) {
	c := pinAnimClock(t)
	unhosted := NewBox(Column, 0, 0)
	branch := NewBox(Column, 0, 0)
	leaf := NewBox(Row, 0, 0)
	branch.Append(leaf, false)
	unhosted.Append(branch, false)
	layout(unhosted, 100, 100)
	landed := 0.0
	anim.Start(leaf, time.Second, func(p float64) { landed = p })
	unhosted.Remove(branch)
	hosted := NewBox(Column, 0, 0)
	c.host(t, hosted)
	layout(hosted, 100, 100)
	AttachAnimations(&c.clock)
	if c.active() {
		t.Fatal("a launch in a subtree no hosted tree holds started")
	}
	hosted.Append(branch, false)
	layout(hosted, 100, 100)
	AttachAnimations(&c.clock)
	c.drive()
	if landed != 1 {
		t.Errorf("the launch that waited in the moved subtree stands at %v, want it landed", landed)
	}
	if unhosted.top != nil {
		t.Error("the tree the waiting subtree left still keeps its waiting list")
	}
}

func TestAWidgetUnderANodelessRootWaitsForThatRootsHost(t *testing.T) {
	c := pinAnimClock(t)
	leaf := NewBox(Row, 0, 0)
	root := &nodelessRoot{child: leaf}
	SetParents(root, leaf)
	landed := 0.0
	anim.Start(leaf, time.Second, func(p float64) { landed = p })
	if c.active() {
		t.Fatal("a launch under an unhosted nodeless root started")
	}
	unhost := Host(root, &c.clock)
	AttachAnimations(&c.clock)
	c.drive()
	if landed != 1 {
		t.Errorf("the launch under the nodeless root stands at %v once it was hosted, want it landed", landed)
	}
	unhost()
	nodelessTops.mu.Lock()
	_, kept := nodelessTops.tops[root]
	nodelessTops.mu.Unlock()
	if kept {
		t.Error("an unhosted nodeless root with nothing waiting is still kept")
	}
}

func TestTwoApplicationsAttachTheirTreesLaunchesAtOnce(t *testing.T) {
	const rounds = 40
	atOnce(t, 2, func(int) error {
		var clock anim.Clock
		root := NewBox(Column, 0, 0)
		defer Host(root, &clock)()
		var errs []error
		for i := range rounds {
			branch := NewBox(Column, 0, 0)
			leaf := NewBox(Row, 0, 0)
			branch.Append(leaf, false)
			landed := 0.0
			anim.Start(leaf, time.Millisecond, func(p float64) { landed = p })
			AttachAnimations(&clock)
			if clock.Active() {
				errs = append(errs, fmt.Errorf("round %d: a launch no hosted tree holds started", i))
			}
			root.Append(branch, false)
			layout(root, 100, 100)
			AttachAnimations(&clock)
			clock.Tick(time.Now().Add(time.Hour))
			if landed != 1 {
				errs = append(errs, fmt.Errorf("round %d: the launch attached with its subtree stands at %v, want it landed", i, landed))
			}
			root.Remove(branch)
		}
		return errors.Join(errs...)
	})
}
