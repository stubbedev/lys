package compositor

import "github.com/stubbedev/lys/wm/internal/backend"

const DefaultPowerSaverRate = 60

func (c *Compositor) SetPowerSaver(on bool) {
	c.powerSaver = on
	c.capFrameRates()
}

func (c *Compositor) capFrameRates() {
	for _, o := range c.outputs {
		hz := 0
		if sc, ok := c.scenes[o]; ok {
			hz = sc.contentCap
		}
		c.capFrameRate(o.Backend(), hz)
	}
	for _, b := range c.unplaced() {
		c.capFrameRate(b, 0)
	}
}

func (c *Compositor) capFrameRate(b backend.Output, contentHz int) {
	fc, ok := b.(backend.FrameRateCap)
	if !ok {
		return
	}
	hz := contentHz
	if c.powerSaver && c.powerSaverRate > 0 && (hz == 0 || c.powerSaverRate < hz) {
		hz = c.powerSaverRate
	}
	fc.CapFrameRate(hz)
}
