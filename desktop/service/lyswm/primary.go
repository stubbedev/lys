package lyswm

import (
	"encoding/json"
	"fmt"
	"slices"
	"sync"

	"github.com/stubbedev/lys/lib/wmipc"
)

// PrimaryWatcher follows the output lys-wm marks primary: seeded from the
// outputs request once the stream is open, then kept by the output events,
// ticking after each.
type PrimaryWatcher struct {
	*Watcher
	mu      sync.Mutex
	primary string
}

// WatchPrimary opens the event stream and follows the primary output.
func WatchPrimary() (*PrimaryWatcher, error) {
	pw := &PrimaryWatcher{}
	seed := func() error {
		outputs, err := Outputs()
		if err != nil {
			return fmt.Errorf("seed the primary output: %w", err)
		}
		if i := slices.IndexFunc(outputs, func(o wmipc.Output) bool { return o.Primary }); i >= 0 {
			pw.primary = outputs[i].Name
		}
		return nil
	}
	w, err := watch([]wmipc.EventKind{wmipc.EventOutput}, seed, pw.apply, nil, nil)
	if err != nil {
		return nil, err
	}
	pw.Watcher = w
	return pw, nil
}

// Primary is the connector of the primary output as the last event left
// it; empty while lys-wm marks none.
func (w *PrimaryWatcher) Primary() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.primary
}

func (w *PrimaryWatcher) apply(e event) {
	if e.Event != wmipc.EventOutput {
		return
	}
	var o wmipc.Output
	if json.Unmarshal(e.Output, &o) != nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	switch {
	case o.Primary && e.Change != wmipc.OutputRemoved:
		w.primary = o.Name
	case w.primary == o.Name:
		w.primary = ""
	}
}
