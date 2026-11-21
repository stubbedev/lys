package gpu

import (
	"fmt"
	"image"
	"image/color"
	"runtime"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/fourcc"
	"github.com/stubbedev/lys/lib/testbuild"
)

// passTarget is a size x size dmabuf target of d for passes.
func passTarget(t *testing.T, d *Device, size int) *Target {
	t.Helper()
	mods, err := d.Modifiers(fourcc.ARGB8888)
	if err != nil {
		t.Fatal(err)
	}
	tg, err := d.NewTarget(size, size, fourcc.ARGB8888, mods)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tg.Close)
	return tg
}

// fencingDevice is a device with sync_files that hands targets over by
// a fence on their dmabuf, NVIDIA's too: its driver attaches the fence
// as any does, and it is the compositor sampling the buffer that would
// not wait on it.
func fencingDevice(t *testing.T) *Device {
	t.Helper()
	d := syncDevice(t)
	d.implicitFences = true
	return d
}

// stillFencing skips the test on a kernel that attaches no fence to a
// dmabuf, which Publish found out and waits instead.
func stillFencing(t *testing.T, d *Device) {
	t.Helper()
	if !d.implicitFences {
		t.Skipf("the kernel attaches no sync_file to %s's dmabufs", d.Name())
	}
}

// A pass's target handed over by Publish carries a fence on its dmabuf
// that covers the pass, and Publish waits for nothing: a compositor
// sampling the buffer waits on that fence instead. The fence signals
// after the pass was submitted, or the pass had finished when the fence
// was exported, and not before the pass finished on the GPU.
func TestPublishAttachesThePassesFence(t *testing.T) {
	d := fencingDevice(t)
	tg := passTarget(t, d, busySize)
	plane := tg.Dmabuf().Planes[0].FD
	colors := []color.RGBA{{0, 255, 0, 255}, {0, 0, 255, 255}}
	for frame := range 8 {
		since := monotonic()
		heavyPass(t, d, tg, busyDraws, colors[frame%2])
		waits := d.BlockingWaits()
		if err := d.Publish(tg); err != nil {
			t.Fatal(err)
		}
		if got := d.BlockingWaits(); got != waits {
			t.Errorf("frame %d: Publish waited here %d times, want none", frame, got-waits)
		}
		stillFencing(t, d)
		if d.batch != 0 {
			t.Fatalf("frame %d: Publish left the pass unsubmitted", frame)
		}
		handedOut(d, dmabufFence(t, plane), since).signalled(t)
		if got := readTarget(t, tg).RGBAAt(1, 1); got != colors[frame%2] {
			t.Errorf("frame %d: the published target holds %v, want %v", frame, got, colors[frame%2])
		}
	}
}

// A pass submitted before Publish, by Flush, is covered by the fence
// Publish leaves on the dmabuf: Publish signals with a submission of its
// own, after the pass, and the fence signals after the pass was
// submitted, or the pass had finished when the fence was exported.
func TestPublishCoversPassesAlreadySubmitted(t *testing.T) {
	d := fencingDevice(t)
	tg := passTarget(t, d, busySize)
	since := monotonic()
	heavyPass(t, d, tg, busyDraws, color.RGBA{0, 0, 255, 255})
	if err := d.Flush(); err != nil {
		t.Fatal(err)
	}
	submitted := d.Submissions()
	if err := d.Publish(tg); err != nil {
		t.Fatal(err)
	}
	stillFencing(t, d)
	if got := d.Submissions(); got != submitted+1 {
		t.Errorf("Publish after Flush made %d submissions, want the one that signals", got-submitted)
	}
	handedOut(d, dmabufFence(t, tg.Dmabuf().Planes[0].FD), since).signalled(t)
	if got := readTarget(t, tg).RGBAAt(1, 1); got != (color.RGBA{0, 0, 255, 255}) {
		t.Errorf("the published target holds %v, want what the flushed pass drew", got)
	}
}

type judgement struct {
	testing.TB
	errors []string
	ended  string
}

func (j *judgement) Helper() {}

func (j *judgement) Errorf(format string, args ...any) {
	j.errors = append(j.errors, fmt.Sprintf(format, args...))
}

func (j *judgement) Fatalf(format string, args ...any) {
	j.ended = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

func (j *judgement) Skipf(format string, args ...any) { j.Fatalf(format, args...) }

func judge(t *testing.T, h handout) *judgement {
	j := &judgement{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.signalled(j)
	}()
	<-done
	return j
}

// A dmabuf's fence exported after it signalled is the kernel's stub, and
// the stub covers the pass when the pass had finished on the GPU by the
// export, whatever the device's fence status still says: NVIDIA's lags
// the sync_file it signals. A stub handed out before the pass finished
// does not cover it.
func TestAStubHandoutIsJudgedByWhenThePassFinishedOnTheGPU(t *testing.T) {
	d := fencingDevice(t)
	if !d.calibrated {
		t.Skipf("%s has no calibrated timestamps", d.Name())
	}
	tg := passTarget(t, d, busySize)
	plane := tg.Dmabuf().Planes[0].FD
	since := monotonic()
	heavyPass(t, d, tg, busyDraws, color.RGBA{0, 255, 0, 255})
	if err := d.Publish(tg); err != nil {
		t.Fatal(err)
	}
	stillFencing(t, d)
	waitReadable(t, dmabufFence(t, plane))
	d.pollFault = true
	t.Cleanup(func() { d.pollFault = false })
	if d.finished() == len(d.inflight) {
		t.Fatal("the device's fence status reports the pass finished while it is made to lag")
	}
	stub := handedOut(d, dmabufFence(t, plane), since)
	if at := signalTime(t, stub.fd); at > since {
		t.Fatalf("the fence exported after the pass's signalled at %v, after the pass began at %v: not the stub", at, since)
	}
	if j := judge(t, stub); len(j.errors) != 0 || j.ended != "" {
		t.Errorf("a stub of a finished pass, its fence status lagging: errors %q, ended %q; want it to cover the pass", j.errors, j.ended)
	}
	early := stub
	early.by = since
	if j := judge(t, early); len(j.errors) != 1 || j.ended != "" || !strings.Contains(j.errors[0], "does not cover it") {
		t.Errorf("a stub handed out before the pass finished: errors %q, ended %q; want one saying it does not cover the pass", j.errors, j.ended)
	}
}

// Where the consumer cannot be told by a fence (NVIDIA's driver, a
// device without sync_files), Publish hands the target over finished.
func TestPublishWithoutAFenceFinishesThePass(t *testing.T) {
	d := exportingDevice(t)
	d.implicitFences = false
	tg := passTarget(t, d, busySize)
	heavyPass(t, d, tg, busyDraws, color.RGBA{0, 255, 0, 255})
	if err := d.Publish(tg); err != nil {
		t.Fatal(err)
	}
	if n := len(d.inflight); n != 0 || d.batch != 0 {
		t.Errorf("%d submissions in flight and batch %v when Publish returned", n, d.batch != 0)
	}
}

// Publish refuses what it cannot hand over, by name, and submits an
// offscreen target's passes without more: it has no consumer.
func TestPublishRefusesMisuse(t *testing.T) {
	d := openDevice(t)
	tg := newTarget(t, d, 8, 8)
	clearPass(t, d, tg)
	if err := d.Publish(tg); err != nil {
		t.Fatalf("an offscreen target: %v", err)
	}
	if d.batch != 0 {
		t.Error("Publish left an offscreen target's pass unsubmitted")
	}
	p, err := d.Begin(tg, []image.Rectangle{{Max: tg.Size()}})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Publish(tg); err == nil || !strings.Contains(err.Error(), "pass is open") {
		t.Errorf("during a pass: %v, want an error saying a pass is open", err)
	}
	if err := p.End(); err != nil {
		t.Fatal(err)
	}
	other := openDevice(t)
	if err := other.Publish(tg); err == nil || !strings.Contains(err.Error(), "another device") {
		t.Errorf("another device's target: %v, want an error saying so", err)
	}
	closed, err := d.NewOffscreen(8, 8)
	if err != nil {
		t.Fatal(err)
	}
	closed.Close()
	for name, tg := range map[string]*Target{"closed": closed, "nil": nil} {
		if err := d.Publish(tg); err == nil || !strings.Contains(err.Error(), "closed target") {
			t.Errorf("a %s target: %v, want an error saying it is closed", name, err)
		}
	}
}

// A frame of a pass and its Publish allocates nothing, with a fence or
// without. Each frame finishes before the next, so the semaphores it
// signals are there to use again whatever the GPU's pace.
func TestPublishDoesNotAllocate(t *testing.T) {
	testbuild.SkipAllocBudget(t)
	for _, fenced := range []bool{true, false} {
		name := "waiting"
		if fenced {
			name = "fenced"
		}
		t.Run(name, func(t *testing.T) {
			d := syncDevice(t)
			d.implicitFences = fenced
			tg := passTarget(t, d, 64)
			damage := []image.Rectangle{{Max: tg.Size()}}
			frame := func() {
				p, err := d.Begin(tg, damage)
				if err != nil {
					t.Fatal(err)
				}
				p.Clear(color.RGBA{0, 255, 0, 255})
				if err := p.End(); err != nil {
					t.Fatal(err)
				}
				if err := d.Publish(tg); err != nil {
					t.Fatal(err)
				}
				if err := d.Finish(); err != nil {
					t.Fatal(err)
				}
			}
			for range 4 {
				frame()
			}
			if fenced {
				stillFencing(t, d)
			}
			if n := testing.AllocsPerRun(100, frame); n != 0 {
				t.Errorf("%.1f allocations a frame, want 0", n)
			}
		})
	}
}
