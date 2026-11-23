package swaytest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/lib/procstat"
)

const (
	maxLogTail       = 64 << 10
	maxBacktrace     = 64 << 10
	maxThreads       = 64
	backtraceTimeout = 10 * time.Second
)

type watch struct {
	pid   int
	event string
	began time.Time
	cpu   cpuSample
}

type cpuSample struct {
	time time.Duration
	err  error
}

func watchSway(pid int, event string) watch {
	return watch{pid: pid, event: event, began: time.Now(), cpu: sampleCPU(pid)}
}

func (w watch) report(dir string) string {
	failed := sampleCPU(w.pid)
	waited := time.Since(w.began)
	var b strings.Builder
	fmt.Fprintf(&b, "sway, pid %d, at failure:\n", w.pid)
	fmt.Fprintf(&b, "  State: %s\n", orError(statusState(w.pid)))
	fmt.Fprintf(&b, "  wchan: %s\n", orError(readTrimmed(procPath(w.pid, "wchan"))))
	fmt.Fprintf(&b, "  CPU (user+system): %s when %s, %s at failure, %s in the %v between\n", w.cpu, w.event, failed, cpuDelta(w.cpu, failed), waited)
	writeThreads(&b, w.pid)
	b.WriteString("host:\n")
	fmt.Fprintf(&b, "  load average: %s\n", orError(readTrimmed("/proc/loadavg")))
	fmt.Fprintf(&b, "  online CPUs: %s, %d usable by this process\n", orError(onlineCPUs()), runtime.NumCPU())
	fmt.Fprintf(&b, "  kernel.yama.ptrace_scope: %s\n", orError(readTrimmed("/proc/sys/kernel/yama/ptrace_scope")))
	b.WriteString(backtrace(w.pid))
	b.WriteString(swayLog(dir))
	return b.String()
}

func (s cpuSample) String() string {
	if s.err != nil {
		return "(error: " + s.err.Error() + ")"
	}
	return s.time.String()
}

func cpuDelta(from, to cpuSample) string {
	if from.err != nil || to.err != nil {
		return "unknown"
	}
	return (to.time - from.time).String()
}

func sampleCPU(pid int) cpuSample {
	st, err := procstat.Of(pid)
	if err != nil {
		return cpuSample{err: err}
	}
	d, err := st.CPU()
	return cpuSample{time: d, err: err}
}

func writeThreads(b *strings.Builder, pid int) {
	dir := procPath(pid, "task")
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(b, "  threads: error: %v\n", err)
		return
	}
	tids := make([]int, 0, len(entries))
	for _, e := range entries {
		tid, err := strconv.Atoi(e.Name())
		if err != nil {
			fmt.Fprintf(b, "  threads: error: %s holds %q, not a thread id\n", dir, e.Name())
			return
		}
		tids = append(tids, tid)
	}
	slices.Sort(tids)
	fmt.Fprintf(b, "  threads (%d):\n", len(tids))
	for _, tid := range tids[:min(len(tids), maxThreads)] {
		b.WriteString("    " + threadLine(filepath.Join(dir, strconv.Itoa(tid)), tid) + "\n")
	}
	if left := len(tids) - maxThreads; left > 0 {
		fmt.Fprintf(b, "    %d more left out\n", left)
	}
}

func threadLine(dir string, tid int) string {
	wchan := orError(readTrimmed(filepath.Join(dir, "wchan")))
	st, err := procstat.Read(filepath.Join(dir, "stat"))
	if err != nil {
		return fmt.Sprintf("%d: error: %v, wchan %s", tid, err, wchan)
	}
	cpu, err := st.CPU()
	return fmt.Sprintf("%d %s: state %s, wchan %s, CPU %s", tid, st.Comm, st.State, wchan, cpuSample{time: cpu, err: err})
}

func procPath(pid int, name string) string {
	return filepath.Join("/proc", strconv.Itoa(pid), name)
}

func readTrimmed(name string) (string, error) {
	b, err := os.ReadFile(name) //nolint:gosec // a /proc or /sys file
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func orError(s string, err error) string {
	if err != nil {
		return "error: " + err.Error()
	}
	return s
}

func statusState(pid int) (string, error) {
	name := procPath(pid, "status")
	status, err := readTrimmed(name)
	if err != nil {
		return "", err
	}
	for line := range strings.Lines(status) {
		if state, ok := strings.CutPrefix(line, "State:"); ok {
			return strings.TrimSpace(state), nil
		}
	}
	return "", fmt.Errorf("no State line in %s", name)
}

func onlineCPUs() (string, error) {
	const name = "/sys/devices/system/cpu/online"
	ranges, err := readTrimmed(name)
	if err != nil {
		return "", err
	}
	count := 0
	for r := range strings.SplitSeq(ranges, ",") {
		first, last, isRange := strings.Cut(r, "-")
		if !isRange {
			last = first
		}
		lo, err := strconv.Atoi(first)
		if err != nil {
			return "", fmt.Errorf("parse %s %q: %w", name, ranges, err)
		}
		hi, err := strconv.Atoi(last)
		if err != nil {
			return "", fmt.Errorf("parse %s %q: %w", name, ranges, err)
		}
		count += hi - lo + 1
	}
	return fmt.Sprintf("%d (%s)", count, ranges), nil
}

func backtrace(pid int) string {
	p := strconv.Itoa(pid)
	for _, argv := range [][]string{
		{"eu-stack", "-p", p},
		{"gdb", "-batch", "-p", p, "-ex", "thread apply all bt"},
	} {
		if _, err := exec.LookPath(argv[0]); err == nil {
			return runBacktrace(argv)
		}
	}
	return "backtrace: neither eu-stack nor gdb is on PATH\n"
}

func runBacktrace(argv []string) string {
	ctx, cancel := context.WithTimeout(context.Background(), backtraceTimeout)
	defer cancel()
	var out capped
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // a debugger from PATH, for a test
	cmd.Stdout, cmd.Stderr = &out, &out
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	outcome := "exit status 0"
	switch {
	case ctx.Err() != nil:
		outcome = fmt.Sprintf("killed after its %v timeout", backtraceTimeout)
	case err != nil:
		outcome = err.Error()
	}
	return fmt.Sprintf("backtrace by %q (%s):\n%s", argv, outcome, out.String())
}

type capped struct {
	buf     []byte
	dropped int
}

func (c *capped) Write(p []byte) (int, error) {
	n := min(len(p), maxBacktrace-len(c.buf))
	c.buf = append(c.buf, p[:n]...)
	c.dropped += len(p) - n
	return len(p), nil
}

func (c *capped) String() string {
	if c.dropped > 0 {
		return fmt.Sprintf("%s\n(its last %d bytes left out)\n", c.buf, c.dropped)
	}
	return string(c.buf)
}

func swayLog(dir string) string {
	name := filepath.Join(dir, logName)
	tail, skipped, err := readTail(name)
	if err != nil {
		return fmt.Sprintf("sway's log: error: %v\n", err)
	}
	if skipped > 0 {
		return fmt.Sprintf("sway's log %s, its first %d bytes left out:\n%s", name, skipped, tail)
	}
	return fmt.Sprintf("sway's log %s:\n%s", name, tail)
}

func readTail(name string) (tail []byte, skipped int64, err error) {
	f, err := os.Open(name) //nolint:gosec // a file in the test's own directory
	if err != nil {
		return nil, 0, fmt.Errorf("open %s: %w", name, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, 0, fmt.Errorf("stat %s: %w", name, err)
	}
	skipped = max(info.Size()-maxLogTail, 0)
	tail = make([]byte, info.Size()-skipped)
	n, err := f.ReadAt(tail, skipped)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, 0, fmt.Errorf("read %s: %w", name, err)
	}
	return tail[:n], skipped, nil
}
