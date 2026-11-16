package widget

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"golang.org/x/image/font/gofont/goregular"
)

// TestOffLoopMutationTripsHook enforces the threading contract's
// detection half (docs/threading.md): with the guard armed on the
// goroutine playing the loop, mutations on that goroutine are silent,
// and the same mutations from any other goroutine fire the off-loop
// hook. This is the test that fails when a callback touches widgets
// off-loop.
func TestOffLoopMutationTripsHook(t *testing.T) {
	defer SetOffLoopHook(nil)
	var hits atomic.Int32
	SetOffLoopHook(func(what string) { hits.Add(1) })

	face, err := render.LoadFont(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}

	unmark := MarkLoop()
	defer unmark()
	label := NewLabel(face, 14, "one", render.RGB(255, 255, 255))
	label.SetText("two")
	box := NewBox(Column, 4, 4)
	box.Append(label, false)
	box.Invalidate()
	if got := hits.Load(); got != 0 {
		t.Fatalf("on-loop mutations tripped the hook %d times", got)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		label.SetText("off loop") // the violation under test
	}()
	<-done
	if got := hits.Load(); got == 0 {
		t.Error("off-loop SetText was not detected; the guard missed a real violation")
	}

	// A second off-loop widget (InvalidateRect path) is caught too.
	before := hits.Load()
	off := NewLabel(face, 14, "x", render.RGB(255, 255, 255))
	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		off.InvalidateRect(render.Rect{X: 1, Y: 1, W: 3, H: 3})
	}()
	<-done2
	if hits.Load() == before {
		t.Error("off-loop InvalidateRect was not detected")
	}

	unmark()
	done3 := make(chan struct{})
	go func() {
		defer close(done3)
		label.SetText("teardown")
	}()
	<-done3
	if got := hits.Load(); got != before+1 {
		t.Errorf("disarmed guard still tripped (%d hits, want %d)", hits.Load(), before+1)
	}
}

func TestMarkLoopReArms(t *testing.T) {
	defer SetOffLoopHook(nil)
	var hits atomic.Int32
	SetOffLoopHook(func(string) { hits.Add(1) })

	face, err := render.LoadFont(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}

	defer MarkLoop()()
	entered := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer MarkLoop()()
		close(entered)
		label := NewLabel(face, 12, "new loop owns the tree", render.RGB(255, 255, 255))
		label.SetText("moved")
	}()
	<-entered
	if got := hits.Load(); got != 0 {
		t.Errorf("re-marked loop goroutine tripped the hook %d times", got)
	}
	<-done
}

func TestTwoLoopGoroutinesEachChangeTheirOwnTree(t *testing.T) {
	defer SetOffLoopHook(nil)
	var hits atomic.Int32
	SetOffLoopHook(func(string) { hits.Add(1) })
	const loops = 2
	var marked, changed, checked sync.WaitGroup
	marked.Add(loops)
	changed.Add(loops)
	checked.Add(1)
	var onLoops, offLoop int32
	atOnce(t, loops, func(w int) error {
		defer MarkLoop()()
		box := NewBox(Column, 0, 0)
		count := NewBinding(0)
		marked.Done()
		marked.Wait()
		for i := range 50 {
			count.Set(w*100 + i)
			box.Invalidate()
		}
		changed.Done()
		changed.Wait()
		if w != 0 {
			checked.Wait()
			return nil
		}
		defer checked.Done()
		onLoops = hits.Load()
		done := make(chan struct{})
		go func() {
			defer close(done)
			box.Invalidate()
		}()
		<-done
		offLoop = hits.Load() - onLoops
		return nil
	})
	if onLoops != 0 {
		t.Errorf("two marked loops changing their own trees tripped the hook %d times", onLoops)
	}
	if offLoop == 0 {
		t.Error("a mutation from a goroutine no loop marked, with both loops marked, did not trip the hook")
	}
}
