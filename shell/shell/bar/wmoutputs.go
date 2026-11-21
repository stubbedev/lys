package bar

import (
	"log"
	"sync/atomic"

	"github.com/stubbedev/lys/desktop/service/lyswm"
	"github.com/stubbedev/lys/ui/app"

	"github.com/stubbedev/lys/shell/shell/layering"
)

// wmOutputs is lys-wm's view of the outputs, followed once its watch is
// open: the output it marks primary and those it reports as OLED. Until
// then, without lys-wm, or while lys-wm marks none, it names no primary
// output and "primary" stays the first output listed.
type wmOutputs struct {
	watcher atomic.Pointer[lyswm.OutputsWatcher]
}

// follow opens the watch and passes oled the OLED outputs now and after
// every output event; it blocks on lys-wm, so it runs off the UI loop.
func (p *wmOutputs) follow(oled func([]string)) (stop func()) {
	w, err := lyswm.WatchOutputs()
	if err != nil {
		log.Printf("lys-shell: lys-wm outputs: %v; \"primary\" is the first output and no wallpaper dims for OLED", err)
		return func() {}
	}
	p.watcher.Store(w)
	oled(w.OLED())
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range w.Ticks() {
			oled(w.OLED())
		}
	}()
	return func() {
		w.Close()
		<-done
	}
}

func (p *wmOutputs) name() string {
	w := p.watcher.Load()
	if w == nil {
		return ""
	}
	return w.Primary()
}

// first lists outputs' outputs with the primary first.
func (p *wmOutputs) first(outputs func() []*app.Output) func() []*app.Output {
	return func() []*app.Output { return layering.PrimaryFirst(outputs(), p.name()) }
}
