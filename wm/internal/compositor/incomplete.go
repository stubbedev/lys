package compositor

import (
	"fmt"

	"github.com/stubbedev/lys-wm/internal/render"
)

func (c *Compositor) buildOffThread() error {
	b, ok := c.renderer.(render.OffThreadBuilder)
	if !ok {
		return nil
	}
	if err := b.BuildOffThread(); err != nil {
		return fmt.Errorf("build the renderer's pipelines off the compositor thread: %w", err)
	}
	return nil
}

func (c *Compositor) PipelinesBuilt() (bool, error) {
	b, ok := c.renderer.(render.OffThreadBuilder)
	if !ok {
		return true, nil
	}
	return b.PipelinesBuilt()
}

func (c *Compositor) redrawIncomplete() error {
	n := c.renderer.Counts().Incomplete
	if n == c.incompletePasses {
		return nil
	}
	c.incompletePasses = n
	for _, sc := range c.scenes {
		sc.invalidate()
		sc.blurs.invalidate()
	}
	for _, w := range c.windows {
		w.jellyVersion = 0
	}
	c.DirtyOverviewThumbs()
	if sw := c.switcher; sw != nil {
		for _, t := range sw.thumbs {
			t.dirty = true
		}
	}
	return c.scheduleAll()
}
