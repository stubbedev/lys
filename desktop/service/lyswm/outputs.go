package lyswm

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/stubbedev/lys/lib/wmipc"
)

// OutputsWatcher follows lys-wm's outputs: seeded from the outputs
// request once the stream is open, then kept by the output events,
// ticking after each.
type OutputsWatcher struct {
	*Watcher
	mu      sync.Mutex
	outputs map[string]wmipc.Output
	primary string
}

// WatchOutputs opens the event stream and follows the outputs.
func WatchOutputs() (*OutputsWatcher, error) {
	ow := &OutputsWatcher{outputs: map[string]wmipc.Output{}}
	seed := func() error {
		outputs, err := Outputs()
		if err != nil {
			return fmt.Errorf("seed the outputs: %w", err)
		}
		for _, o := range outputs {
			ow.outputs[o.Name] = o
			if o.Primary {
				ow.primary = o.Name
			}
		}
		return nil
	}
	w, err := watch([]wmipc.EventKind{wmipc.EventOutput}, seed, ow.apply, nil, nil)
	if err != nil {
		return nil, err
	}
	ow.Watcher = w
	return ow, nil
}

// Primary is the connector of the primary output as the last event left
// it; empty while lys-wm marks none.
func (w *OutputsWatcher) Primary() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.primary
}

// OLED is the connectors of the outputs lys-wm reports as OLED panels,
// sorted.
func (w *OutputsWatcher) OLED() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	var names []string
	for _, name := range slices.Sorted(maps.Keys(w.outputs)) {
		if c := w.outputs[name].Capabilities; c != nil && c.OLED {
			names = append(names, name)
		}
	}
	return names
}

func (w *OutputsWatcher) apply(e event) {
	if e.Event != wmipc.EventOutput {
		return
	}
	var o wmipc.Output
	if json.Unmarshal(e.Output, &o) != nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if e.Change == wmipc.OutputRemoved {
		delete(w.outputs, o.Name)
	} else {
		w.outputs[o.Name] = o
	}
	switch {
	case o.Primary && e.Change != wmipc.OutputRemoved:
		w.primary = o.Name
	case w.primary == o.Name:
		w.primary = ""
	}
}
