package bar

import (
	"context"
	"math"
	"strconv"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/shell/service/nightlight"
)

const nightlightTemperatureStep = 100

type nightlightCard struct {
	ctx      ModuleContext
	svc      *nightlightService
	card     *widget.Box
	item     *widget.Item
	toggle   *widget.Switch
	slider   *widgets.DebouncedSlider
	failure  *widget.Alert
	syncing  bool
	writeErr error
}

func newNightlightCard(ctx ModuleContext, font render.Font, px float64, svc *nightlightService) *nightlightCard {
	c := &nightlightCard{ctx: ctx, svc: svc}
	card, group := itemCard()
	card.AddClass("brightness-nightlight")
	c.card = card
	row := widget.NewBox(widget.Column, 0, 0)
	c.item = widget.NewItem(font, i18n.T("dropdown-brightness-nightlight-title"), "")
	c.item.SetSmall(true)
	c.item.SetMedia(widget.NewThemeIcon(ctx.Config.Nightlight.IconOn, itemIconPx), false)
	c.toggle = namedSwitch(i18n.T("dropdown-brightness-nightlight-title"), false)
	c.toggle.AddClass("brightness-nightlight-toggle")
	c.toggle.OnChanged = c.switched
	c.item.AddAction(c.toggle)
	row.Append(c.item, false)

	c.slider = widgets.NewRangedSlider(nightlight.MinTemp, nightlight.NeutralTemp, nightlight.NeutralTemp, font, px, 0, ctx.Invoke, ctx.Clock)
	c.slider.SetFormat(func(v float64) string { return kelvinText(nightlightTemperature(v)) })
	c.slider.Knob.AddClass("brightness-nightlight-slider")
	c.slider.ValueLabel().AddClass("brightness-slider-value", css.Numeric)
	c.slider.OnCommit = c.committed
	c.failure = writeAlert(font, "brightness-nightlight-error")
	body := itemBody()
	body.Append(c.slider, false)
	body.Append(c.failure, false)
	row.Append(body, false)
	group.Append(row)
	c.apply()
	return c
}

func nightlightTemperature(v float64) uint32 {
	return uint32(math.Round(v/nightlightTemperatureStep)) * nightlightTemperatureStep
}

func kelvinText(kelvin uint32) string {
	return i18n.T("dropdown-brightness-nightlight-kelvin", i18n.Str("kelvin", strconv.FormatUint(uint64(kelvin), 10)))
}

func (c *nightlightCard) follow(life context.Context) {
	c.apply()
	stop := c.svc.watch(func() {
		if life.Err() == nil {
			c.apply()
		}
	})
	context.AfterFunc(life, func() { c.ctx.Invoke(stop) })
}

func (c *nightlightCard) apply() {
	s := c.svc
	c.syncing = true
	c.toggle.SetOn(s.showing())
	c.syncing = false
	if s.saveErr != nil {
		c.slider.Reset(float64(s.cfg.Temperature))
	} else {
		c.slider.Set(float64(s.cfg.Temperature))
	}
	available := s.unavailable == nil
	c.toggle.SetEnabled(available)
	c.slider.SetEnabled(available)
	c.item.SetDescription(c.status())
	title, problem := c.problem()
	if !available {
		problem = nil
	}
	showFailure(c.failure, title, problem)
}

// problem is what keeps the night light from what was asked, with the
// summary its alert shows: a temperature that cannot be set, one not
// saved, or a display that refused it; nil without one.
func (c *nightlightCard) problem() (string, error) {
	s := c.svc
	switch {
	case c.writeErr != nil:
		return i18n.T("dropdown-brightness-nightlight-invalid"), c.writeErr
	case s.saveErr != nil:
		return i18n.T("dropdown-brightness-nightlight-not-saved"), s.saveErr
	case s.refused != nil:
		return s.refusal(), s.refused
	}
	return "", nil
}

func (c *nightlightCard) status() string {
	s := c.svc
	switch {
	case s.unavailable != nil:
		return i18n.T("dropdown-brightness-nightlight-unavailable")
	case s.noLocation() != nil:
		return i18n.T("bar-nightlight-no-location")
	case s.showing():
		return i18n.T("dropdown-brightness-nightlight-on", i18n.Str("kelvin", strconv.Itoa(s.live.Temp())))
	}
	return i18n.T("bar-nightlight-off")
}

func (c *nightlightCard) switched(on bool) {
	if c.syncing {
		return
	}
	c.writeErr = nil
	c.svc.setEnabled(on)
	c.apply()
}

func (c *nightlightCard) committed(v float64) {
	c.writeErr = c.svc.setTemperature(nightlightTemperature(v))
	if c.writeErr != nil {
		c.slider.Reset(float64(c.svc.cfg.Temperature))
	}
	c.apply()
}
