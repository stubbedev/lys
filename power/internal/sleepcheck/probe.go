package sleepcheck

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/power/internal/supply"
)

const (
	lpitPath       = "devices/system/cpu/cpuidle/low_power_idle_system_residency_us"
	slpS0Path      = "kernel/debug/pmc_core/slp_s0_residency_usec"
	substatesPath  = "kernel/debug/pmc_core/substate_requirements"
	wakeupClassDir = "class/wakeup"
)

var counterPaths = [...]struct {
	counter Counter
	path    string
}{
	{CounterLPIT, lpitPath},
	{CounterSLPS0, slpS0Path},
}

// Probe reads snapshots from the sysfs mounted at Sys, debugfs at its
// kernel/debug, and the power supplies of Supply.
type Probe struct {
	Sys    string
	Supply supply.Supply
}

// Snapshot reads the machine as it is at at.
func (p Probe) Snapshot(at time.Time) (Snapshot, error) {
	s := Snapshot{At: at}
	var err error
	if s.Residency, err = p.residency(); err != nil {
		return Snapshot{}, err
	}
	if s.OnBattery, s.Energy, err = p.battery(); err != nil {
		return Snapshot{}, err
	}
	if s.Wakeups, err = p.wakeups(); err != nil {
		return Snapshot{}, err
	}
	if s.Blockers, err = p.blockers(); err != nil {
		return Snapshot{}, err
	}
	return s, nil
}

func (p Probe) residency() (*Residency, error) {
	for _, c := range counterPaths {
		n, err := readUint(filepath.Join(p.Sys, c.path))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return &Residency{Counter: c.counter, Micros: n}, nil
	}
	return nil, nil
}

func (p Probe) battery() (bool, *uint64, error) {
	battery, err := p.Supply.Battery()
	if err != nil || battery == nil {
		return false, nil, err
	}
	onAC, err := p.Supply.OnAC()
	if err != nil {
		return false, nil, err
	}
	energy, known, err := battery.EnergyMicrowattHours()
	if err != nil || !known {
		return !onAC, nil, err
	}
	return !onAC, &energy, nil
}

func (p Probe) wakeups() (map[string]uint64, error) {
	dir := filepath.Join(p.Sys, wakeupClassDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]uint64{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	counts := make(map[string]uint64, len(entries))
	for _, e := range entries {
		source := filepath.Join(dir, e.Name())
		name, err := os.ReadFile(filepath.Join(source, "name"))
		if err != nil {
			return nil, fmt.Errorf("reading the name of %s: %w", source, err)
		}
		n, err := readUint(filepath.Join(source, "wakeup_count"))
		if err != nil {
			return nil, err
		}
		counts[strings.TrimSpace(string(name))] += n
	}
	return counts, nil
}

func (p Probe) blockers() ([]string, error) {
	path := filepath.Join(p.Sys, substatesPath)
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	blockers, err := ParseSubstateRequirements(string(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return blockers, nil
}

// ParseSubstateRequirements reads Intel pmc_core's substate_requirements
// table: a row per element, a column per low-power substate holding
// Required where the substate needs it, and a Status column holding Yes
// where the platform last found it met. It returns the elements some
// substate requires and the platform found unmet, in table order.
func ParseSubstateRequirements(table string) ([]string, error) {
	lines := strings.Split(strings.TrimRight(table, "\n"), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return nil, errors.New("no header")
	}
	header := cells(lines[0])
	status := slices.Index(header, "Status")
	if len(header) < 3 || header[0] != "Element" || status < 2 {
		return nil, fmt.Errorf("header %q is not Element, the substates, Status", lines[0])
	}
	var out []string
	for _, line := range lines[1:] {
		row := cells(line)
		if len(row) != len(header) {
			return nil, fmt.Errorf("row %q has %d cells, the header %d", line, len(row), len(header))
		}
		required := slices.Contains(row[1:status], "Required")
		if required && row[status] != "Yes" {
			out = append(out, row[0])
		}
	}
	return out, nil
}

func cells(line string) []string {
	parts := strings.Split(strings.TrimSuffix(strings.TrimSpace(line), "|"), "|")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

func readUint(path string) (uint64, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", path, err)
	}
	n, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing %s: %w", path, err)
	}
	return n, nil
}
