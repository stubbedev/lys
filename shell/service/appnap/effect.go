package appnap

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/power/nap"
)

const (
	systemdName    = "org.freedesktop.systemd1"
	systemdPath    = dbus.ObjectPath("/org/freedesktop/systemd1")
	systemdManager = "org.freedesktop.systemd1.Manager"
	noSuchUnit     = "org.freedesktop.systemd1.NoSuchUnit"
)

// ErrNoEfficiencyCores is the efficiency-cores level on a machine without them.
var ErrNoEfficiencyCores = errors.New("this machine has no efficiency cores the kernel names")

// Slacker sets the timer slack of a unit's threads, as lys-power does.
type Slacker interface {
	SetTimerSlack(ctx context.Context, unit nap.Unit, slack time.Duration) error
}

// Systemd applies changes through the user's systemd manager and a Slacker.
type Systemd struct {
	Manager    dbus.BusObject
	Slack      Slacker
	Efficiency func() ([]byte, error)
}

// NewSystemd is the user's systemd on the session bus, finding the
// efficiency cores under sys.
func NewSystemd(session *dbus.Conn, slack Slacker, sys string) Systemd {
	return Systemd{
		Manager:    session.Object(systemdName, systemdPath),
		Slack:      slack,
		Efficiency: func() ([]byte, error) { return EfficiencyMask(sys) },
	}
}

type property struct {
	Name  string
	Value dbus.Variant
}

// Apply makes one change, thawing first and freezing last; a unit that
// is gone needs nothing.
func (s Systemd) Apply(ctx context.Context, c Change) error {
	err := s.apply(ctx, c)
	if dbusErr, ok := errors.AsType[dbus.Error](err); ok && dbusErr.Name == noSuchUnit {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s from %+v to %+v: %w", c.Unit, c.From, c.To, err)
	}
	return nil
}

func (s Systemd) apply(ctx context.Context, c Change) error {
	if c.From.Frozen && !c.To.Frozen {
		if err := s.call(ctx, "ThawUnit", string(c.Unit)); err != nil {
			return fmt.Errorf("thaw: %w", err)
		}
	}
	var props []property
	if c.From.CPUWeight != c.To.CPUWeight {
		props = append(props, property{"CPUWeight", dbus.MakeVariant(uint64(c.To.CPUWeight))})
	}
	if c.From.IOWeight != c.To.IOWeight {
		props = append(props, property{"IOWeight", dbus.MakeVariant(uint64(c.To.IOWeight))})
	}
	if c.From.Cores != c.To.Cores {
		mask := []byte{}
		if c.To.Cores == nap.CoresEfficiency {
			var err error
			if mask, err = s.Efficiency(); err != nil {
				return err
			}
		}
		props = append(props, property{"AllowedCPUs", dbus.MakeVariant(mask)})
	}
	if len(props) > 0 {
		if err := s.call(ctx, "SetUnitProperties", string(c.Unit), true, props); err != nil {
			return fmt.Errorf("set %d properties: %w", len(props), err)
		}
	}
	if c.From.TimerSlack != c.To.TimerSlack {
		if err := s.Slack.SetTimerSlack(ctx, c.Unit, c.To.TimerSlack); err != nil {
			return err
		}
	}
	if c.To.Frozen && !c.From.Frozen {
		if err := s.call(ctx, "FreezeUnit", string(c.Unit)); err != nil {
			return fmt.Errorf("freeze: %w", err)
		}
	}
	return nil
}

func (s Systemd) call(ctx context.Context, method string, args ...any) error {
	return s.Manager.CallWithContext(ctx, systemdManager+"."+method, 0, args...).Err
}

// EfficiencyMask is the AllowedCPUs mask of the cores the kernel lists as
// cpu_atom under sys.
func EfficiencyMask(sys string) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join(sys, "devices", "cpu_atom", "cpus")) //nolint:gosec // under /sys
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoEfficiencyCores
	}
	if err != nil {
		return nil, fmt.Errorf("reading the efficiency cores: %w", err)
	}
	return CPUMask(strings.TrimSpace(string(raw)))
}

// CPUMask turns a kernel cpu list (0-3,8) into an AllowedCPUs mask.
func CPUMask(list string) ([]byte, error) {
	var mask []byte
	for part := range strings.SplitSeq(list, ",") {
		first, last, ranged := strings.Cut(part, "-")
		lo, err := strconv.ParseUint(first, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("cpu list %q: %w", list, err)
		}
		hi := lo
		if ranged {
			if hi, err = strconv.ParseUint(last, 10, 16); err != nil {
				return nil, fmt.Errorf("cpu list %q: %w", list, err)
			}
		}
		if hi < lo {
			return nil, fmt.Errorf("cpu list %q: range %d-%d runs backwards", list, lo, hi)
		}
		for cpu := lo; cpu <= hi; cpu++ {
			for uint64(len(mask)) <= cpu/8 {
				mask = append(mask, 0)
			}
			mask[cpu/8] |= 1 << (cpu % 8)
		}
	}
	if len(mask) == 0 {
		return nil, fmt.Errorf("cpu list %q: %w", list, ErrNoEfficiencyCores)
	}
	return mask, nil
}

// UnitOf is the unit of a /proc/<pid>/cgroup in a user's manager.
func UnitOf(cgroup string) (nap.Unit, bool) {
	for line := range strings.Lines(cgroup) {
		path, ok := strings.CutPrefix(strings.TrimSpace(line), "0::")
		if !ok || !strings.Contains(path, "/user@") {
			continue
		}
		u, err := nap.ParseUnit(filepath.Base(path))
		return u, err == nil
	}
	return "", false
}

// UnitsOf maps each pid to its unit; a process that is gone has none.
func UnitsOf(proc string, pids []int32) (map[int32]nap.Unit, error) {
	out := make(map[int32]nap.Unit, len(pids))
	var errs error
	for _, pid := range pids {
		if _, done := out[pid]; done || pid <= 0 {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(proc, strconv.Itoa(int(pid)), "cgroup")) //nolint:gosec // under /proc
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
			continue
		}
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("the unit of process %d: %w", pid, err))
			continue
		}
		if u, ok := UnitOf(string(raw)); ok {
			out[pid] = u
		}
	}
	return out, errs
}
