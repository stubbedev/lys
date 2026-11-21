// Package meter samples the counters energy attribution works from: each
// app cgroup's CPU time and processes, each thread's context switches,
// each DRM client's engine time, the RAPL energy counters, the battery's
// discharge power and each eDP panel's self-refresh state in the DRM
// debugfs.
package meter

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/stubbedev/lys/power/energy"
	"github.com/stubbedev/lys/power/internal/supply"
)

type Meter struct {
	Cgroup   string
	Proc     string
	Powercap string
	DRIDebug string
	Supply   supply.Supply
}

type Gaps struct {
	EnergyUnreadable bool
	GPUUnreadable    int
	PanelsUnreadable bool
}

func (m Meter) Sample(now func() time.Time) (energy.Sample, Gaps, error) {
	var gaps Gaps
	units, err := m.units()
	if err != nil {
		return energy.Sample{}, gaps, err
	}
	total, err := usage(filepath.Join(m.Cgroup, "cpu.stat"))
	if err != nil {
		return energy.Sample{}, gaps, err
	}
	s := energy.Sample{TotalCPU: total, Groups: make([]energy.Group, 0, len(units))}
	seen := map[energy.DRMClient]bool{}
	for _, rel := range units {
		g, ok, err := m.group(rel, seen, &gaps)
		if err != nil {
			return energy.Sample{}, gaps, err
		}
		if ok {
			s.Groups = append(s.Groups, g)
		}
	}
	counters, err := m.energy()
	switch {
	case errors.Is(err, fs.ErrPermission):
		gaps.EnergyUnreadable = true
	case err != nil:
		return energy.Sample{}, gaps, err
	default:
		s.Energy = counters
	}
	if s.Battery, err = m.battery(); err != nil {
		return energy.Sample{}, gaps, err
	}
	if s.Panels, gaps.PanelsUnreadable, err = m.Panels(); err != nil {
		return energy.Sample{}, gaps, err
	}
	s.At = now()
	return s, gaps, nil
}

func Gone(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ESRCH)
}

func (m Meter) units() ([]string, error) {
	var out []string
	var walk func(rel string) error
	walk = func(rel string) error {
		children, err := subdirs(filepath.Join(m.Cgroup, rel))
		if err != nil {
			return err
		}
		for _, name := range children {
			child := filepath.Join(rel, name)
			nested, err := hasSlice(filepath.Join(m.Cgroup, child))
			if err != nil {
				return err
			}
			if strings.HasSuffix(name, ".slice") || nested {
				if err := walk(child); err != nil {
					return err
				}
				continue
			}
			out = append(out, child)
		}
		return nil
	}
	if err := walk("."); err != nil {
		return nil, err
	}
	return out, nil
}

func subdirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if Gone(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing cgroup %s: %w", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

func hasSlice(dir string) (bool, error) {
	children, err := subdirs(dir)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(children, func(n string) bool { return strings.HasSuffix(n, ".slice") }), nil
}

func (m Meter) group(rel string, seen map[energy.DRMClient]bool, gaps *Gaps) (energy.Group, bool, error) {
	dir := filepath.Join(m.Cgroup, rel)
	cpu, err := usage(filepath.Join(dir, "cpu.stat"))
	if Gone(err) {
		return energy.Group{}, false, nil
	}
	if err != nil {
		return energy.Group{}, false, err
	}
	pids, err := IDs(dir, "cgroup.procs")
	if err != nil {
		return energy.Group{}, false, err
	}
	g := energy.Group{Cgroup: rel, CPU: cpu, PIDs: make([]uint32, 0, len(pids)), GPU: map[energy.DRMClient]map[string]energy.Engine{}}
	for _, pid := range pids {
		alive, err := m.process(&g, pid, seen, gaps)
		if err != nil {
			return energy.Group{}, false, err
		}
		if alive {
			g.PIDs = append(g.PIDs, pid)
		}
	}
	return g, true, nil
}

func IDs(dir, file string) ([]uint32, error) {
	var ids []uint32
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if Gone(err) {
			return fs.SkipDir
		}
		if err != nil {
			return fmt.Errorf("walking cgroup %s: %w", path, err)
		}
		if !d.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(filepath.Join(path, file))
		if Gone(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading %s/%s: %w", path, file, err)
		}
		for field := range strings.FieldsSeq(string(raw)) {
			id, err := strconv.ParseUint(field, 10, 32)
			if err != nil {
				return fmt.Errorf("parsing %s/%s: %w", path, file, err)
			}
			ids = append(ids, uint32(id))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(ids)
	return slices.Compact(ids), nil
}

func usage(path string) (time.Duration, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", path, err)
	}
	for line := range strings.Lines(string(raw)) {
		value, ok := strings.CutPrefix(strings.TrimSpace(line), "usage_usec ")
		if !ok {
			continue
		}
		usec, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parsing usage_usec in %s: %w", path, err)
		}
		return time.Duration(usec) * time.Microsecond, nil
	}
	return 0, fmt.Errorf("reading %s: no usage_usec", path)
}

func (m Meter) process(g *energy.Group, pid uint32, seen map[energy.DRMClient]bool, gaps *Gaps) (bool, error) {
	dir := filepath.Join(m.Proc, strconv.FormatUint(uint64(pid), 10))
	comm, err := readTrim(filepath.Join(dir, "comm"))
	if Gone(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if g.Command == "" {
		g.Command = comm
	}
	tids, err := os.ReadDir(filepath.Join(dir, "task"))
	if Gone(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("listing the threads of %d: %w", pid, err)
	}
	for _, e := range tids {
		tid, err := strconv.ParseUint(e.Name(), 10, 32)
		if err != nil {
			return false, fmt.Errorf("thread %q of %d: %w", e.Name(), pid, err)
		}
		name, switches, err := threadStatus(filepath.Join(dir, "task", e.Name(), "status"))
		if Gone(err) {
			continue
		}
		if err != nil {
			return false, err
		}
		g.Threads = append(g.Threads, energy.Thread{PID: pid, TID: uint32(tid), Process: comm, Name: name, Switches: switches})
	}
	return true, m.clients(g, dir, seen, gaps)
}

func threadStatus(path string) (name string, switches uint64, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", 0, fmt.Errorf("reading %s: %w", path, err)
	}
	found := 0
	for line := range strings.Lines(string(raw)) {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch key {
		case "Name":
			name = value
		case "voluntary_ctxt_switches", "nonvoluntary_ctxt_switches":
			n, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return "", 0, fmt.Errorf("parsing %s in %s: %w", key, path, err)
			}
			switches += n
			found++
		}
	}
	if found != 2 {
		return "", 0, fmt.Errorf("reading %s: no context switch counts", path)
	}
	return name, switches, nil
}

func (m Meter) clients(g *energy.Group, dir string, seen map[energy.DRMClient]bool, gaps *Gaps) error {
	fds, err := os.ReadDir(filepath.Join(dir, "fd"))
	if Gone(err) {
		return nil
	}
	if errors.Is(err, fs.ErrPermission) {
		gaps.GPUUnreadable++
		return nil
	}
	if err != nil {
		return fmt.Errorf("listing %s/fd: %w", dir, err)
	}
	for _, fd := range fds {
		target, err := os.Readlink(filepath.Join(dir, "fd", fd.Name()))
		if Gone(err) {
			continue
		}
		if errors.Is(err, fs.ErrPermission) {
			gaps.GPUUnreadable++
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading %s/fd/%s: %w", dir, fd.Name(), err)
		}
		if !strings.HasPrefix(target, "/dev/dri/") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, "fdinfo", fd.Name()))
		if Gone(err) {
			continue
		}
		if errors.Is(err, fs.ErrPermission) {
			gaps.GPUUnreadable++
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading %s/fdinfo/%s: %w", dir, fd.Name(), err)
		}
		client, engines, ok, err := ParseFDInfo(raw)
		if err != nil {
			return fmt.Errorf("%s/fdinfo/%s: %w", dir, fd.Name(), err)
		}
		if !ok || seen[client] {
			continue
		}
		seen[client] = true
		g.GPU[client] = engines
	}
	return nil
}

func ParseFDInfo(raw []byte) (energy.DRMClient, map[string]energy.Engine, bool, error) {
	var client energy.DRMClient
	var driver string
	hasID := false
	engines := map[string]energy.Engine{}
	scan := bufio.NewScanner(bytes.NewReader(raw))
	for scan.Scan() {
		key, value, ok := strings.Cut(scan.Text(), ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch {
		case key == "drm-driver":
			driver = value
		case key == "drm-pdev":
			client.Device = value
		case key == "drm-client-id":
			id, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return energy.DRMClient{}, nil, false, fmt.Errorf("drm-client-id %q: %w", value, err)
			}
			client.ID, hasID = id, true
		case strings.HasPrefix(key, "drm-engine-capacity-"):
		case strings.HasPrefix(key, "drm-engine-"):
			ns, err := strconv.ParseUint(strings.TrimSuffix(value, " ns"), 10, 64)
			if err != nil {
				return energy.DRMClient{}, nil, false, fmt.Errorf("%s %q: %w", key, value, err)
			}
			engines[strings.TrimPrefix(key, "drm-engine-")] = energy.Engine{Clock: energy.EngineNanoseconds, Busy: ns}
		case strings.HasPrefix(key, "drm-total-cycles-"):
			n, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return energy.DRMClient{}, nil, false, fmt.Errorf("%s %q: %w", key, value, err)
			}
			name := strings.TrimPrefix(key, "drm-total-cycles-")
			e := engines[name]
			e.Clock, e.Total = energy.EngineCycles, n
			engines[name] = e
		case strings.HasPrefix(key, "drm-cycles-"):
			n, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return energy.DRMClient{}, nil, false, fmt.Errorf("%s %q: %w", key, value, err)
			}
			name := strings.TrimPrefix(key, "drm-cycles-")
			e := engines[name]
			e.Clock, e.Busy = energy.EngineCycles, n
			engines[name] = e
		}
	}
	if err := scan.Err(); err != nil {
		return energy.DRMClient{}, nil, false, err
	}
	if !hasID {
		return energy.DRMClient{}, nil, false, nil
	}
	if client.Device == "" {
		client.Device = driver
	}
	return client, engines, true, nil
}

func (m Meter) energy() ([]energy.Counter, error) {
	entries, err := os.ReadDir(m.Powercap)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", m.Powercap, err)
	}
	var packages, platform []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "intel-rapl:") || strings.Count(name, ":") != 1 {
			continue
		}
		zone, err := readTrim(filepath.Join(m.Powercap, name, "name"))
		if err != nil {
			return nil, err
		}
		switch {
		case zone == "psys":
			platform = append(platform, name)
		case strings.HasPrefix(zone, "package"):
			packages = append(packages, name)
		}
	}
	zones := packages
	if len(platform) > 0 {
		zones = platform
	}
	slices.Sort(zones)
	counters := make([]energy.Counter, 0, len(zones))
	for _, zone := range zones {
		dir := filepath.Join(m.Powercap, zone)
		uj, err := readUint(filepath.Join(dir, "energy_uj"))
		if err != nil {
			return nil, err
		}
		limit, err := readUint(filepath.Join(dir, "max_energy_range_uj"))
		if err != nil {
			return nil, err
		}
		counters = append(counters, energy.Counter{Microjoules: uj, Range: limit})
	}
	return counters, nil
}

func (m Meter) battery() (*uint64, error) {
	b, err := m.Supply.Battery()
	if err != nil || b == nil {
		return nil, err
	}
	microwatts, discharging, err := b.DischargeMicrowatts()
	if err != nil || !discharging {
		return nil, err
	}
	return &microwatts, nil
}

func readTrim(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	return strings.TrimSpace(string(raw)), nil
}

func readUint(path string) (uint64, error) {
	raw, err := readTrim(path)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing %s: %w", path, err)
	}
	return n, nil
}
