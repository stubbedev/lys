package cava

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/stubbedev/lys/lib/waittest"
)

// waitDrained polls in until the n samples what names accumulated.
func waitDrained(t *testing.T, in Input, what string, n int) []float64 {
	t.Helper()
	var got []float64
	waittest.Until(t, func() bool {
		got = append(got, in.Drained(nil)...)
		return len(got) >= n
	}, fmt.Sprintf("%s (%d samples)", what, n))
	return got
}

func s16le(vals ...int16) []byte {
	out := make([]byte, 2*len(vals))
	for i, v := range vals {
		binary.LittleEndian.PutUint16(out[2*i:], uint16(v))
	}
	return out
}

func TestFifoInputReadsBlocksAndGoesSilentWhenTheWriterLeaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cava.fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	in, err := NewFifoInput(path, 1, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if err := in.Start(); err != nil {
		t.Fatal(err)
	}
	defer in.Stop()
	w, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	block := make([]int16, fragmentFrames)
	for i := range block {
		block[i] = int16(i - 256)
	}
	if _, err := w.Write(s16le(block...)); err != nil {
		t.Fatal(err)
	}
	got := waitDrained(t, in, "a block", fragmentFrames)
	if got[0] != -256 || got[511] != 255 {
		t.Errorf("block = %v … %v, want the raw s16 values", got[0], got[511])
	}
	_ = w.Close()
	// Ten idle reads later the buffer goes silent: a full buffer of zeros.
	silent := waitDrained(t, in, "silence", 4096)
	for _, v := range silent {
		if v != 0 {
			t.Fatalf("silence carried %v", v)
		}
	}
}

func TestFifoInputStopsWithoutAWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "never.fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	in, _ := NewFifoInput(path, 2, 4096)
	if err := in.Start(); err != nil {
		t.Fatal(err)
	}
	if err := in.Start(); err == nil {
		t.Error("a second Start succeeded")
	}
	stopped := make(chan struct{})
	go func() { in.Stop(); close(stopped) }()
	waittest.For(t, stopped, "Stop to return on a pipe nobody writes")
	in.Stop() // again: nothing
}

func TestFifoInputDevZeroIsTheTestSource(t *testing.T) {
	in, _ := NewFifoInput("/dev/zero", 2, 8192)
	if err := in.Start(); err != nil {
		t.Fatal(err)
	}
	defer in.Stop()
	got := waitDrained(t, in, "zeros", 2*fragmentFrames)
	if len(got)%2 != 0 {
		t.Errorf("%d samples: stereo blocks come in whole frames", len(got))
	}
}

func TestInputsRejectBadArguments(t *testing.T) {
	if _, err := NewFifoInput("/x", 3, 16); err == nil {
		t.Error("fifo: three channels accepted")
	}
	if _, err := NewShmemInput("/x", 1, 0); err == nil {
		t.Error("shmem: a zero capacity accepted")
	}
}

// writeVisArea writes a squeezelite vis_t into /dev/shm and returns its
// name.
func writeVisArea(t *testing.T, running bool, rate uint32, fill func(i int) int16) string {
	t.Helper()
	name := "/lys-shell-cava-test-" + strconv.Itoa(os.Getpid()) + "-" + strings.ReplaceAll(t.Name(), "/", "-")
	area := make([]byte, visSize)
	binary.LittleEndian.PutUint32(area[visBufSizeOff:], visBufSamples)
	if running {
		area[visRunningOff] = 1
	}
	binary.LittleEndian.PutUint32(area[visRateOff:], rate)
	for i := range visBufSamples {
		binary.LittleEndian.PutUint16(area[visBufferOff+2*i:], uint16(fill(i)))
	}
	path := "/dev/shm" + name
	if err := os.WriteFile(path, area, 0o600); err != nil {
		t.Skipf("no /dev/shm: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	return name
}

func TestShmemInputReadsSqueezelitesArea(t *testing.T) {
	name := writeVisArea(t, true, 44100, func(i int) int16 { return int16(i % 1000) })
	in, err := NewShmemInput(name, 1, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if err := in.Start(); err != nil {
		t.Fatal(err)
	}
	defer in.Stop()
	got := waitDrained(t, in, "the first block", fragmentFrames)
	for n := range fragmentFrames {
		if got[n] != float64(n%1000) {
			t.Fatalf("sample %d = %v, want the area's buffer[%d]", n, got[n], n)
		}
	}
}

func TestShmemInputIsSilentWhileStopped(t *testing.T) {
	name := writeVisArea(t, false, 0, func(int) int16 { return 7 })
	in, _ := NewShmemInput(name, 2, 4096)
	if err := in.Start(); err != nil {
		t.Fatal(err)
	}
	defer in.Stop()
	got := waitDrained(t, in, "silence", 2*fragmentFrames)
	for _, v := range got {
		if v != 0 {
			t.Fatalf("a stopped squeezelite gave %v, want silence", v)
		}
	}
}

func TestShmemInputMissingSource(t *testing.T) {
	in, _ := NewShmemInput("/lys-shell-cava-no-such-area", 1, 4096)
	err := in.Start()
	if err == nil || !strings.Contains(err.Error(), "could not open source '/lys-shell-cava-no-such-area'") {
		t.Errorf("err = %v, want the could-not-open error", err)
	}
}
