package compositor

import (
	"errors"
	"fmt"
	"image"
	"time"

	"github.com/stubbedev/lys/lib/dmabuf"
	"github.com/stubbedev/lys/lib/fourcc"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/wm/internal/anim"
	"github.com/stubbedev/lys/wm/internal/backend"
	"github.com/stubbedev/lys/wm/internal/gpu"
	"github.com/stubbedev/lys/wm/internal/output"
	"github.com/stubbedev/lys/wm/internal/render"
)

type inheritedFade struct {
	frame *gpu.Buffer
	tex   render.Texture
	alpha anim.Tween[float64]
}

const DefaultInheritTimeout = 30 * time.Second

func showsInherited(o *output.Output) bool {
	in, ok := o.Backend().(backend.Inheriting)
	return ok && in.Inherited()
}

func (c *Compositor) keepInherited(o *output.Output) error {
	if !showsInherited(o) {
		return nil
	}
	t, err := c.loop.AddTimer(func() error { return c.inheritTimedOut(o) })
	if err != nil {
		return fmt.Errorf("compositor: time the frame %s inherited: %w", o.Backend().Name(), err)
	}
	if err := t.Arm(c.inheritTimeout); err != nil {
		return errors.Join(fmt.Errorf("compositor: time the frame %s inherited: %w", o.Backend().Name(), err), t.Remove())
	}
	c.inherited[o] = t
	c.presence.unmoved = true
	return nil
}

func (c *Compositor) holdsInherited(o *output.Output, content bool) (bool, error) {
	if _, ok := c.inherited[o]; !ok {
		return false, nil
	}
	if !content && showsInherited(o) {
		return true, nil
	}
	c.fadeFromInherited(o, c.frameTime)
	return false, c.endInherited(o)
}

func (c *Compositor) fadeFromInherited(o *output.Output, now time.Duration) {
	curve := c.animations.Curve(anim.FadeIn)
	in, ok := o.Backend().(backend.Inheriting)
	if !ok || !in.Inherited() || curve == anim.Instant {
		return
	}
	frame, err := in.InheritedFrame()
	if err != nil {
		c.report(fmt.Errorf("compositor: %w; lys-wm cuts from the frame %s inherited instead of fading from it", err, o.Backend().Name()))
		return
	}
	tex, err := c.inheritedTexture(frame.DMABUF)
	if err != nil {
		c.report(errors.Join(fmt.Errorf("compositor: read the frame %s inherited: %w; lys-wm cuts from it instead of fading", o.Backend().Name(), err), frame.Close()))
		return
	}
	f := &inheritedFade{frame: frame, tex: tex, alpha: anim.NewTween(1.0, anim.Lerp)}
	f.alpha.Set(0, now, curve)
	c.inheritFades[o] = f
}

func (c *Compositor) inheritedTexture(frame gpu.DMABUF) (render.Texture, error) {
	var imported error
	if c.importer != nil {
		tex, err := c.importer.ImportTexture(frame)
		if err == nil {
			return tex, nil
		}
		imported = err
	}
	tex, err := c.copyInherited(frame)
	if err != nil {
		return nil, errors.Join(imported, err)
	}
	return tex, nil
}

func (c *Compositor) copyInherited(frame gpu.DMABUF) (render.Texture, error) {
	linear := frame.Modifier == fourcc.Linear || frame.Modifier == fourcc.Invalid
	opaque := frame.Format == fourcc.XRGB8888 || frame.Format == fourcc.ARGB8888
	if !linear || !opaque || len(frame.Planes) != 1 {
		return nil, fmt.Errorf("compositor: a %v frame in modifier %v with %d planes is no linear 32-bit RGB frame to copy", frame.Format, frame.Modifier, len(frame.Planes))
	}
	p := frame.Planes[0]
	stride := int(p.Stride)
	length := int(p.Offset) + stride*frame.Size.Y
	mem, err := unix.Mmap(p.FD, 0, length, unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("compositor: map the %v frame to copy it: %w", frame.Size, err)
	}
	defer func() { _ = unix.Munmap(mem) }()
	if err := dmabuf.BeginCPURead(p.FD); err != nil {
		return nil, err
	}
	tex, err := c.renderer.NewTexture(render.XRGB8888, frame.Size, stride, mem[p.Offset:])
	if end := dmabuf.EndCPURead(p.FD); err == nil && end != nil {
		return nil, errors.Join(end, tex.Destroy())
	}
	if err != nil {
		return nil, fmt.Errorf("compositor: copy the %v frame: %w", frame.Size, err)
	}
	return tex, nil
}

func (c *Compositor) entersUnderInherited(o *output.Output) bool {
	if o == nil {
		return false
	}
	_, held := c.inherited[o]
	_, fading := c.inheritFades[o]
	return held || fading
}

func (c *Compositor) drawInheritedFade(pass *frame, o *output.Output, bounds image.Rectangle) {
	f := c.inheritFades[o]
	if f == nil {
		return
	}
	alpha := min(max(f.alpha.At(c.frameTime), 0), 1)
	if alpha <= 0 {
		return
	}
	size := f.tex.Size()
	pass.DrawTexture(f.tex, render.FRect{W: float64(size.X), H: float64(size.Y)}, bounds, o.Transform(), float32(alpha))
}

func (c *Compositor) inheritFading(o *output.Output, now time.Duration) bool {
	f := c.inheritFades[o]
	return f != nil && !f.alpha.Done(now)
}

func (c *Compositor) settleInheritedFade(o *output.Output, now time.Duration) error {
	if f := c.inheritFades[o]; f == nil || !f.alpha.Done(now) {
		return nil
	}
	return c.dropInheritedFade(o)
}

func (c *Compositor) dropInheritedFade(o *output.Output) error {
	f := c.inheritFades[o]
	if f == nil {
		return nil
	}
	delete(c.inheritFades, o)
	if err := errors.Join(f.tex.Destroy(), f.frame.Close()); err != nil {
		return fmt.Errorf("compositor: release the frame %s inherited: %w", o.Backend().Name(), err)
	}
	return nil
}

func (c *Compositor) inheritTimedOut(o *output.Output) error {
	if _, ok := c.inherited[o]; !ok {
		return nil
	}
	c.report(fmt.Errorf("compositor: no client drew on %s within %v, so lys-wm draws over the frame it inherited", o.Backend().Name(), c.inheritTimeout))
	c.fadeFromInherited(o, c.clock())
	return errors.Join(c.endInherited(o), c.schedule(o))
}

func (c *Compositor) endInherited(o *output.Output) error {
	t, ok := c.inherited[o]
	if !ok {
		return nil
	}
	delete(c.inherited, o)
	return t.Remove()
}
