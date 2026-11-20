package bar

import (
	"log"
	"sync/atomic"

	"github.com/stubbedev/lys/desktop/service/lyswm"
	"github.com/stubbedev/lys/ui/app"

	"github.com/stubbedev/lys/shell/shell/layering"
)

// primaryOutput is the output lys-wm marks primary, followed once its
// watch is open. Until then, without lys-wm, or while lys-wm marks none,
// it names no output and "primary" stays the first output listed.
type primaryOutput struct {
	watcher atomic.Pointer[lyswm.PrimaryWatcher]
}

// follow opens the watch; it blocks on lys-wm, so it runs off the UI loop.
func (p *primaryOutput) follow() (stop func()) {
	w, err := lyswm.WatchPrimary()
	if err != nil {
		log.Printf("lys-shell: lys-wm primary output: %v; \"primary\" is the first output", err)
		return func() {}
	}
	p.watcher.Store(w)
	return w.Close
}

func (p *primaryOutput) name() string {
	w := p.watcher.Load()
	if w == nil {
		return ""
	}
	return w.Primary()
}

// first lists outputs' outputs with the primary first.
func (p *primaryOutput) first(outputs func() []*app.Output) func() []*app.Output {
	return func() []*app.Output { return layering.PrimaryFirst(outputs(), p.name()) }
}
