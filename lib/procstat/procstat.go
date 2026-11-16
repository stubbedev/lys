package procstat

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
)

// Stat is what /proc/<pid>/stat says of a process: its command name, its
// state, the clock ticks it ran in user and kernel mode, and the clock
// tick it started at after boot.
type Stat struct {
	Comm      string
	State     string
	CPUTicks  uint64
	StartTime uint64
}

const (
	firstAfterComm = 3
	stateField     = 3
	utimeField     = 14
	stimeField     = 15
	startTimeField = 22
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
	return Stat{Comm: string(comm), State: string(fields[stateField-firstAfterComm]), CPUTicks: utime + stime, StartTime: start}, nil
}
