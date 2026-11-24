package gpu

import (
	"errors"
	"runtime"
	"sync"
)

func currentG() uintptr

type pinnedThread struct {
	compiling func()
}

var pinned = struct {
	sync.Mutex
	threads map[uintptr]*pinnedThread
}{threads: map[uintptr]*pinnedThread{}}

func PinThread() (unpin func()) {
	runtime.LockOSThread()
	g := currentG()
	pinned.Lock()
	_, nested := pinned.threads[g]
	if !nested {
		pinned.threads[g] = &pinnedThread{}
	}
	pinned.Unlock()
	return func() {
		defer runtime.UnlockOSThread()
		if nested {
			return
		}
		pinned.Lock()
		delete(pinned.threads, g)
		pinned.Unlock()
	}
}

func pinnedHere() *pinnedThread {
	g := currentG()
	pinned.Lock()
	defer pinned.Unlock()
	return pinned.threads[g]
}

func CompileThrough(leaveRealtime func()) error {
	t := pinnedHere()
	if t == nil {
		return errors.New("gpu: CompileThrough on a goroutine that pinned no thread")
	}
	t.compiling = leaveRealtime
	return nil
}

func Compiling() {
	if t := pinnedHere(); t != nil && t.compiling != nil {
		t.compiling()
	}
}
