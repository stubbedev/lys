package compositor

import (
	"time"

	"github.com/stubbedev/lys/wm/internal/backend"
	"github.com/stubbedev/lys/wm/internal/output"
	"github.com/stubbedev/lys/wm/internal/surface"
	"github.com/stubbedev/lys/wm/internal/wm"
)

var videoRates = [...]int{24, 25, 30, 48, 50, 60}

func videoRate(interval time.Duration) int {
	for _, hz := range videoRates {
		frame := time.Second / time.Duration(hz)
		if (interval-frame).Abs()*200 <= frame {
			return hz
		}
	}
	return 0
}

func (c *Compositor) contentCap(b backend.Output, s *surface.Surface, alone bool) int {
	if !alone || c.contentType(s) != wm.ContentVideo {
		return 0
	}
	hz := videoRate(c.timing.Interval(s))
	as, ok := b.(backend.AdaptiveSync)
	return b.State().Mode.ContentCap(hz, ok && as.AdaptiveSync())
}

func (c *Compositor) showContentRate(o *output.Output, sc *scene, s *surface.Surface, alone bool) {
	b := o.Backend()
	hz := c.contentCap(b, s, alone)
	if hz == sc.contentCap {
		return
	}
	sc.contentCap = hz
	c.capFrameRate(b, hz)
}
