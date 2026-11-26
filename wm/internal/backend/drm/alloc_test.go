package drm

import (
	"errors"
	"image"
	"testing"

	"github.com/stubbedev/lys/lib/fourcc"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/wm/internal/gpu"
	"github.com/stubbedev/lys/wm/internal/render/soft"
)

func TestTheProbeTurnsAScanoutBufferIntoAFramebufferAndRemovesIt(t *testing.T) {
	a := fakeStack(&blitWorld{memory: map[int]*soft.Buffer{}}, "card", true)
	dev := a.dev.kernel.(*fakeDevice)
	if err := a.probe(); err != nil {
		t.Fatal(err)
	}
	if dev.fbs != 1 {
		t.Fatalf("the probe created %d framebuffers, want 1", dev.fbs)
	}
	if len(dev.removedFBs) != 1 {
		t.Fatalf("the probe left %d framebuffers removed, want the 1 it created", len(dev.removedFBs))
	}
}

func TestTheProbeBufferIsAsLargeAsTheCardsSmallestFramebuffer(t *testing.T) {
	a := fakeStack(&blitWorld{memory: map[int]*soft.Buffer{}}, "card", true)
	dev := a.dev.kernel.(*fakeDevice)
	dev.minSize = [2]uint32{32, 24}
	if err := a.probe(); err != nil {
		t.Fatalf("a card whose framebuffers are at least 32x24 (virtio-gpu's are 32x32) refused the probe: %v", err)
	}
	if dev.fbs != 1 {
		t.Fatalf("the probe created %d framebuffers, want 1", dev.fbs)
	}
}

func TestTheProbeFailsWhenTheCardTakesNoFramebufferFromAScanoutBuffer(t *testing.T) {
	a := fakeStack(&blitWorld{memory: map[int]*soft.Buffer{}}, "card", true)
	a.dev.kernel.(*fakeDevice).refuseAddFB = true
	if err := a.probe(); !errors.Is(err, unix.EINVAL) {
		t.Fatalf("probe() = %v, want the card's EINVAL: an allocator whose buffers never reach the screen must not pass", err)
	}
}

type rendersNoLinear struct {
	fakeGBM
}

func (g rendersNoLinear) Allocate(size image.Point, format fourcc.Format, modifiers []fourcc.Modifier, usage gpu.Usage) (*gpu.Buffer, error) {
	if usage&gpu.UseRendering != 0 {
		return nil, errors.New("this GBM's GPU renders into no linear buffer")
	}
	return g.fakeGBM.Allocate(size, format, modifiers, usage)
}

func TestTheCPUDrawsIntoLinearScanoutBuffersOfACardWhoseGPURendersIntoNone(t *testing.T) {
	for _, c := range []struct {
		drawer drawer
		ok     bool
	}{{cpuDraws, true}, {gbmDeviceDraws, false}} {
		a := fakeStack(&blitWorld{memory: map[int]*soft.Buffer{}}, "card", true)
		m := a.mem.(gbmMemory)
		m.gbm, m.drawer = rendersNoLinear{m.gbm.(fakeGBM)}, c.drawer
		a.mem = m
		if err := a.probe(); (err == nil) != c.ok {
			t.Errorf("drawer %d: probe() = %v, want success %v: GBM must be asked to render only into buffers its own GPU draws", c.drawer, err, c.ok)
		}
	}
}
