package drm

import (
	"strings"
	"testing"

	kdrm "github.com/stubbedev/lys/lib/drm"
	"github.com/stubbedev/lys/lib/fourcc"

	"github.com/stubbedev/lys/wm/internal/backend"
)

func (r *rig) liveBuffers() int {
	return int(r.alloc.next) - r.alloc.freed
}

func (r *rig) power(o *Output, p backend.Power) error {
	r.t.Helper()
	state := o.State()
	state.Power = p
	return o.Commit(state)
}

func (r *rig) poweredOn(o *Output, seq uint32) {
	r.t.Helper()
	if err := r.power(o, backend.PoweredOn); err != nil {
		r.t.Fatal(err)
	}
	r.until("the power-on modeset", func() bool {
		return o.flipping && r.dev.commits[len(r.dev.commits)-1].values[[2]uint32{10, propActive}] == 1
	})
	r.dev.flip(r.t, 10, seq)
	r.until("the power-on flip", func() bool { return !o.flipping })
}

func TestEveryPowerOffFreesTheOutputsBuffersAndEveryPowerOnDrawsInAFreshSet(t *testing.T) {
	r := start(t)
	o := r.lit()
	if live := r.liveBuffers(); live != buffersPerOut {
		t.Fatalf("a lit output holds %d buffers, want %d", live, buffersPerOut)
	}
	const cycles = 4
	for i := range cycles {
		if err := r.power(o, backend.PoweredOff); err != nil {
			t.Fatal(err)
		}
		if live := r.liveBuffers(); live != 0 || len(o.buffers) != 0 || o.front != nil || o.queued != nil {
			t.Fatalf("cycle %d: a powered-off output holds %d buffers, %d listed, front %v, queued %v", i, live, len(o.buffers), o.front, o.queued)
		}
		r.poweredOn(o, uint32(2+i))
		if live := r.liveBuffers(); live != buffersPerOut || len(o.buffers) != buffersPerOut {
			t.Fatalf("cycle %d: a powered-on output holds %d buffers, %d listed, want %d", i, live, len(o.buffers), buffersPerOut)
		}
	}
	if allocated := int(r.alloc.next); allocated != buffersPerOut*(cycles+1) {
		t.Fatalf("%d power cycles allocated %d buffers, want a fresh set of %d each", cycles, allocated, buffersPerOut)
	}
	if len(r.reports) != 0 {
		t.Fatalf("reports %v", r.reports)
	}
}

func TestAPowerOffDuringAFlipFreesTheBuffersOnceTheFlipLands(t *testing.T) {
	r := start(t)
	o := r.lit()
	frames := len(r.dev.commits)
	if err := o.ScheduleFrame(); err != nil {
		t.Fatal(err)
	}
	r.committedSince("a frame in flight", o, frames)
	if err := r.power(o, backend.PoweredOff); err != nil {
		t.Fatal(err)
	}
	if live := r.liveBuffers(); live != buffersPerOut {
		t.Fatalf("buffers the kernel still scans out were freed before their flip: %d left", live)
	}
	r.dev.flip(t, 10, 2)
	r.until("the power off after the flip", func() bool { return !o.offDue })
	if live := r.liveBuffers(); live != 0 || len(o.buffers) != 0 {
		t.Fatalf("an output powered off after its flip holds %d buffers, %d listed", live, len(o.buffers))
	}
}

func TestARefusedPowerOffKeepsTheBuffersTheOutputStillShows(t *testing.T) {
	r := start(t)
	o := r.lit()
	r.dev.refusePowerOff = true
	if err := r.power(o, backend.PoweredOff); err == nil {
		t.Fatal("a refused power off succeeded")
	}
	if live := r.liveBuffers(); live != buffersPerOut || len(o.buffers) != buffersPerOut || r.alloc.freed != 0 {
		t.Fatalf("a refused power off left %d buffers, %d listed, %d freed", live, len(o.buffers), r.alloc.freed)
	}
}

func TestAPowerOffHeldBusyFreesTheBuffersWhenItCommits(t *testing.T) {
	r := start(t)
	_, second := r.twoOutputsOneFlipping()
	r.flipSecondsFirstFrame(second)
	r.dev.busyUntilFlip = true
	held := len(second.buffers)
	freed := r.alloc.freed
	if err := r.power(second, backend.PoweredOff); err != nil {
		t.Fatal(err)
	}
	if !second.offBusy || len(second.buffers) != held || r.alloc.freed != freed {
		t.Fatalf("a power off held busy freed %d buffers, %d of %d listed", r.alloc.freed-freed, len(second.buffers), held)
	}
	r.dev.flip(t, 10, 2)
	r.until("DP-2's power off after DP-1's flip", func() bool { return !second.offBusy })
	if len(second.buffers) != 0 || r.alloc.freed != freed+held {
		t.Fatalf("a held power off freed %d buffers, %d still listed, want all %d freed", r.alloc.freed-freed, len(second.buffers), held)
	}
}

func TestAPowerOnTheAllocatorRefusesLeaksNothingAndDrawsOnceItAllocates(t *testing.T) {
	r := start(t)
	o := r.lit()
	if err := r.power(o, backend.PoweredOff); err != nil {
		t.Fatal(err)
	}
	r.rec.idle = true
	r.alloc.buffersFor = map[fourcc.Format]int{scanoutFormat: buffersPerOut - 1}
	if err := r.power(o, backend.PoweredOn); err != nil {
		t.Fatal(err)
	}
	allocated := r.alloc.next
	if _, err := o.BeginFrame(); err == nil || !strings.Contains(err.Error(), "the allocator refuses") {
		t.Fatalf("a frame the allocator cannot back began: %v", err)
	}
	if r.alloc.next == allocated {
		t.Fatal("the refused power-on allocated nothing before the refusal")
	}
	if live := r.liveBuffers(); live != 0 || len(o.buffers) != 0 || o.drawing != nil {
		t.Fatalf("a refused power-on holds %d buffers, %d listed, drawing %v", live, len(o.buffers), o.drawing)
	}
	delete(r.alloc.buffersFor, scanoutFormat)
	r.rec.idle = false
	if err := o.ScheduleFrame(); err != nil {
		t.Fatal(err)
	}
	r.until("the power-on modeset once the allocator recovers", func() bool {
		return o.flipping && r.dev.commits[len(r.dev.commits)-1].values[[2]uint32{10, propActive}] == 1
	})
	if c := r.dev.commits[len(r.dev.commits)-1]; c.flags&kdrm.AtomicAllowModeset == 0 {
		t.Fatalf("the first frame after a refused power-on committed %+v, not a modeset", c)
	}
	if live := r.liveBuffers(); live != buffersPerOut || len(o.buffers) != buffersPerOut {
		t.Fatalf("the recovered power-on holds %d buffers, %d listed, want %d", live, len(o.buffers), buffersPerOut)
	}
}
