package nested

import (
	"errors"
	"fmt"
	"image"
	"slices"

	"github.com/stubbedev/lys/lib/fourcc"

	"github.com/stubbedev/lys/wm/internal/gpu"
	ld "github.com/stubbedev/lys/wm/internal/protocol/client/linuxdmabuf"
	cw "github.com/stubbedev/lys/wm/internal/protocol/client/wayland"
	"github.com/stubbedev/lys/wm/internal/render"
	"github.com/stubbedev/lys/wm/internal/render/vulkan"
	"github.com/stubbedev/lys/wm/internal/wire"
	"github.com/stubbedev/lys/wm/internal/wlclient"
	"github.com/stubbedev/lys/wm/internal/wlclient/feedback"
)

const (
	dmabufVersion    = 4
	dmabufMinVersion = 3
	dmabufBuffers    = 2
	dmabufFormat     = fourcc.XRGB8888
	dmabufHostName   = "zwp_linux_dmabuf_v1"
)

type sharedBuffer struct {
	target  render.Target
	dmabuf  gpu.DMABUF
	prepare func() error
	free    func() error
}

type dmabufChain struct {
	linux     *ld.ZwpLinuxDmabufV1
	stack     *vulkanStack
	modifiers []fourcc.Modifier
}

func hostFormats(host *wlclient.Display, globals registry, reg *cw.WlRegistry) (*ld.ZwpLinuxDmabufV1, feedback.Feedback, error) {
	g, ok := globals[dmabufHostName]
	if !ok || g.version < dmabufMinVersion {
		return nil, feedback.Feedback{}, fmt.Errorf("nested: rendering on a GPU needs the host's %s version %d", dmabufHostName, dmabufMinVersion)
	}
	events := &dmabufEvents{}
	linux, err := ld.NewZwpLinuxDmabufV1(host, min(g.version, dmabufVersion), events)
	if err != nil {
		return nil, feedback.Feedback{}, err
	}
	if err := reg.Bind(g.name, linux); err != nil {
		return nil, feedback.Feedback{}, err
	}
	if g.version >= dmabufVersion {
		fb, err := feedback.Default(host, linux)
		if err != nil {
			return nil, feedback.Feedback{}, errors.Join(fmt.Errorf("nested: %w", err), linux.Destroy())
		}
		return linux, fb, nil
	}
	if err := host.Roundtrip(); err != nil {
		return nil, feedback.Feedback{}, errors.Join(fmt.Errorf("nested: list the host's dmabuf modifiers: %w", err), linux.Destroy())
	}
	return linux, feedback.Feedback{Tranches: []feedback.Tranche{{Formats: events.formats}}}, nil
}

func newDMABUFChain(host *wlclient.Display, globals registry, reg *cw.WlRegistry) (*dmabufChain, error) {
	linux, fb, err := hostFormats(host, globals, reg)
	if err != nil {
		return nil, err
	}
	stack, err := openVulkan(fb.Main)
	if err != nil {
		return nil, errors.Join(err, linux.Destroy())
	}
	modifiers, err := stack.modifiers(fb)
	if err != nil {
		return nil, errors.Join(err, stack.close(), linux.Destroy())
	}
	slices.Sort(modifiers)
	return &dmabufChain{linux: linux, stack: stack, modifiers: modifiers}, nil
}

func hostImports(host feedback.Feedback, rendered []fourcc.Modifier) []fourcc.Modifier {
	return slices.DeleteFunc(slices.Clone(rendered), func(m fourcc.Modifier) bool {
		return !host.Imports(gpu.FormatModifier{Format: dmabufFormat, Modifier: m})
	})
}

// hostLayout is how a buffer reaches a host that imports no modifier the
// device renders in: the bytes are linear, which is what an import with
// no modifier and no kernel metadata reads as, on every driver with an
// implicit path.
type hostLayout struct {
	direct   bool
	modifier fourcc.Modifier
	copied   bool
}

// chooseHostLayout decides how to share buffers with a host: directly in
// a modifier both take, or as linear bytes under what the host calls
// them, rendered in place where the device renders linear and otherwise
// copied into. Vulkan has no import or export of the implicit modifier,
// so it is only ever how the host is told about linear bytes.
func chooseHostLayout(host feedback.Feedback, rendered, shared []fourcc.Modifier) (hostLayout, bool) {
	if len(shared) > 0 {
		return hostLayout{direct: true}, true
	}
	switch {
	case host.Imports(gpu.FormatModifier{Format: dmabufFormat, Modifier: fourcc.Linear}):
		return hostLayout{modifier: fourcc.Linear, copied: true}, true
	case host.Imports(gpu.FormatModifier{Format: dmabufFormat, Modifier: fourcc.Invalid}):
		return hostLayout{modifier: fourcc.Invalid, copied: !slices.Contains(rendered, fourcc.Linear)}, true
	}
	return hostLayout{}, false
}

type vulkanStack struct {
	r      *vulkan.Renderer
	layout hostLayout
}

func openVulkan(main uint64) (*vulkanStack, error) {
	var (
		r   *vulkan.Renderer
		err error
	)
	if main != 0 {
		r, err = vulkan.Open(main)
	} else {
		r, err = vulkan.OpenDefault()
	}
	if err != nil {
		return nil, fmt.Errorf("nested: %w", err)
	}
	return &vulkanStack{r: r}, nil
}

func (s *vulkanStack) modifiers(host feedback.Feedback) ([]fourcc.Modifier, error) {
	if !s.r.Device().Exports() {
		return nil, fmt.Errorf("nested: %s shares no dmabufs with the host", s.r.Device().Name())
	}
	rendered, err := s.r.ExportModifiers(dmabufFormat)
	if err != nil {
		return nil, fmt.Errorf("nested: %s: %w", s.r.Device().Name(), err)
	}
	shared := hostImports(host, rendered)
	layout, ok := chooseHostLayout(host, rendered, shared)
	if !ok {
		return nil, fmt.Errorf("nested: the host imports %v in modifiers %v, %s renders it in %v, none of them shared, and the host takes neither linear nor implicit buffers", dmabufFormat, host.Modifiers(dmabufFormat), s.r.Device().Name(), rendered)
	}
	s.layout = layout
	return shared, nil
}

func (s *vulkanStack) allocate(size image.Point, modifiers []fourcc.Modifier) (sharedBuffer, error) {
	switch {
	case s.layout.direct:
		return s.exported(size, modifiers)
	case s.layout.copied:
		c, err := s.r.NewLinearCopyTarget(size, dmabufFormat)
		if err != nil {
			return sharedBuffer{}, err
		}
		return s.toldAs(sharedBuffer{target: c.Target, dmabuf: c.Buffer.DMABUF, prepare: c.Copy, free: func() error { return errors.Join(c.Free(), c.Buffer.Close()) }}), nil
	}
	b, err := s.exported(size, []fourcc.Modifier{fourcc.Linear})
	if err != nil {
		return sharedBuffer{}, err
	}
	return s.toldAs(b), nil
}

func (s *vulkanStack) exported(size image.Point, modifiers []fourcc.Modifier) (sharedBuffer, error) {
	target, shared, err := s.r.NewExportedTarget(size, dmabufFormat, modifiers)
	if err != nil {
		return sharedBuffer{}, err
	}
	return sharedBuffer{target: target, dmabuf: shared.DMABUF, free: func() error { return errors.Join(target.Destroy(), shared.Close()) }}, nil
}

func (s *vulkanStack) toldAs(b sharedBuffer) sharedBuffer {
	b.dmabuf.Modifier = s.layout.modifier
	return b
}

func (s *vulkanStack) close() error {
	return s.r.Close()
}

func (c *dmabufChain) renderer() render.Renderer {
	return c.stack.r
}

func (c *dmabufChain) finish() error {
	return c.stack.r.Finish()
}

func (c *dmabufChain) allocate(o *Output, size image.Point) ([]*buffer, func() error, error) {
	var shared []sharedBuffer
	free := func() error {
		errs := make([]error, 0, len(shared))
		for _, s := range shared {
			errs = append(errs, s.free())
		}
		return errors.Join(errs...)
	}
	buffers := make([]*buffer, dmabufBuffers)
	for i := range buffers {
		s, err := c.stack.allocate(size, c.modifiers)
		if err != nil {
			return nil, nil, errors.Join(err, free())
		}
		shared = append(shared, s)
		buf := &buffer{target: s.target, prepare: s.prepare}
		if buf.wl, err = c.hostBuffer(o, buf, s.dmabuf); err != nil {
			return nil, nil, errors.Join(err, free())
		}
		buffers[i] = buf
	}
	return buffers, free, nil
}

func (c *dmabufChain) hostBuffer(o *Output, buf *buffer, d gpu.DMABUF) (*cw.WlBuffer, error) {
	params, err := c.linux.CreateParams(paramsEvents{})
	if err != nil {
		return nil, err
	}
	hi, lo := wire.Halves(uint64(d.Modifier))
	for i, p := range d.Planes {
		if err := params.Add(p.FD, uint32(i), p.Offset, p.Stride, hi, lo); err != nil {
			return nil, err
		}
	}
	wl, err := params.CreateImmed(bufferEvents{o, buf}, int32(d.Size.X), int32(d.Size.Y), uint32(d.Format), 0) //nolint:gosec // buffer sizes fit the protocol
	if err != nil {
		return nil, err
	}
	return wl, params.Destroy()
}

func (c *dmabufChain) close() error {
	return errors.Join(c.stack.close(), c.linux.Destroy())
}

type dmabufEvents struct {
	formats []gpu.FormatModifier
}

func (*dmabufEvents) Format(*ld.ZwpLinuxDmabufV1, uint32) error { return nil }

func (e *dmabufEvents) Modifier(_ *ld.ZwpLinuxDmabufV1, format, hi, lo uint32) error {
	e.formats = append(e.formats, gpu.FormatModifier{Format: fourcc.Format(format), Modifier: fourcc.Modifier(uint64(hi)<<32 | uint64(lo))})
	return nil
}

type paramsEvents struct{}

func (paramsEvents) Created(*ld.ZwpLinuxBufferParamsV1, *cw.WlBuffer) (cw.WlBufferHandler, error) {
	return nil, errors.New("nested: the host announced a buffer lys-wm created with create_immed")
}

func (paramsEvents) Failed(*ld.ZwpLinuxBufferParamsV1) error {
	return errors.New("nested: the host rejected a dmabuf buffer")
}
