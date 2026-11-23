package bar

import (
	"errors"
	"strconv"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/jinja"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/service/nightlight"
)

func nightlightLabel(format string, live *nightlight.Setting, temp, gamma int) string {
	return jinja.RenderOr(format, nightlightVars(live, temp, gamma))
}

func nightlightVars(live *nightlight.Setting, temp, gamma int) map[string]any {
	status, tempText, gammaText := i18n.T("bar-nightlight-off"), "--", "--"
	if live != nil {
		status, tempText, gammaText = i18n.T("bar-nightlight-on"), strconv.Itoa(live.Temp()), strconv.Itoa(live.Gamma())
	}
	return map[string]any{
		"status": status, "temp": tempText, "gamma": gammaText,
		"config_temp": strconv.Itoa(temp), "config_gamma": strconv.Itoa(gamma),
	}
}

type nightlightModule struct {
	ctx    ModuleContext
	svc    *nightlightService
	btn    *barButton
	icon   *widget.Icon
	chrome moduleChrome
	firstRead
}

func newNightlight(ctx ModuleContext) (Module, error) {
	if ctx.Nightlight == nil {
		return nil, errors.New("nightlight: the shell runs no night light")
	}
	if err := ctx.Nightlight.unavailable; err != nil {
		return nil, err
	}
	return newNightlightModule(ctx, ctx.Nightlight), nil
}

func newNightlightModule(ctx ModuleContext, svc *nightlightService) *nightlightModule {
	m := &nightlightModule{ctx: ctx, svc: svc}
	m.icon = barview.Icon(ctx.Config.Nightlight.IconOffView())
	m.btn = newBarButton(ctx, m.icon, "")
	ctx.OnRetire(svc.watch(m.render))
	m.render()
	return m
}

func (m *nightlightModule) setChrome(c moduleChrome) {
	m.chrome = c
	m.hideUntilRead(c)
	m.render()
}

func (m *nightlightModule) render() {
	cfg := m.ctx.Config.Nightlight
	if m.svc.connected {
		m.reveal()
	}
	var live *nightlight.Setting
	icon := cfg.IconOffView()
	if m.svc.showing() {
		live = &m.svc.live
		icon = cfg.IconOnView()
	}
	m.btn.SetLabel(nightlightLabel(cfg.Format, live, int(m.svc.cfg.Temperature), int(m.svc.cfg.Gamma)))
	if m.icon != nil {
		m.icon.SetThemeName(icon.Name)
	}
	refusal, noLocation := "", ""
	switch {
	case m.svc.unavailable != nil:
		refusal = i18n.T("bar-nightlight-unavailable")
	case m.svc.refused != nil:
		refusal = m.svc.refusal()
	case m.svc.noLocation() != nil:
		noLocation = i18n.T("bar-nightlight-no-location")
	}
	setClass(m.chrome, "nightlight-refused", refusal != "")
	setClass(m.chrome, "nightlight-no-location", noLocation != "")
	m.btn.SetDescription(refusal + noLocation)
}

func (m *nightlightModule) RunAction(action config.ClickAction) {
	if action.Kind == config.ClickShell && action.Command == ":toggle" {
		m.svc.toggle()
		return
	}
	runClickAction(m.ctx, action)
}

func (m *nightlightModule) Root() widget.Widget { return m.btn }
