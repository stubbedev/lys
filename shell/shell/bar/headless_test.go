package bar

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/ui/widget"
)

// onHeadlessLoop reads widget state the way the loop would: under
// headlessLoop, which every headless Invoke holds while it mutates.
func onHeadlessLoop[T any](read func() T) T {
	headlessLoop.Lock()
	defer headlessLoop.Unlock()
	return read()
}

func firstIn[T widget.Widget](root widget.Widget) (T, error) {
	all := treetest.All[T](root)
	if len(all) == 0 {
		var zero T
		return zero, fmt.Errorf("no %T in the tree", zero)
	}
	return all[0], nil
}

func createOnHeadlessLoop(name string, ctx ModuleContext) (Module, error) {
	return onHeadlessLoop(func() moduleResult { return createResult(name, ctx) }).unpack()
}

func buildRootOnHeadlessLoop(ctx ModuleContext, layout config.BarLayout, connector string) *widget.Bar {
	return onHeadlessLoop(func() *widget.Bar { return buildRoot(ctx, layout, connector) })
}

func newCwsModuleOnHeadlessLoop(ctx ModuleContext, kind string, flavor barview.WorkspaceFlavor, cfg config.CompositorWorkspacesConfig, backend cwsBackend) *cwsModule {
	return onHeadlessLoop(func() *cwsModule { return newCwsModule(ctx, kind, flavor, cfg, backend) })
}

// waitHeadless polls cond on the headless loop until it holds
// (followers deliver from their own goroutines).
func waitHeadless(t testing.TB, what string, cond func() bool) {
	t.Helper()
	waittest.Until(t, func() bool { return onHeadlessLoop(cond) }, what)
}

func untilNil(t testing.TB, what string, cond func() error) {
	t.Helper()
	var last error
	defer func() {
		if last != nil {
			t.Logf("%s: %v", what, last)
		}
	}()
	waittest.Until(t, func() bool { last = cond(); return last == nil }, what)
}

// waitForText waits until the label carries want.
func waitForText(t *testing.T, label *widget.Label, want string) {
	t.Helper()
	waitHeadless(t, "label "+want, func() bool { return label.Text() == want })
}

// Work waiting for the loop when its generation retires is dropped:
// the retirement is read under the loop lock, as the application loop
// reads it inside the loop.
func TestHeadlessInvokeDropsWorkRetiredWhileQueued(t *testing.T) {
	ctx := ModuleContext{gen: newMountGen()}
	var ran atomic.Bool
	done := make(chan struct{})
	headlessLoop.Lock()
	go func() {
		ctx.Invoke(func() { ran.Store(true) })
		close(done)
	}()
	time.Sleep(20 * time.Millisecond) // let the call queue on the lock
	ctx.gen.retire()
	headlessLoop.Unlock()
	<-done
	if ran.Load() {
		t.Fatal("work queued before the retirement ran after it")
	}
}

// Headless Invoke runs the work inline and serialized; a retired
// generation's work is dropped.
func TestHeadlessInvokeRunsInlineUntilRetired(t *testing.T) {
	ctx := ModuleContext{gen: newMountGen()}
	ran := false
	ctx.Invoke(func() { ran = true })
	if !ran {
		t.Fatal("headless Invoke did not run the work")
	}
	ctx.gen.retire()
	ran = false
	ctx.Invoke(func() { ran = true })
	if ran {
		t.Fatal("headless Invoke ran work for a retired generation")
	}
}

// findByClass walks a tree for the first widget carrying class.
func findByClass(w widget.Widget, class string) widget.Widget {
	if c, ok := w.(interface{ HasClass(string) bool }); ok && c.HasClass(class) {
		return w
	}
	if box, ok := w.(interface{ Children() []widget.Widget }); ok {
		for _, kid := range box.Children() {
			if hit := findByClass(kid, class); hit != nil {
				return hit
			}
		}
	}
	return nil
}

// routeClick presses and releases at the center of w through a router
// over root, which must already be arranged.
func routeClick(root widget.Widget, w widget.Widget) {
	b := w.(interface{ Bounds() render.Rect }).Bounds()
	p := widget.Point{X: b.X + b.W/2, Y: b.Y + b.H/2}
	r := &widget.Router{Root: root}
	r.Move(p)
	r.Press(widget.BTNLeft, p, 0)
	r.Release(widget.BTNLeft, p)
}

func readAndApply[T any](read func(context.Context) (T, error), apply func(T)) error {
	v, err := read(context.Background())
	if err != nil {
		return err
	}
	apply(v)
	return nil
}
