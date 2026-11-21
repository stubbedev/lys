package meter

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/stubbedev/lys/power/energy"
)

const psrStatusSuffix = "_edp_psr_status"

var (
	psr1Refreshing = []string{"SRDENT", "SRDENT_ON"}
	psr2Refreshing = []string{"SLEEP", "SU_STANDBY", "FAST_SLEEP", "DEEP_SLEEP"}
)

func (m Meter) Panels() ([]energy.PanelState, bool, error) {
	devices, err := os.ReadDir(m.DRIDebug)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, false, nil
	case errors.Is(err, fs.ErrPermission):
		return nil, true, nil
	case err != nil:
		return nil, false, fmt.Errorf("listing %s: %w", m.DRIDebug, err)
	}
	seen := map[string]bool{}
	var out []energy.PanelState
	for _, d := range devices {
		dir, err := filepath.EvalSymlinks(filepath.Join(m.DRIDebug, d.Name()))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, false, fmt.Errorf("resolving %s/%s: %w", m.DRIDebug, d.Name(), err)
		}
		if seen[dir] {
			continue
		}
		seen[dir] = true
		files, err := os.ReadDir(dir)
		if errors.Is(err, fs.ErrPermission) {
			return nil, true, nil
		}
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			continue
		}
		if err != nil {
			return nil, false, fmt.Errorf("listing %s: %w", dir, err)
		}
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), psrStatusSuffix) {
				continue
			}
			path := filepath.Join(dir, f.Name())
			raw, err := os.ReadFile(path)
			if errors.Is(err, fs.ErrPermission) {
				return nil, true, nil
			}
			if err != nil {
				return nil, false, fmt.Errorf("reading %s: %w", path, err)
			}
			mode, refreshing, err := ParsePSRStatus(raw)
			if err != nil {
				return nil, false, fmt.Errorf("%s: %w", path, err)
			}
			out = append(out, energy.PanelState{Device: filepath.Base(dir), Mode: mode, Refreshing: refreshing})
		}
	}
	slices.SortFunc(out, func(a, b energy.PanelState) int { return strings.Compare(a.Device, b.Device) })
	return out, false, nil
}

func ParsePSRStatus(raw []byte) (energy.SelfRefresh, bool, error) {
	var sink, mode, status string
	var hasMode, hasStatus bool
	for line := range strings.Lines(string(raw)) {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch key {
		case "Sink support":
			sink = value
		case "PSR mode":
			mode, hasMode = value, true
		case "Source PSR/PanelReplay status", "Source PSR status":
			status, hasStatus = value, true
		}
	}
	if !hasMode {
		if sink != "" && !strings.Contains(sink, "= yes") {
			return energy.SelfRefreshUnsupported, false, nil
		}
		return 0, false, fmt.Errorf("no PSR mode in a status whose sink support is %q", sink)
	}
	sr, err := selfRefreshMode(mode)
	if err != nil {
		return 0, false, err
	}
	if sr == energy.SelfRefreshOff {
		return sr, false, nil
	}
	if !hasStatus {
		return 0, false, fmt.Errorf("PSR mode %q without a source status", mode)
	}
	state, _, _ := strings.Cut(status, " ")
	refreshing := psr2Refreshing
	if sr == energy.SelfRefreshPSR1 {
		refreshing = psr1Refreshing
	}
	return sr, slices.Contains(refreshing, state), nil
}

func selfRefreshMode(mode string) (energy.SelfRefresh, error) {
	switch {
	case mode == "disabled":
		return energy.SelfRefreshOff, nil
	case strings.HasPrefix(mode, "Panel Replay Selective Update"):
		return energy.SelfRefreshPanelReplaySU, nil
	case strings.HasPrefix(mode, "Panel Replay"):
		return energy.SelfRefreshPanelReplay, nil
	case strings.HasPrefix(mode, "PSR2"):
		return energy.SelfRefreshPSR2, nil
	case strings.HasPrefix(mode, "PSR1"):
		return energy.SelfRefreshPSR1, nil
	}
	return 0, fmt.Errorf("PSR mode %q: want PSR1, PSR2, Panel Replay, Panel Replay Selective Update or disabled", mode)
}
