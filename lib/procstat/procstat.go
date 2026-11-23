package procstat

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/intconv"
)

// Stat is what /proc/<pid>/stat says of a process: its command name, its
// state, its parent, the clock ticks it ran in user and kernel mode, and
// the clock tick it started at after boot.
type Stat struct {
	Comm      string
	State     string
	Parent    uint32
	CPUTicks  uint64
	StartTime uint64
}

const (
	firstAfterComm = 3
	stateField     = 3
	parentField    = 4
	utimeField     = 14
	stimeField     = 15
	startTimeField = 22
	atClkTck       = 17
)

// Of is the Stat of process pid.
func Of(pid int) (Stat, error) {
	return Read("/proc/" + strconv.Itoa(pid) + "/stat")
}

// Read is the Stat in the procfs stat file at path, a process's or a
// thread's.
func Read(path string) (Stat, error) {
	stat, err := os.ReadFile(path) //nolint:gosec // a procfs stat file
	if err != nil {
		return Stat{}, fmt.Errorf("procstat: %w", err)
	}
	st, err := parse(stat)
	if err != nil {
		return Stat{}, fmt.Errorf("procstat: %s: %w", path, err)
	}
	return st, nil
}

func parse(stat []byte) (Stat, error) {
	_, afterOpen, ok := bytes.Cut(stat, []byte("("))
	if !ok {
		return Stat{}, fmt.Errorf("no (comm) in %q", stat)
	}
	comm, afterComm, ok := bytes.CutLast(afterOpen, []byte(")"))
	if !ok {
		return Stat{}, fmt.Errorf("no end to (comm) in %q", stat)
	}
	fields := bytes.Fields(afterComm)
	if len(fields) <= startTimeField-firstAfterComm {
		return Stat{}, fmt.Errorf("%d fields after (comm), want at least %d, in %q", len(fields), startTimeField-firstAfterComm+1, stat)
	}
	field := func(n int) (uint64, error) {
		v, err := strconv.ParseUint(string(fields[n-firstAfterComm]), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("field %d of %q: %w", n, stat, err)
		}
		return v, nil
	}
	parent, err := field(parentField)
	if err != nil {
		return Stat{}, err
	}
	if parent > math.MaxUint32 {
		return Stat{}, fmt.Errorf("parent %d of %q: not a process id", parent, stat)
	}
	utime, err := field(utimeField)
	if err != nil {
		return Stat{}, err
	}
	stime, err := field(stimeField)
	if err != nil {
		return Stat{}, err
	}
	start, err := field(startTimeField)
	if err != nil {
		return Stat{}, err
	}
	return Stat{Comm: string(comm), State: string(fields[stateField-firstAfterComm]), Parent: uint32(parent), CPUTicks: utime + stime, StartTime: start}, nil
}

// CPU is the time the process or thread ran in user and kernel mode.
func (s Stat) CPU() (time.Duration, error) {
	clockTick.once.Do(func() { clockTick.tick, clockTick.err = readClockTick() })
	if clockTick.err != nil {
		return 0, clockTick.err
	}
	return intconv.Duration(s.CPUTicks, clockTick.tick), nil
}

var clockTick struct {
	once sync.Once
	tick time.Duration
	err  error
}

func readClockTick() (time.Duration, error) {
	hz, err := readClockTicks()
	if err != nil {
		return 0, err
	}
	if hz == 0 || hz > uint64(time.Second) {
		return 0, fmt.Errorf("procstat: AT_CLKTCK %d is not a clock rate in (0, 1e9] Hz", hz)
	}
	return time.Second / time.Duration(hz), nil
}

func readClockTicks() (uint64, error) {
	const name = "/proc/self/auxv"
	auxv, err := os.ReadFile(name)
	if err != nil {
		return 0, fmt.Errorf("procstat: %w", err)
	}
	word := strconv.IntSize / 8
	for i := 0; i+2*word <= len(auxv); i += 2 * word {
		if auxvWord(auxv[i:], word) == atClkTck {
			return auxvWord(auxv[i+word:], word), nil
		}
	}
	return 0, fmt.Errorf("procstat: no AT_CLKTCK in %s", name)
}

func auxvWord(b []byte, word int) uint64 {
	if word == 8 {
		return binary.NativeEndian.Uint64(b)
	}
	return uint64(binary.NativeEndian.Uint32(b))
}
