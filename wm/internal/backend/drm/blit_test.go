package drm

import (
	"errors"
	"fmt"
	"image"
	"slices"
	"strings"
	"testing"

	kdrm "github.com/stubbedev/lys/lib/drm"
	"github.com/stubbedev/lys/lib/fourcc"
	lgpu "github.com/stubbedev/lys/lib/gpu"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/wm/internal/backend"
	"github.com/stubbedev/lys/wm/internal/gpu"
	"github.com/stubbedev/lys/wm/internal/render"
	"github.com/stubbedev/lys/wm/internal/render/soft"
)

type blitWorld struct {
	memory  map[int]*soft.Buffer
	scanout map[int]bool
	log     []string
}

type fakeGBM struct {
	world *blitWorld
}

func (g fakeGBM) Allocate(size image.Point, format fourcc.Format, modifiers []fourcc.Modifier, usage gpu.Usage) (*gpu.Buffer, error) {
	fd, err := unix.MemfdCreate("blit", unix.MFD_CLOEXEC)
	if err != nil {
		return nil, err
	}
	modifier := fourcc.Linear
	if len(modifiers) > 0 {
		modifier = modifiers[0]
	}
	g.world.memory[fd] = soft.NewBuffer(size.X, size.Y)
	if usage&gpu.UseScanout != 0 {
		if g.world.scanout == nil {
			g.world.scanout = map[int]bool{}
		}
		g.world.scanout[fd] = true
	}
	d := gpu.DMABUF{Size: size, Format: format, Modifier: modifier, Planes: []gpu.Plane{{FD: fd, Stride: uint32(size.X * 4)}}}
	return gpu.NewBuffer(d, func() {}), nil
}

type sharedTexture struct {
	tex render.Texture
	buf *soft.Buffer
}

type fakeRenderer struct {
	*soft.Renderer
	imported  []sharedTexture
	world     *blitWorld
	name      string
	native    bool
	unlinear  bool
	mainError error
}

func (r *fakeRenderer) RenderableModifiers(fourcc.Format) ([]fourcc.Modifier, error) {
	return []fourcc.Modifier{fourcc.Linear}, nil
}

func (r *fakeRenderer) ImportTarget(b gpu.DMABUF) (render.ReadTarget, error) {
	if r.unlinear && b.Modifier == fourcc.Linear {
		return nil, fmt.Errorf("%s renders into no linear buffer", r.name)
	}
	return r.world.memory[b.Planes[0].FD], nil
}

func (r *fakeRenderer) ImportTexture(b gpu.DMABUF) (render.Texture, error) {
	buf := r.world.memory[b.Planes[0].FD]
	tex, err := r.NewTexture(render.ARGB8888, b.Size, buf.Stride, buf.Pix)
	if err == nil {
		r.imported = append(r.imported, sharedTexture{tex, buf})
	}
	return tex, err
}

func (r *fakeRenderer) Begin(target render.Target, damage render.Region) (render.Pass, error) {
	for _, s := range r.imported {
		if err := s.tex.Update(image.Rectangle{Max: s.tex.Size()}, s.buf.Stride, s.buf.Pix); err != nil {
			return nil, err
		}
	}
	return r.Renderer.Begin(target, damage)
}

func (r *fakeRenderer) MainDevice() (uint64, error) { return 0, r.mainError }

func (r *fakeRenderer) NativeFences() bool { return r.native }

func (r *fakeRenderer) SyncFile() (int, error) {
	r.world.log = append(r.world.log, r.name+" fence")
	return unix.MemfdCreate("fence", unix.MFD_CLOEXEC)
}

func (r *fakeRenderer) WaitFence(fd int) error {
	r.world.log = append(r.world.log, r.name+" wait")
	return unix.Close(fd)
}

func (r *fakeRenderer) Finish() error {
	r.world.log = append(r.world.log, r.name+" finish")
	return nil
}

func fakeStack(world *blitWorld, name string, native bool) *gpuAllocator {
	r := &fakeRenderer{Renderer: soft.New(), world: world, name: name, native: native, mainError: errors.New("this fake opens no Vulkan device")}
	return &gpuAllocator{dev: device{kernel: &fakeDevice{}}, path: "/dev/dri/" + name, mem: gbmMemory{gbm: fakeGBM{world}, gpu: r}, gpu: r, closeGPU: func() error { return nil }}
}

func TestABlitShowsTheFrameThePrimaryDrewWhicheverRendererEachCardRuns(t *testing.T) {
	size := image.Pt(16, 16)
	for _, c := range []struct {
		name           string
		primary, local bool
		want           []string
	}{
		{"both have fences", true, true, []string{"primary fence", "local wait"}},
		{"only the primary has fences", true, false, []string{"primary finish"}},
		{"only the local card has fences", false, true, []string{"primary finish"}},
		{"neither has fences", false, false, []string{"primary finish"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			world := &blitWorld{memory: map[int]*soft.Buffer{}}
			a := &blitAllocator{primary: fakeStack(world, "primary", c.primary), local: fakeStack(world, "local", c.local)}
			if err := a.probe(); err != nil {
				t.Fatal(err)
			}
			a.local.gpu.(*fakeRenderer).imported = nil
			s, err := a.share(size, scanoutFormat)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := s.release(); err != nil {
					t.Error(err)
				}
			})
			p, err := a.primary.gpu.Begin(s.target, render.Region{{Max: size}})
			if err != nil {
				t.Fatal(err)
			}
			p.FillRect(image.Rectangle{Max: size}, render.Color{R: 0xff, A: 0xff}, render.Mask{})
			if err := p.End(); err != nil {
				t.Fatal(err)
			}
			dst := soft.NewBuffer(size.X, size.Y)
			if err := a.blit(s, dst, size); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(world.log, c.want) {
				t.Errorf("handed the frame over with %v, want %v", world.log, c.want)
			}
			if got := [4]byte(dst.Pix[:4]); got != [4]byte{0, 0, 0xff, 0xff} {
				t.Errorf("the local card shows %v, want the primary's red", got)
			}
		})
	}
}

func TestASecondaryStaysOnTheFirstStackItsCardRuns(t *testing.T) {
	world := &blitWorld{memory: map[int]*soft.Buffer{}}
	primary := &Backend{alloc: fakeStack(world, "primary", true)}
	dev := device{kernel: kdrm.FromFD(-1, "/dev/dri/card1")}
	vulkanStack := &namedStack{"Vulkan", func(device) (*gpuAllocator, error) { return fakeStack(world, "local", true), nil }}
	noVulkan := &namedStack{"Vulkan", func(device) (*gpuAllocator, error) { return nil, errors.New("no Vulkan driver") }}
	unlinear := fakeStack(world, "primary", true)
	unlinear.gpu.(*fakeRenderer).unlinear = true
	for _, c := range []struct {
		name      string
		primary   *Backend
		stack     *namedStack
		canRender bool
		wantBlit  bool
		want      []string
	}{
		{"Vulkan on both", primary, vulkanStack, true, true, []string{"the primary GPU's linear buffers, or, where card1 refuses them, a GPU blit with Vulkan on card1"}},
		{"a card without Vulkan takes the CPU copy", primary, noVulkan, true, false, []string{"a CPU copy", "no Vulkan driver"}},
		{"a card beside a primary on the CPU takes the CPU copy", primary, nil, true, false, []string{"the primary GPU's linear buffers, or, where card1 refuses them, a CPU copy, since the primary renders on the CPU"}},
		{"a card without a render node takes the CPU copy", primary, vulkanStack, false, false, []string{"a CPU copy", "no render node"}},
		{"GPUs that share no buffer take the CPU copy", &Backend{alloc: unlinear}, vulkanStack, true, false, []string{"a CPU copy", "share no buffer", "renders into no linear buffer", "opens no Vulkan device"}},
		{"a primary that is no GPU stack takes the CPU copy", &Backend{alloc: &copyAllocator{primary: soft.New()}}, vulkanStack, true, false, []string{"a CPU copy", "shares no buffers"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			alloc, how := secondaryAllocator(dev, c.primary, c.stack, c.canRender)
			if first, ok := alloc.(*linearFirst); ok {
				alloc = first.own
			}
			if _, blit := alloc.(*lazyBlit); blit != c.wantBlit {
				t.Errorf("%T for %q", alloc, how)
			}
			for _, want := range c.want {
				if !strings.Contains(how, want) {
					t.Errorf("%q does not say %q", how, want)
				}
			}
		})
	}
}

func TestSecondariesRenderWithThePrimarysRendererAndNoOther(t *testing.T) {
	for _, c := range []struct {
		renderer Renderer
		want     string
	}{{Vulkan, "Vulkan"}, {Lavapipe, ""}} {
		got := ""
		if s := stackFor(c.renderer, gpu.Paths{}); s != nil {
			got = s.name
		}
		if got != c.want {
			t.Errorf("renderer %d gives secondaries the stack %q, want %q", c.renderer, got, c.want)
		}
	}
}

func TestAFileWhoseBuffersTheCardRefusesGivesWayToTheNext(t *testing.T) {
	file := func(name string) func() (*kdrm.Device, error) {
		return func() (*kdrm.Device, error) {
			fd, err := unix.MemfdCreate(name, unix.MFD_CLOEXEC)
			if err != nil {
				return nil, err
			}
			return kdrm.FromFD(fd, name), nil
		}
	}
	var opened, closed int
	open := func(int) (gbmFile, error) {
		opened++
		return gbmFile{close: func() error { closed++; return nil }}, nil
	}
	vouched := 0
	refuseFirst := func(gbmFile) error {
		vouched++
		if vouched == 1 {
			return errors.New("ADDFB2: invalid argument")
		}
		return nil
	}
	s, err := openGBMOnFile([]func() (*kdrm.Device, error){file("renderD128"), file("card0")}, open, refuseFirst)
	if err != nil {
		t.Fatal(err)
	}
	if opened != 2 || closed != 1 {
		t.Fatalf("opened %d stacks and closed %d, want the refused one closed and the next kept", opened, closed)
	}
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
	_, err = openGBMOnFile([]func() (*kdrm.Device, error){file("renderD128"), file("card0")}, open, func(gbmFile) error { return errors.New("ADDFB2: invalid argument") })
	if err == nil || !strings.Contains(err.Error(), "on renderD128: ADDFB2") || !strings.Contains(err.Error(), "on card0: ADDFB2") {
		t.Fatalf("no file whose buffers the card takes: %v", err)
	}
}

func TestASecondaryGPUResetRebuildsItsRendererAndAFourthEndsTheSession(t *testing.T) {
	size := image.Pt(16, 16)
	world := &blitWorld{memory: map[int]*soft.Buffer{}}
	a := &blitAllocator{primary: fakeStack(world, "primary", true), local: fakeStack(world, "local", true)}
	s, err := a.share(size, scanoutFormat)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.release(); err != nil {
			t.Error(err)
		}
	})
	dst := soft.NewBuffer(size.X, size.Y)
	if err := a.blit(s, dst, size); err != nil {
		t.Fatal(err)
	}
	for i := range lgpu.LossLimit {
		a.local.gpu.LoseDevice()
		err := a.blit(s, dst, size)
		if !backend.GPULost(err) || backend.Fatal(err) || !strings.Contains(err.Error(), "renderer rebuilt") {
			t.Fatalf("loss %d: the blit gave %v, want a recovered reset", i, err)
		}
		if err := a.local.gpu.Lost(); err != nil {
			t.Fatalf("loss %d: the local renderer is still lost: %v", i, err)
		}
		if err := a.blit(s, dst, size); err != nil {
			t.Fatalf("loss %d: the blit after the reset: %v", i, err)
		}
	}
	a.local.gpu.LoseDevice()
	if err := a.blit(s, dst, size); !backend.Fatal(err) || strings.Count(err.Error(), "raised by LoseDevice") != lgpu.LossLimit+1 {
		t.Fatalf("the fourth loss gave %v, want a fatal error naming all four", err)
	}
}
