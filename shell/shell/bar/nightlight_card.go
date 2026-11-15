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
	failure  *widget.Label
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

	c.slider = widgets.NewRangedSlider(nightlight.MinTemp, nightlight.NeutralTemp, nightlight.NeutralTemp, font, px, 0, ctx.Invoke)
	c.slider.SetFormat(func(v float64) string { return kelvinText(nightlightTemperature(v)) })
	c.slider.Knob.AddClass("brightness-nightlight-slider")
	c.slider.ValueLabel().AddClass("brightness-slider-value", css.Numeric)
	c.slider.OnCommit = c.committed
	c.failure = widget.NewLabel(font, sectionLabelPx(), "", 0)
	c.failure.AddClass("brightness-nightlight-error", css.Caption, css.Error)
	c.failure.SetWrap(true)
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
	problem := s.problem()
	if s.saveErr != nil {
		problem = s.saveErr
	}
	if c.writeErr != nil {
		problem = c.writeErr
	}
	c.failure.SetVisible(problem != nil && available)
	if problem != nil {
		c.failure.SetText(problem.Error())
	}
}

func (c *nightlightCard) status() string {
	s := c.svc
	switch {
	case s.unavailable != nil:
		return i18n.T("dropdown-brightness-nightlight-unavailable", i18n.Str("reason", s.unavailable.Error()))
	case s.noLocation() != nil:
		return i18n.T("bar-nightlight-no-location", i18n.Str("reason", s.noLocation().Error()))
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
