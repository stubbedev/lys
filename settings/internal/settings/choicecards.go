package settings

import (
	"log"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

const (
	choiceCardW   = 104
	choiceCardH   = 92
	choicePicH    = 56
	choiceCardGap = 8
	choiceRing    = 2
)

type picture struct {
	draw func(cv *render.Canvas, r render.Rect)
	icon string
}

func drawn(f func(cv *render.Canvas, r render.Rect)) picture { return picture{draw: f} }

type choiceCards struct {
	*widget.FlowBox
	slot    slot
	cards   []*choiceCard
	current string
}

type choiceCard struct {
	widget.Base
	k     *kit
	owner *choiceCards
	value string
	label string
	pic   picture
	icon  *widget.Icon
}

func newChoiceCards(k *kit, s slot, meta config.FieldMeta, pictures map[string]picture) *choiceCards {
	return newCards(k, s, meta.Variants, enumLabels(meta), pictures)
}

func newCards(k *kit, s slot, values, labels []string, pictures map[string]picture) *choiceCards {
	c := &choiceCards{FlowBox: widget.NewWrapBox(choiceCardGap, choiceCardGap, widget.JustifyStart), slot: s}
	c.AddClass("choice-cards")
	for i, v := range values {
		card := &choiceCard{Base: widget.NewBase(), k: k, owner: c, value: v, label: labels[i], pic: pictures[v]}
		card.SetAccessibleName(labels[i])
		if name := pictures[v].icon; name != "" {
			card.icon = widget.NewThemeIcon(name, choicePicH/2)
			widget.SetParents(card, card.icon)
		}
		c.cards = append(c.cards, card)
		c.Append(card)
	}
	c.refresh()
	return c
}

func (c *choiceCards) refresh() {
	v, _ := c.slot.get().(string)
	c.current = v
	for _, card := range c.cards {
		card.Invalidate()
	}
}

func (c *choiceCards) choose(value string) {
	if err := c.slot.set(value); err != nil {
		return
	}
	c.refresh()
}

func (c *choiceCard) Measure(axis widget.Axis, _ int) widget.Sizes {
	if axis == widget.Row {
		return widget.FixedSizes(choiceCardW)
	}
	return widget.FixedSizes(choiceCardH)
}

func (c *choiceCard) picRect() render.Rect {
	b := c.Bounds()
	return render.Rect{X: b.X + choiceCardGap, Y: b.Y + choiceCardGap, W: b.W - 2*choiceCardGap, H: choicePicH - choiceCardGap}
}

func (c *choiceCard) Children() []widget.Widget {
	if c.icon == nil {
		return nil
	}
	return []widget.Widget{c.icon}
}

func (c *choiceCard) Arrange(r render.Rect) {
	c.ArrangeSelf(r)
	if c.icon != nil {
		pic, size := c.picRect(), choicePicH/2
		c.icon.Arrange(render.Rect{X: pic.X + (pic.W-size)/2, Y: pic.Y + (pic.H-size)/2, W: size, H: size})
	}
}

func (c *choiceCard) Paint(cv *render.Canvas) {
	th := widget.Current()
	b := c.Bounds()
	corners := render.UniformCorners(tileRadius)
	cv.RoundedRect(b, tileRadius, th.CardColor())
	if c.pic.draw != nil {
		c.pic.draw(cv, c.picRect())
	}
	if c.icon != nil {
		c.icon.Paint(cv)
	}
	selected := c.value == c.owner.current
	switch {
	case selected:
		cv.RoundedBorder(b, corners, render.UniformInsets(choiceRing), th.Accent)
	case widget.FocusVisible(c):
		cv.RoundedBorder(b, corners, render.UniformInsets(focusRing), th.Text)
	default:
		cv.RoundedBorder(b, corners, render.UniformInsets(1), th.Border)
	}
	text := render.Rect{X: b.X, Y: b.Y + choicePicH, W: b.W, H: b.H - choicePicH}
	c.k.face.DrawAligned(cv, c.label, text, c.k.textPx(), th.Text, render.AlignCenter)
}

func (c *choiceCard) HitTest(p widget.Point) widget.Widget {
	if c.Bounds().Contains(p.X, p.Y) {
		return c
	}
	return nil
}

func (c *choiceCard) PressAt(widget.Point) { c.owner.choose(c.value) }

func (c *choiceCard) Role() widget.Role { return widget.RoleButton }

func (c *choiceCard) KeyAction(a widget.KeyAction, _ widget.Mods) {
	if a == widget.KeyEnter || a == widget.KeySpace {
		c.owner.choose(c.value)
	}
}

func (c *choiceCard) FocusMoved(_, _ widget.Widget) { c.Invalidate() }

func pictureChoice(pictures map[string]picture) rowOpt {
	return func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, meta config.FieldMeta) control {
			for _, v := range meta.Variants {
				if _, ok := pictures[v]; !ok {
					log.Panicf("settings: the picture choice for %v has no picture for %s", meta.Variants, v)
				}
			}
			return newChoiceCards(k, s, meta, pictures)
		}
	}
}

var (
	previewLight      = render.RGB(0xf4, 0xf4, 0xf6)
	previewLightInk   = render.RGB(0xc8, 0xc8, 0xd2)
	previewDark       = render.RGB(0x22, 0x22, 0x2a)
	previewDarkInk    = render.RGB(0x4a, 0x4a, 0x56)
	previewImageTop   = render.RGB(0x6c, 0x8e, 0xd8)
	previewImageFloor = render.RGB(0x5a, 0xa0, 0x6e)
)

func miniWindow(cv *render.Canvas, r render.Rect, bg, ink render.Color) {
	cv.RoundedRect(r, tileRadius/2, bg)
	pad := max(3, r.H/8)
	bar := render.Rect{X: r.X + pad, Y: r.Y + pad, W: r.W - 2*pad, H: max(3, r.H/7)}
	cv.RoundedRect(bar, bar.H/2, ink)
	for i := range 2 {
		line := render.Rect{X: r.X + pad, Y: bar.Y + bar.H + pad*(i+1) + bar.H*i, W: (r.W - 2*pad) * (3 - i) / 4, H: max(2, bar.H*2/3)}
		cv.RoundedRect(line, line.H/2, ink)
	}
}

var appearancePictures = map[string]picture{
	string(config.AppearanceLight): drawn(func(cv *render.Canvas, r render.Rect) { miniWindow(cv, r, previewLight, previewLightInk) }),
	string(config.AppearanceDark):  drawn(func(cv *render.Canvas, r render.Rect) { miniWindow(cv, r, previewDark, previewDarkInk) }),
	string(config.AppearanceAuto): drawn(func(cv *render.Canvas, r render.Rect) {
		left := render.Rect{X: r.X, Y: r.Y, W: r.W / 2, H: r.H}
		right := render.Rect{X: r.X + r.W/2, Y: r.Y, W: r.W - r.W/2, H: r.H}
		prev := cv.PushClip(left)
		miniWindow(cv, r, previewLight, previewLightInk)
		cv.PopClip(prev)
		prev = cv.PushClip(right)
		miniWindow(cv, r, previewDark, previewDarkInk)
		cv.PopClip(prev)
	}),
}

func photo(cv *render.Canvas, r render.Rect) {
	sky := render.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H * 2 / 3}
	cv.FillRect(sky, previewImageTop)
	cv.FillRect(render.Rect{X: r.X, Y: sky.Y + sky.H, W: r.W, H: r.H - sky.H}, previewImageFloor)
}

func screenFrame(cv *render.Canvas, r render.Rect, inner func(screen render.Rect)) {
	th := widget.Current()
	cv.RoundedRect(r, tileRadius/2, th.MutedColor())
	prev := cv.PushClip(r)
	inner(r)
	cv.PopClip(prev)
	cv.RoundedBorder(r, render.UniformCorners(tileRadius/2), render.UniformInsets(1), th.Border)
}

var fitPictures = map[string]picture{
	string(config.FitFill): drawn(func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) { photo(cv, render.Rect{X: s.X - s.W/6, Y: s.Y, W: s.W + s.W/3, H: s.H}) })
	}),
	string(config.FitFit): drawn(func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) {
			w := s.H * 4 / 3
			photo(cv, render.Rect{X: s.X + (s.W-w)/2, Y: s.Y, W: w, H: s.H})
		})
	}),
	string(config.FitCenter): drawn(func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) {
			photo(cv, render.Rect{X: s.X + s.W/4, Y: s.Y + s.H/4, W: s.W / 2, H: s.H / 2})
		})
	}),
	string(config.FitStretch): drawn(func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) { photo(cv, s) })
	}),
}

var themeProviderPictures = map[string]picture{
	string(config.ThemeProviderLysShell):  {icon: "ld-palette-symbolic"},
	string(config.ThemeProviderMatugen):   {icon: "ld-droplets-symbolic"},
	string(config.ThemeProviderPywal):     {icon: "ld-droplet-symbolic"},
	string(config.ThemeProviderWallust):   {icon: "ld-sun-moon-symbolic"},
	string(config.ThemeProviderWallpaper): {icon: "ld-image-symbolic"},
}
