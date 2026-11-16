package widget

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/internal/waittest"
)

type testLoop struct {
	work   chan func()
	exited chan struct{}
	mu     sync.Mutex
	queued []func()
	end    func()
}

func newTestLoop(t *testing.T) *testLoop {
	t.Helper()
	l := &testLoop{work: make(chan func()), exited: make(chan struct{})}
	ready := make(chan func())
	go func() {
		defer close(l.exited)
		unmark := MarkLoop()
		defer unmark()
		ready <- SetInvoker(l.post)
		for fn := range l.work {
			fn()
		}
	}()
	l.end = <-ready
	t.Cleanup(func() {
		l.end()
		close(l.work)
		waittest.For(t, l.exited, "a test loop to exit")
	})
	return l
}

func (l *testLoop) post(fn func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.queued = append(l.queued, fn)
}

func (l *testLoop) do(t *testing.T, fn func()) {
	t.Helper()
	ran := make(chan struct{})
	l.work <- func() {
		defer close(ran)
		fn()
	}
	waittest.For(t, ran, "work on a test loop")
}

func (l *testLoop) pump(t *testing.T) int {
	t.Helper()
	l.mu.Lock()
	fns := l.queued
	l.queued = nil
	l.mu.Unlock()
	l.do(t, func() {
		for _, fn := range fns {
			fn()
		}
	})
	return len(fns)
}

func adoptLoop(l *eventLoop) (drop func()) {
	id := eventLoops.bind(l)
	return func() { eventLoops.unbind(id, l) }
}

func holdAssetWorkers(t *testing.T) (release func()) {
	t.Helper()
	holder := newTestLoop(t)
	hold := make(chan struct{})
	holder.do(t, func() {
		for i := range maxAssetWorkers {
			loadAsset(heldWorker(i), currentLoop(), func() { <-hold }, func() {})
		}
	})
	released := false
	release = func() {
		if !released {
			released = true
			close(hold)
		}
	}
	t.Cleanup(func() {
		release()
		onTime(t, "the held workers", WaitAssetLoads)
	})
	return release
}

type heldWorker int

func TestAnInvokerServesOnlyTheGoroutineThatInstalledIt(t *testing.T) {
	t.Cleanup(func() { SetInvoker(nil) })
	SetInvoker(func(fn func()) { fn() })
	if currentLoop() == nil {
		t.Fatal("the goroutine that installed an invoker has none")
	}
	elsewhere := make(chan *eventLoop)
	go func() { elsewhere <- currentLoop() }()
	if l := <-elsewhere; l != nil {
		t.Error("another goroutine answers through an invoker it never installed")
	}
	SetInvoker(nil)
	if currentLoop() != nil {
		t.Error("installing none left the goroutine's invoker in place")
	}
}

type twoLoopTree struct {
	loop  *testLoop
	clock anim.Clock
	root  *Box
}

func (tr *twoLoopTree) owed(t *testing.T) int {
	t.Helper()
	n := 0
	tr.loop.do(t, func() { n = TreeAssetsOwed(tr.root) })
	return n
}

func TestTwoLoopsAskingForOneIconAndImageEachHearTheirOwnAnswer(t *testing.T) {
	dir := t.TempDir()
	iconPath := filepath.Join(dir, "shared.svg")
	if err := os.WriteFile(iconPath, []byte(testGreenSVG), 0o600); err != nil {
		t.Fatal(err)
	}
	imagePath := filepath.Join(dir, "shared.png")
	if err := os.WriteFile(imagePath, []byte(redPNG(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	addCSS(t, bytesReplaceOne(`.shared { background-image: url("%s"); }`, imagePath), 900)
	var offLoop atomic.Int64
	SetOffLoopHook(func(string) { offLoop.Add(1) })
	t.Cleanup(func() { SetOffLoopHook(nil) })
	release := holdAssetWorkers(t)

	trees := [2]*twoLoopTree{{loop: newTestLoop(t)}, {loop: newTestLoop(t)}}
	for _, tr := range trees {
		tr.loop.do(t, func() {
			tr.root = NewBox(Row, 0, 0)
			tr.root.AddClass("shared")
			tr.root.Append(NewThemeIcon(iconPath, 16), false)
			unhost := Host(tr.root, &tr.clock)
			t.Cleanup(func() { tr.loop.do(t, unhost) })
			SizeWithin(tr.root, Size{W: 30, H: 30})
			tr.root.Arrange(render.Rect{W: 30, H: 30})
			paintTree(tr.root, 30, 30)
		})
		if n := tr.owed(t); n != 2 {
			t.Fatalf("a tree showing the icon and the image owes %d answers, want 2", n)
		}
	}
	release()
	onTime(t, "the shared loads", WaitAssetLoads)

	for i := len(trees) - 1; i >= 0; i-- {
		tr := trees[i]
		if n := tr.loop.pump(t); n == 0 {
			t.Errorf("loop %d heard no answer", i)
		}
		if n := tr.owed(t); n != 0 {
			t.Errorf("after loop %d ran what it heard, its tree still owes %d answers", i, n)
		}
	}
	if n := offLoop.Load(); n != 0 {
		t.Errorf("%d widget mutations ran off every loop", n)
	}
}

func TestEndingALoopDropsItsLoadsThatHaveNotStartedAndNoOthers(t *testing.T) {
	release := holdAssetWorkers(t)
	ending, staying := newTestLoop(t), newTestLoop(t)
	var endingRan, sharedRan, stayingRan atomic.Bool
	ending.do(t, func() {
		loadAsset("ending", currentLoop(), func() { endingRan.Store(true) }, func() {})
		loadAsset("shared", currentLoop(), func() { sharedRan.Store(true) }, func() {})
	})
	answered := 0
	staying.do(t, func() {
		loadAsset("shared", currentLoop(), func() {}, func() { answered++ })
		loadAsset("staying", currentLoop(), func() { stayingRan.Store(true) }, func() { answered++ })
	})
	ending.end()
	release()
	onTime(t, "the queue", WaitAssetLoads)
	if endingRan.Load() {
		t.Error("a load only the ended loop asked for ran after it ended")
	}
	if !sharedRan.Load() || !stayingRan.Load() {
		t.Errorf("loads another loop still waits for: shared ran %v, its own ran %v", sharedRan.Load(), stayingRan.Load())
	}
	if ending.pump(t) != 0 {
		t.Error("the ended loop heard an answer")
	}
	staying.pump(t)
	if answered != 2 {
		t.Errorf("the loop still running heard %d answers, want 2", answered)
	}
}

func TestAStyleChangeOnOneLoopQueuesOnePassOnEveryOther(t *testing.T) {
	changing, other := newTestLoop(t), newTestLoop(t)
	rem := RootFontSize()
	t.Cleanup(func() { SetRootFontSize(rem) })
	var sheet *Stylesheet
	changing.do(t, func() {
		sheet = AddStylesheet(".wake-a { color: red; }", 0)
		AddStylesheet(".wake-b { color: blue; }", 0).Remove()
		SetRootFontSize(rem + 1)
	})
	t.Cleanup(sheet.Remove)
	gen := styleGen.Load()
	if n := other.pump(t); n != 1 {
		t.Errorf("three style changes on another loop queued %d passes here, want 1", n)
	}
	if styleGen.Load() != gen {
		t.Error("the queued pass changed the style generation itself")
	}
	if n := changing.pump(t); n != 0 {
		t.Errorf("the loop that changed the style queued %d passes for itself, want none", n)
	}
	if n := other.pump(t); n != 0 {
		t.Errorf("with no change since its pass the other loop has %d more queued, want none", n)
	}

	changing.do(t, sheet.Remove)
	if n := other.pump(t); n != 1 {
		t.Errorf("a change after the other loop's pass queued %d passes there, want 1", n)
	}
}

func TestAFontAnswerQueuesOnePassOnEachLoopAndNoMore(t *testing.T) {
	loops := [2]*testLoop{newTestLoop(t), newTestLoop(t)}
	gen := styleGen.Load()
	FontFaceResolved()
	for i, l := range loops {
		if n := l.pump(t); n != 1 {
			t.Errorf("loop %d heard the answer through %d passes, want 1", i, n)
		}
	}
	for i, l := range loops {
		if n := l.pump(t); n != 0 {
			t.Errorf("loop %d has %d more passes queued after the answer, want none", i, n)
		}
	}
	if got := styleGen.Load() - gen; got != 2 {
		t.Errorf("the answer bumped the style %d times over two loops, want 2", got)
	}
}
