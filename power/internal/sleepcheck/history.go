package sleepcheck

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/stubbedev/lys/power/internal/statefile"
	"github.com/stubbedev/lys/power/sleep"
)

// Load reads the reports kept at path, oldest first; none when there is
// no file yet.
func Load(path string) ([]sleep.Report, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the sleep reports: %w", err)
	}
	var wires []sleep.Wire
	if err := json.Unmarshal(raw, &wires); err != nil {
		return nil, fmt.Errorf("parsing the sleep reports in %s: %w", path, err)
	}
	out := make([]sleep.Report, len(wires))
	for i, w := range wires {
		if out[i], err = sleep.FromWire(w); err != nil {
			return nil, fmt.Errorf("parsing the sleep reports in %s: %w", path, err)
		}
	}
	return out, nil
}

// Record appends r to reports, keeps the last sleep.Kept, writes them to
// path and returns them.
func Record(path string, reports []sleep.Report, r sleep.Report) ([]sleep.Report, error) {
	kept := append(reports[max(0, len(reports)+1-sleep.Kept):len(reports):len(reports)], r)
	wires := make([]sleep.Wire, len(kept))
	for i, k := range kept {
		wires[i] = sleep.ToWire(k)
	}
	raw, err := json.Marshal(wires)
	if err != nil {
		return nil, fmt.Errorf("encoding the sleep reports: %w", err)
	}
	if err := statefile.Write(path, raw); err != nil {
		return nil, fmt.Errorf("recording the sleep reports: %w", err)
	}
	return kept, nil
}
