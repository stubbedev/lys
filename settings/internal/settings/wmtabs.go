package settings

import (
	"fmt"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const tabPreviewH = 96

type tabState int

const (
	tabActive tabState = iota
	tabInactive
	tabUrgent
	tabStates
)

var tabStateKeys = [tabStates]string{"active", "inactive", "urgent"}

type tabColors struct{ background, text render.Color }

type tabLook struct {
	height, padding, textHeight, textPadding int
	colors                                   [tabStates]tabColors
}

func readTabLook(value func(path string) any, colour func(text string) (render.Color, bool)) (tabLook, *unreadableValue) {
	r := &lookReader{value: value, colour: colour}
	l := tabLook{
		height:      r.pixels("tabs.height"),
		padding:     r.pixels("tabs.padding"),
		textHeight:  r.pixels("tabs.text_height"),
		textPadding: r.pixels("tabs.text_padding"),
	}
	for i, key := range tabStateKeys {
		l.colors[i] = tabColors{background: r.color("tabs." + key + ".background"), text: r.color("tabs." + key + ".text")}
	}
	if r.err != nil {
		return tabLook{}, r.err
	}
	return l, nil
}

func tabStrip(screen render.Rect, l tabLook) (strip render.Rect, tabs [tabStates]render.Rect, body render.Rect) {
	inset := render.UniformInsets(max(4, screen.H/12)).Shrink(screen)
	strip = render.Rect{X: inset.X, Y: inset.Y, W: inset.W, H: min(l.height, inset.H)}
	w := strip.W / int(tabStates)
	for i := range tabs {
		tabs[i] = render.Rect{X: strip.X + i*w, Y: strip.Y, W: w, H: strip.H}
	}
	tabs[tabStates-1].W = strip.X + strip.W - tabs[tabStates-1].X
	top := strip.Y + strip.H + l.padding
	body = render.Rect{X: inset.X, Y: top, W: inset.W, H: max(0, inset.Y+inset.H-top)}
	return strip, tabs, body
}

func paintTabs(cv *render.Canvas, face render.Font, l tabLook, screen render.Rect, titles [tabStates]string) {
	th := widget.Current()
	photo(cv, screen)
	_, tabs, body := tabStrip(screen, l)
	for i, r := range tabs {
		c := l.colors[i]
		cv.FillRect(r, c.background)
		inner := render.Insets{Left: l.textPadding, Right: l.textPadding}.Shrink(r)
		if inner.W > 0 && l.textHeight > 0 {
			px := float64(l.textHeight)
			face.DrawAligned(cv, fitLabel(face, titles[i], inner.W, px), inner, px, c.text, render.AlignCenter)
		}
	}
	if body.H > 0 {
		cv.FillRect(body, th.CardColor())
		windowLines(cv, body, th.Border)
	}
}

type tabPreview struct {
	*widget.Box
	k      *kit
	area   *widget.DrawingArea
	state  *widget.Label
	look   tabLook
	bad    *unreadableValue
	titles [tabStates]string
}

func newTabPreview(k *kit) widget.Widget {
	t := locale.T()
	p := &tabPreview{Box: widget.NewBox(widget.Column, 0, 0), k: k}
	p.AddClass("settings-live-preview")
	p.SetAccessibleName(t.Get("wm-preview-tabs"))
	for i, key := range tabStateKeys {
		p.titles[i] = t.Get("wm-preview-tab-" + key)
	}
	p.area = widget.NewDrawingArea(0, tabPreviewH)
	p.area.OnDraw = func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) { paintTabs(cv, p.k.face, p.look, s, p.titles) })
	}
	p.state = wrapped(k.label("", "settings-preview-state", css.Caption, css.DimLabel))
	p.state.SetAlignment(render.AlignCenter)
	p.Append(p.area, false)
	p.Append(p.state, false)
	p.configChanged()
	return p
}

func (p *tabPreview) configChanged() {
	p.look, p.bad = readTabLook(p.k.wm.value, func(text string) (render.Color, bool) { return wmColorRGB(p.k, text) })
	if p.bad != nil {
		t := locale.T()
		p.state.SetText(t.Get("wm-preview-unreadable", i18n.Str("setting", t.Get(wmKey(p.bad.path))), i18n.Str("value", fmt.Sprint(p.bad.value))))
	}
	p.area.SetVisible(p.bad == nil)
	p.state.SetVisible(p.bad != nil)
	p.area.Invalidate()
}

func (p *tabPreview) draftChanged() { p.configChanged() }
