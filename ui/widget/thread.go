package widget

import (
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
)

// The loop-goroutine guard samples lys-ui's threading contract (see
// docs/threading.md): widget state belongs to the event-loop
// goroutine, and every other goroutine reaches the tree only through
// app.Invoke. The guard is a debugging aid, not a lock. Run arms it on
// the loop goroutine; the sampled mutation entry points below call
// checkLoop, which fires the hook installed by SetOffLoopHook when a
// mutation comes from a goroutine no mark names. Sampling keeps the cost at
// one atomic load per call while disarmed and one goroutine-id parse
// per mutation while armed - mutations happen on input and state
// changes, never per frame.

var loopGuard struct {
	armed atomic.Bool
	mu    sync.Mutex
	loops map[uint64]int
	hook  func(what string)
}

// MarkLoop records the calling goroutine as an event-loop goroutine and
// arms the guard: a sampled widget mutation from a goroutine no mark
// names now trips the off-loop hook. Every Application's Run marks its
// own goroutine, so applications sharing a process each keep theirs;
// tests can mark whichever goroutine they drive the tree on. unmark
// drops this mark, and the guard disarms once no mark is left, so
// post-Run teardown may touch widgets from any goroutine again.
func MarkLoop() (unmark func()) {
	id := goroutineID()
	loopGuard.mu.Lock()
	if loopGuard.loops == nil {
		loopGuard.loops = map[uint64]int{}
	}
	loopGuard.loops[id]++
	loopGuard.armed.Store(true)
	loopGuard.mu.Unlock()
	return sync.OnceFunc(func() {
		loopGuard.mu.Lock()
		defer loopGuard.mu.Unlock()
		if loopGuard.loops[id]--; loopGuard.loops[id] == 0 {
			delete(loopGuard.loops, id)
		}
		loopGuard.armed.Store(len(loopGuard.loops) > 0)
	})
}

// SetOffLoopHook installs the debug hook fired with the sampled entry
// point's name when a widget mutation runs off every loop goroutine;
// nil disables. The hook runs on the offending goroutine and must not
// touch widgets. Production code leaves it nil; tests and lysdebug
// builds use it to enforce the threading contract.
func SetOffLoopHook(hook func(what string)) {
	loopGuard.mu.Lock()
	loopGuard.hook = hook
	loopGuard.mu.Unlock()
}

// checkLoop fires the off-loop hook when the guard is armed, a hook is
// installed, and the caller is no marked loop goroutine.
func checkLoop(what string) {
	if !loopGuard.armed.Load() {
		return
	}
	loopGuard.mu.Lock()
	hook := loopGuard.hook
	offLoop := hook != nil && loopGuard.loops[goroutineID()] == 0
	loopGuard.mu.Unlock()
	if offLoop {
		hook(what)
	}
}

// goroutineID extracts the calling goroutine's id from a stack dump;
// the first line is always "goroutine N [state]:". There is no
// sanctioned runtime API for goroutine ids; the parse happens only
// while the guard is armed.
func goroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	s := buf[:n]
	const prefix = "goroutine "
	if len(s) <= len(prefix) {
		return 0
	}
	s = s[len(prefix):]
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	id, _ := strconv.ParseUint(string(s[:end]), 10, 64)
	return id
}
