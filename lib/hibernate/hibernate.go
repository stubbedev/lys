// Package hibernate names what keeps a Linux system from hibernating: a
// kernel built without it, a kernel in lockdown, no swap or less swap than
// memory, and no resume device on the kernel command line. It reads
// /proc and /sys through an fs.FS rooted at /, so the tests read trees of
// their own.
package hibernate

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
)

// Blocker is one reason the system cannot hibernate.
type Blocker uint8

// The blockers, in the order Blockers reports them.
const (
	KernelUnsupported Blocker = iota + 1
	Lockdown
	NoSwap
	SmallSwap
	NoResumeDevice
)

var blockerNames = [...]string{
	KernelUnsupported: "kernel-unsupported",
	Lockdown:          "lockdown",
	NoSwap:            "no-swap",
	SmallSwap:         "small-swap",
	NoResumeDevice:    "no-resume-device",
}

// String is the blocker's name, the suffix of its message id.
func (b Blocker) String() string {
	if b < KernelUnsupported || int(b) >= len(blockerNames) {
		return fmt.Sprintf("Blocker(%d)", uint8(b))
	}
	return blockerNames[b]
}

const (
	statePath    = "sys/power/state"
	lockdownPath = "sys/kernel/security/lockdown"
	meminfoPath  = "proc/meminfo"
	swapsPath    = "proc/swaps"
	cmdlinePath  = "proc/cmdline"
)

// Blockers reads root, the system's /, and returns every blocker that
// holds, none when nothing this package knows of keeps hibernation off.
func Blockers(root fs.FS) ([]Blocker, error) {
	state, err := fs.ReadFile(root, statePath)
	if err != nil {
		return nil, fmt.Errorf("hibernate: read /%s: %w", statePath, err)
	}
	lockdown, err := fs.ReadFile(root, lockdownPath)
	if errors.Is(err, fs.ErrNotExist) {
		lockdown, err = []byte("[none]"), nil
	}
	if err != nil {
		return nil, fmt.Errorf("hibernate: read /%s: %w", lockdownPath, err)
	}
	meminfo, err := fs.ReadFile(root, meminfoPath)
	if err != nil {
		return nil, fmt.Errorf("hibernate: read /%s: %w", meminfoPath, err)
	}
	swaps, err := fs.ReadFile(root, swapsPath)
	if err != nil {
		return nil, fmt.Errorf("hibernate: read /%s: %w", swapsPath, err)
	}
	cmdline, err := fs.ReadFile(root, cmdlinePath)
	if err != nil {
		return nil, fmt.Errorf("hibernate: read /%s: %w", cmdlinePath, err)
	}
	memory, err := memTotalKiB(string(meminfo))
	if err != nil {
		return nil, fmt.Errorf("hibernate: /%s: %w", meminfoPath, err)
	}
	swap, err := swapKiB(string(swaps))
	if err != nil {
		return nil, fmt.Errorf("hibernate: /%s: %w", swapsPath, err)
	}
	locked, err := lockedDown(string(lockdown))
	if err != nil {
		return nil, fmt.Errorf("hibernate: /%s: %w", lockdownPath, err)
	}
	return Assess(System{
		DiskSleep:  slices.Contains(strings.Fields(string(state)), "disk"),
		LockedDown: locked,
		MemoryKiB:  memory,
		SwapKiB:    swap,
		Resume:     slices.ContainsFunc(strings.Fields(string(cmdline)), func(f string) bool { return strings.HasPrefix(f, "resume=") }),
	}), nil
}

// System is what decides whether the system can hibernate.
type System struct {
	DiskSleep  bool
	LockedDown bool
	MemoryKiB  uint64
	SwapKiB    uint64
	Resume     bool
}

// Assess returns the blockers that hold for s.
func Assess(s System) []Blocker {
	var out []Blocker
	if !s.DiskSleep {
		out = append(out, KernelUnsupported)
	}
	if s.LockedDown {
		out = append(out, Lockdown)
	}
	switch {
	case s.SwapKiB == 0:
		out = append(out, NoSwap)
	case s.SwapKiB < s.MemoryKiB:
		out = append(out, SmallSwap)
	}
	if !s.Resume {
		out = append(out, NoResumeDevice)
	}
	return out
}

func memTotalKiB(meminfo string) (uint64, error) {
	for line := range strings.Lines(meminfo) {
		rest, ok := strings.CutPrefix(line, "MemTotal:")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) != 2 || fields[1] != "kB" {
			return 0, fmt.Errorf("MemTotal %q is not a size in kB", strings.TrimSpace(rest))
		}
		n, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("MemTotal %q: %w", fields[0], err)
		}
		return n, nil
	}
	return 0, errors.New("no MemTotal")
}

func swapKiB(swaps string) (uint64, error) {
	sc := bufio.NewScanner(strings.NewReader(swaps))
	if !sc.Scan() {
		return 0, errors.New("no header")
	}
	var total uint64
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			return 0, fmt.Errorf("swap line %q has no size", sc.Text())
		}
		n, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("swap %s size %q: %w", fields[0], fields[2], err)
		}
		total += n
	}
	return total, sc.Err()
}

func lockedDown(lockdown string) (bool, error) {
	for f := range strings.FieldsSeq(lockdown) {
		if mode, ok := strings.CutPrefix(f, "["); ok {
			return strings.TrimSuffix(mode, "]") != "none", nil
		}
	}
	return false, fmt.Errorf("no mode selected in %q", strings.TrimSpace(lockdown))
}
