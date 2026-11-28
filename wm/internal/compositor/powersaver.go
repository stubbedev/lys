package compositor

import (
	"slices"

	"github.com/stubbedev/lys/wm/internal/backend"
	"github.com/stubbedev/lys/wm/internal/output"
)

const DefaultPowerSaverRate = 60

func (c *Compositor) SetPowerSaver(on bool) error {
	if on == c.powerSaver {
		return nil
	}
	c.powerSaver = on
	c.capFrameRates()
	if c.panelLit() {
		return nil
	}
	c.panelSaver = on
	return c.reapplyOutputRules()
}

func (c *Compositor) panelLit() bool {
	return slices.ContainsFunc(c.outputs, func(o *output.Output) bool {
		return internalPanel(o.Backend()) && o.Backend().State().Lit()
	})
}

func (c *Compositor) startingSaver(b backend.Output) bool {
	in, ok := b.(backend.Inheriting)
	if !ok || !in.Inherited() || c.powerSaverRate <= 0 {
		return c.powerSaver
	}
	lit := b.State().Mode
	low, err := lowRefreshMode{within: c.configuredRule(b).mode(), hz: c.powerSaverRate}.choose(b.Modes(), lit)
	return err == nil && low == lit
}

func (c *Compositor) settlePanelSaver(o *output.Output) error {
	if !internalPanel(o.Backend()) || c.panelSaver == c.powerSaver {
		return nil
	}
	c.panelSaver = c.powerSaver
	return c.applyEnabledRule(o)
}

func (c *Compositor) lowPowerRule(b backend.Output, rule OutputRule) OutputRule {
	if !c.panelSaver || !internalPanel(b) {
		return rule
	}
	if c.powerSaverRate > 0 {
		rule.Mode = lowRefreshMode{within: rule.mode(), hz: c.powerSaverRate}
	}
	if rule.HDR == HDROn {
		rule.HDR = HDRFullscreen
	}
	return rule
}

type lowRefreshMode struct {
	within ModePolicy
	hz     int
}

func (l lowRefreshMode) choose(modes []backend.Mode, current backend.Mode) (backend.Mode, error) {
	chosen, err := l.within.choose(modes, current)
	if err != nil {
		return backend.Mode{}, err
	}
	floor := int64(l.hz)*1000 - refreshToleranceMHz
	for _, m := range modes {
		if m.Size == chosen.Size && int64(m.RefreshMHz) >= floor && m.RefreshMHz < chosen.RefreshMHz {
			chosen = m
		}
	}
	return chosen, nil
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
