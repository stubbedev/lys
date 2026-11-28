package drm

import (
	"errors"
	"image"
	"slices"
	"testing"

	kdrm "github.com/stubbedev/lys/lib/drm"
	"github.com/stubbedev/lys/lib/fourcc"

	"github.com/stubbedev/lys/wm/internal/backend"
	"github.com/stubbedev/lys/wm/internal/gpu"
	"github.com/stubbedev/lys/wm/internal/render/soft"
)

func (r *rig) twoOutputsOneFlipping() (flipping, other *Output) {
	r.t.Helper()
	flipping = r.flipFirstFrame()
	r.dev.connectors = []uint32{20, 21}
	if err := r.hotplug(); err != nil {
		r.t.Fatal(err)
	}
	if len(r.b.Outputs()) != 2 {
		r.t.Fatalf("outputs %v, reports %v", r.rec.log, r.reports)
	}
	other = r.b.Outputs()[1]
	if err := flipping.ScheduleFrame(); err != nil {
		r.t.Fatal(err)
	}
	r.committedSince("DP-1's frame in flight", flipping, 0)
	r.dev.commits, r.rec.log = nil, nil
	return flipping, other
}

func (r *rig) flipSecondsFirstFrame(second *Output) {
	r.t.Helper()
	if err := second.ScheduleFrame(); err != nil {
		r.t.Fatal(err)
	}
	r.committedSince("DP-2's first frame", second, 0)
	r.dev.flip(r.t, 11, 1)
	r.until("DP-2's flip", func() bool { return !second.inFlight() })
	r.dev.commits, r.rec.log = nil, nil
}

func TestAFlipRefusedBusyWaitsForTheDevicesFlips(t *testing.T) {
	r := start(t)
	first, second := r.twoOutputsOneFlipping()
	r.flipSecondsFirstFrame(second)
	r.dev.busyFlips = true
	if err := second.ScheduleFrame(); err != nil {
		t.Fatal(err)
	}
	r.until("DP-2's flip refused busy", func() bool { return second.busy != nil })
	r.settle()
	if second.inFlight() || slices.Contains(r.rec.log, "damaged DP-2") || len(r.reports) != 0 {
		t.Fatalf("DP-2 flipping %v, log %v, reports %v", second.inFlight(), r.rec.log, r.reports)
	}
	refused := len(r.dev.commits)
	r.dev.flip(t, 10, 2)
	r.until("DP-2's flip after DP-1's", func() bool { return second.inFlight() })
	got := slices.DeleteFunc(slices.Clone(r.dev.commits[refused:]), func(c commit) bool { return c.flags&kdrm.AtomicTestOnly != 0 })
	if len(got) != 1 || got[0].userData != 11 || got[0].flags != planeFlags {
		t.Fatalf("DP-2's held frame committed %+v", got)
	}
	frames := 0
	for _, l := range r.rec.log {
		switch l {
		case "frame":
			frames++
		case "damaged DP-2":
			t.Fatalf("DP-2 dropped a frame: log %v", r.rec.log)
		}
	}
	if frames != 1 || len(r.reports) != 0 || first.inFlight() {
		t.Fatalf("%d renders, reports %v, log %v", frames, r.reports, r.rec.log)
	}
}

func TestACursorCommitRefusedBusyWaitsForTheDevicesFlips(t *testing.T) {
	r := start(t)
	first, second := r.twoOutputsOneFlipping()
	r.dev.flip(t, 10, 2)
	r.until("DP-1's flip", func() bool { return !first.inFlight() })
	if err := second.ScheduleFrame(); err != nil {
		t.Fatal(err)
	}
	r.committedSince("DP-2's first frame", second, 0)
	r.elapse(first.planesWait())
	c := r.shownCursor(first, image.Pt(4, 6))
	r.dev.flip(t, 10, 3)
	r.until("DP-1's cursor flip", func() bool { return !first.inFlight() })
	r.elapse(first.planesWait())
	r.dev.busyFlips = true
	refused := len(r.dev.commits)
	if err := c.MoveCursor(image.Pt(30, 20)); err != nil {
		t.Fatal(err)
	}
	r.until("DP-1's cursor commit refused busy", func() bool { return first.planesBusy })
	if first.inFlight() || len(r.reports) != 0 || !c.dirty {
		t.Fatalf("a busy cursor commit: flipping %v, dirty %v, reports %v", first.inFlight(), c.dirty, r.reports)
	}
	r.dev.flip(t, 11, 1)
	r.until("DP-1's cursor after DP-2's flip", func() bool { return first.inFlight() })
	if _, at, _ := cursorAt(r.dev.commits[len(r.dev.commits)-1]); at != image.Pt(26, 14) || len(r.dev.commits) != refused+2 || len(r.reports) != 0 {
		t.Fatalf("DP-1's held cursor at %v after commits %+v, reports %v", at, r.dev.commits[refused:], r.reports)
	}
}

func TestAModesetRefusedBusyWaitsForTheDevicesFlips(t *testing.T) {
	r := start(t)
	first, second := r.twoOutputsOneFlipping()
	r.dev.busyUntilFlip = true
	if err := second.ScheduleFrame(); err != nil {
		t.Fatal(err)
	}
	r.until("DP-2's modeset refused busy", func() bool { return second.busy != nil })
	r.settle()
	if second.inFlight() || len(r.dev.commits) != 2 || slices.Contains(r.rec.log, "damaged DP-2") {
		t.Fatalf("DP-2 flipping %v after %d commits, log %v", second.inFlight(), len(r.dev.commits), r.rec.log)
	}
	r.dev.flip(t, 10, 2)
	r.until("DP-2's modeset after DP-1's flip", func() bool { return second.inFlight() })
	modeset := r.dev.commits[len(r.dev.commits)-1]
	if modeset.userData != 11 || modeset.flags != planeFlags|kdrm.AtomicAllowModeset || modeset.values[[2]uint32{11, propActive}] != 1 {
		t.Fatalf("DP-2's modeset %+v", modeset)
	}
	frames := 0
	for _, l := range r.rec.log {
		switch l {
		case "frame":
			frames++
		case "damaged DP-2":
			t.Fatalf("DP-2 dropped a frame: log %v", r.rec.log)
		}
	}
	if frames != 1 || len(r.reports) != 0 || first.inFlight() {
		t.Fatalf("%d renders, reports %v, log %v", frames, r.reports, r.rec.log)
	}
}

func TestAModesetRefusedBusyWithNoFlipPendingCommitsBlocking(t *testing.T) {
	r := start(t)
	o := r.b.Outputs()[0]
	r.dev.busyNonblock = true
	if err := o.ScheduleFrame(); err != nil {
		t.Fatal(err)
	}
	r.committedSince("the blocking modeset", o, 0)
	got := r.dev.commits[len(r.dev.commits)-1]
	if got.flags != kdrm.AtomicPageFlipEvent|kdrm.AtomicAllowModeset || o.busy != nil {
		t.Fatalf("the modeset after EBUSY committed %+v", got)
	}
}

func TestAPowerOffRefusedBusyWaitsForTheDevicesFlips(t *testing.T) {
	r := start(t)
	_, second := r.twoOutputsOneFlipping()
	r.flipSecondsFirstFrame(second)
	r.dev.busyUntilFlip = true
	n := len(r.dev.commits)
	state := second.State()
	state.Power = backend.PoweredOff
	if err := second.Commit(state); err != nil {
		t.Fatal(err)
	}
	if second.power != offAfterBusy || second.State().Power != backend.PoweredOff || len(r.dev.commits) != n+1 {
		t.Fatalf("a busy power off: held %v, state %+v, commits %+v", second.power == offAfterBusy, second.State(), r.dev.commits[n:])
	}
	r.dev.flip(t, 10, 2)
	r.until("DP-2's power off after DP-1's flip", func() bool { return second.power != offAfterBusy })
	off := r.dev.commits[len(r.dev.commits)-1]
	if off.flags != kdrm.AtomicAllowModeset|kdrm.AtomicNonblock || off.values[[2]uint32{11, propActive}] != 0 || len(r.reports) != 0 {
		t.Fatalf("power off commit %+v, reports %v", off, r.reports)
	}
}

func TestAPowerOffRefusedBusyWithNoFlipPendingCommitsBlocking(t *testing.T) {
	r := start(t)
	o := r.lit()
	r.dev.busyNonblock = true
	n := len(r.dev.commits)
	state := o.State()
	state.Power = backend.PoweredOff
	if err := o.Commit(state); err != nil {
		t.Fatal(err)
	}
	if got := r.dev.commits[n:]; len(got) != 2 || got[1].flags != kdrm.AtomicAllowModeset || o.power == offAfterBusy {
		t.Fatalf("a busy power off with nothing flipping committed %+v", got)
	}
}

func TestAScanoutRefusedBusyIsComposited(t *testing.T) {
	r := start(t)
	_, second := r.twoOutputsOneFlipping()
	r.flipSecondsFirstFrame(second)
	r.dev.busyFlips = true
	size := modeOf(second.mode).Size
	client := gpu.DMABUF{Size: size, Format: scanoutFormat, Modifier: fourcc.Linear, Planes: []gpu.Plane{{FD: 7, Stride: uint32(size.X) * 4}}}
	if ok, err := second.Scanout(&fakeBuffer{client}, client, backend.Whole(client.Size), func() error { return nil }, false); err != nil || ok {
		t.Fatalf("Scanout refused busy = %t, %v, want the frame composited", ok, err)
	}
}

func (r *rig) copiedFirstFrame(primary *asyncPrimary, o *Output, crtc uint32) {
	r.t.Helper()
	if err := o.ScheduleFrame(); err != nil {
		r.t.Fatal(err)
	}
	read := len(primary.gates)
	r.until(o.Name()+"'s first read", func() bool { return len(primary.gates) > read })
	n := len(r.dev.commits)
	primary.open(read, nil)
	r.committedSince(o.Name()+"'s first frame", o, n)
	r.dev.flip(r.t, crtc, 1)
	r.until(o.Name()+"'s first flip", func() bool { return !o.inFlight() })
}

func TestAFlipRefusedBusyBehindAFrameStillCopyingCommitsOnceTheCopyFails(t *testing.T) {
	primary := asyncPrimaryOf(soft.New())
	primary.gated = true
	r := boot(t, func(dev *fakeDevice) allocator { return &copyAllocator{dev: device{kernel: dev}, primary: primary} })
	r.copiesOffLoop(primary)
	r.dev.connectors = []uint32{20, 21}
	if err := r.hotplug(); err != nil {
		t.Fatal(err)
	}
	if len(r.b.Outputs()) != 2 {
		t.Fatalf("outputs %v, reports %v", r.rec.log, r.reports)
	}
	copying, refused := r.b.Outputs()[0], r.b.Outputs()[1]
	r.copiedFirstFrame(primary, copying, 10)
	r.copiedFirstFrame(primary, refused, 11)
	if err := copying.ScheduleFrame(); err != nil {
		t.Fatal(err)
	}
	r.until("DP-1's read", func() bool { return len(primary.gates) == 3 })
	if err := refused.ScheduleFrame(); err != nil {
		t.Fatal(err)
	}
	r.until("DP-2's read", func() bool { return len(primary.gates) == 4 })
	r.dev.busyFlips = true
	n := len(r.dev.commits)
	primary.open(3, nil)
	r.until("DP-2's commit", func() bool { return len(r.dev.commits) > n })
	if why := r.b.Flipping(); why != "output DP-1 is copying a frame to its device" {
		t.Fatalf("Flipping() = %q while DP-1's frame is copied", why)
	}
	primary.open(2, errors.New("the GPU lost the read"))
	r.until("DP-1's failed copy", func() bool { return len(r.reports) > 0 })
	r.settle()
	if refused.busy != nil {
		t.Fatalf("DP-2's frame refused busy is still held after DP-1's copy failed with no flip on the device: commits %+v, reports %v", r.dev.commits[n:], r.reports)
	}
	got := slices.DeleteFunc(slices.Clone(r.dev.commits[n:]), func(c commit) bool { return c.flags&kdrm.AtomicTestOnly != 0 || c.userData != 11 })
	if len(got) != 2 || got[0].flags != planeFlags || got[1].flags != planeFlags&^kdrm.AtomicNonblock || !refused.inFlight() {
		t.Fatalf("DP-2's frame refused busy committed %+v, flipping %v", got, refused.inFlight())
	}
	if !slices.Contains(r.rec.log, "damaged DP-1") || slices.Contains(r.rec.log, "damaged DP-2") {
		t.Fatalf("log %v, want DP-1's lost frame redrawn and DP-2's kept", r.rec.log)
	}
}
