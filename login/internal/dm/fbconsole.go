package dm

import (
	"log"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/vtconsole"
)

const (
	takeoverPoll = 20 * time.Millisecond
	takeoverWait = 30 * time.Second
)

type framebufferConsole struct {
	unbind func() ([]string, error)
	bind   func([]string) error

	mu      sync.Mutex
	unbound []string
	release uint64
}

func newFramebufferConsole(dir string) *framebufferConsole {
	return &framebufferConsole{
		unbind: func() ([]string, error) { return vtconsole.UnbindFramebuffer(dir) },
		bind:   func(entries []string) error { return vtconsole.Bind(dir, entries) },
	}
}

func (f *framebufferConsole) handOff() uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.release++
	unbound, err := f.unbind()
	f.unbound = append(f.unbound, unbound...)
	if err != nil {
		log.Printf("dm: unbind the framebuffer console before the greeter's compositor lets go of the screen: %v; logind's switch of the VT to text mode puts the console on screen until the session's compositor takes it", err)
		return f.release
	}
	if len(unbound) > 0 {
		log.Printf("dm: unbound the framebuffer console %v, so the greeter's last frame stays on screen until the session's compositor replaces it", unbound)
	}
	return f.release
}

func (f *framebufferConsole) restore(why string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bindAgain(why)
}

func (f *framebufferConsole) restoreAfter(release uint64, why string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if release != f.release {
		return
	}
	f.bindAgain(why)
}

func (f *framebufferConsole) bindAgain(why string) {
	if len(f.unbound) == 0 {
		return
	}
	entries := f.unbound
	f.unbound = nil
	if err := f.bind(entries); err != nil {
		log.Printf("dm: bind the framebuffer console %v again: %v", entries, err)
		return
	}
	log.Printf("dm: bound the framebuffer console %v again: %s", entries, why)
}

func (f *framebufferConsole) awaitTakeover(release uint64, left scanouts, read func() (scanouts, error), clock clock, done <-chan struct{}) {
	deadline := clock.after(takeoverWait)
	for {
		now, err := read()
		switch {
		case err != nil:
			f.restoreAfter(release, "reading what the screen shows failed: "+err.Error())
			return
		case left.replacedIn(now):
			f.restoreAfter(release, "a compositor replaced the greeter's last frame")
			return
		}
		select {
		case <-done:
			return
		case <-deadline:
			f.restoreAfter(release, "no compositor replaced the greeter's last frame within "+takeoverWait.String())
			return
		case <-clock.after(takeoverPoll):
		}
	}
}
