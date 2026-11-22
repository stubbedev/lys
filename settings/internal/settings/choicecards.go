package settings

import (
	"log"
	"strings"
	"unicode"

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
	cardW   int
}

type choiceCard struct {
	tile
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
	c := &choiceCards{FlowBox: widget.NewWrapBox(choiceCardGap, choiceCardGap, widget.JustifyStart), slot: s, cardW: choiceCardW}
	c.AddClass("choice-cards")
	for i, v := range values {
		card := &choiceCard{k: k, owner: c, value: v, label: labels[i], pic: pictures[v]}
		card.tile = newTile(card, func() { c.choose(v) })
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

// Measure balances the lines for the width given before the cards wrap
// in it (balance).
func (c *choiceCards) Measure(axis widget.Axis, forSize int) widget.Sizes {
	if axis == widget.Column && forSize != widget.Unknown {
		c.balance(forSize)
	}
	return c.FlowBox.Measure(axis, forSize)
}

// Arrange balances the lines for the width given, then wraps the cards.
func (c *choiceCards) Arrange(r render.Rect) {
	c.balance(r.W)
	c.FlowBox.Arrange(r)
}

// balance caps the cards a line of width holds so that no line holds a
// single card while another holds several (balancedColumns).
func (c *choiceCards) balance(width int) {
	c.SetMaxChildrenPerLine(balancedColumns(len(c.cards), (width+choiceCardGap)/(c.cardW+choiceCardGap)))
}

// balancedColumns is how many of n cards a line holds when fit fit on
// one: all of them where they fit, else the most that divide n, else the
// most that leave a last line of two or more, else one a line.
func balancedColumns(n, fit int) int {
	if fit >= n {
		return max(n, 1)
	}
	for cols := fit; cols >= 2; cols-- {
		if n%cols == 0 {
			return cols
		}
	}
	for cols := fit; cols >= 2; cols-- {
		if n%cols != 1 {
			return cols
		}
	}
	return 1
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
		return widget.FixedSizes(c.owner.cardW)
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
	text := render.Rect{X: b.X + choiceCardGap, Y: b.Y + choicePicH, W: b.W - 2*choiceCardGap, H: b.H - choicePicH}
	c.k.face.DrawAligned(cv, fitLabel(c.k.face, c.label, text.W, c.k.textPx()), text, c.k.textPx(), th.Text, render.AlignCenter)
}

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
	windowLines(cv, r, ink)
}

func windowLines(cv *render.Canvas, r render.Rect, ink render.Color) {
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

func titlebarMock(cv *render.Canvas, r render.Rect, buttonsLeft bool) {
	th := widget.Current()
	cv.RoundedRect(r, tileRadius/2, th.MutedColor())
	bar := render.Rect{X: r.X, Y: r.Y, W: r.W, H: max(8, r.H/4)}
	cv.RoundedRect(bar, tileRadius/2, th.Border)
	titlebarDots(cv, bar, buttonsLeft, []render.Color{th.Text, th.Text, th.Text})
}

func titlebarDots(cv *render.Canvas, bar render.Rect, left bool, dots []render.Color) {
	dot := bar.H / 2
	gap := dot / 2
	x := bar.X + bar.W - gap - len(dots)*dot - (len(dots)-1)*gap
	if left {
		x = bar.X + gap
	}
	for _, c := range dots {
		cv.RoundedRect(render.Rect{X: x, Y: bar.Y + (bar.H-dot)/2, W: dot, H: dot}, dot/2, c)
		x += dot + gap
	}
}

var titlebarSidePictures = map[string]picture{
	"left":  drawn(func(cv *render.Canvas, r render.Rect) { titlebarMock(cv, r, true) }),
	"right": drawn(func(cv *render.Canvas, r render.Rect) { titlebarMock(cv, r, false) }),
}

func popupMock(cv *render.Canvas, r render.Rect, behind int) {
	th := widget.Current()
	step := max(2, r.H/8)
	h := r.H - behind*step
	for i := behind; i > 0; i-- {
		cv.RoundedRect(render.Rect{X: r.X + i*step, Y: r.Y + i*step, W: r.W - 2*i*step, H: h}, tileRadius/2, th.Border)
	}
	front := render.Rect{X: r.X, Y: r.Y, W: r.W, H: h}
	miniWindow(cv, front, th.CardColor(), th.Border)
}

func popupColumn(cv *render.Canvas, r render.Rect, stacks, behind int) {
	gap := max(2, r.H/12)
	h := (r.H - (stacks-1)*gap) / stacks
	for i := range stacks {
		popupMock(cv, render.Rect{X: r.X, Y: r.Y + i*(h+gap), W: r.W, H: h}, behind)
	}
}

var popupStackPictures = map[string]picture{
	string(config.PopupStackPerApp): drawn(func(cv *render.Canvas, r render.Rect) { popupColumn(cv, r, 2, 2) }),
	string(config.PopupStackAll):    drawn(func(cv *render.Canvas, r render.Rect) { popupColumn(cv, r, 1, 2) }),
	string(config.PopupStackOff):    drawn(func(cv *render.Canvas, r render.Rect) { popupColumn(cv, r, 3, 0) }),
}

func alignedBars(cv *render.Canvas, r render.Rect, align config.OsdTextAlign) {
	th := widget.Current()
	cv.RoundedRect(r, tileRadius/2, th.CardColor())
	pad := max(3, r.H/6)
	h := max(3, r.H/8)
	for i, frac := range []int{3, 2, 3} {
		w := (r.W - 2*pad) * frac / 4
		x := r.X + pad
		switch align {
		case config.OsdTextAlignCenter:
			x = r.X + (r.W-w)/2
		case config.OsdTextAlignEnd:
			x = r.X + r.W - pad - w
		}
		cv.RoundedRect(render.Rect{X: x, Y: r.Y + pad + i*(h+pad), W: w, H: h}, h/2, th.Text)
	}
}

func windowGrid(cv *render.Canvas, r render.Rect) {
	th := widget.Current()
	gap := max(2, r.H/10)
	w, h := (r.W-gap)/2, (r.H-gap)/2
	for i := range 4 {
		miniWindow(cv, render.Rect{X: r.X + (i%2)*(w+gap), Y: r.Y + (i/2)*(h+gap), W: w, H: h}, th.CardColor(), th.Border)
	}
}

func twoScreens(cv *render.Canvas, r render.Rect) {
	gap := max(2, r.W/12)
	w := (r.W - gap) / 2
	h := w * 2 / 3
	y := r.Y + (r.H-h)/2
	for i := range 2 {
		screenFrame(cv, render.Rect{X: r.X + i*(w+gap), Y: y, W: w, H: h}, func(s render.Rect) { photo(cv, s) })
	}
}

func screenRegion(cv *render.Canvas, r render.Rect) {
	screenFrame(cv, r, func(s render.Rect) {
		photo(cv, s)
		region := render.Rect{X: s.X + s.W/4, Y: s.Y + s.H/4, W: s.W / 2, H: s.H / 2}
		cv.RoundedBorder(region, render.UniformCorners(tileRadius/4), render.UniformInsets(max(1, r.H/40)), widget.Current().Accent)
	})
}

var sharePickerPagePictures = map[string]picture{
	string(config.SharePickerWindows): drawn(windowGrid),
	string(config.SharePickerOutputs): drawn(twoScreens),
	string(config.SharePickerRegion):  drawn(screenRegion),
}

var textAlignPictures = map[string]picture{
	string(config.OsdTextAlignStart):  drawn(func(cv *render.Canvas, r render.Rect) { alignedBars(cv, r, config.OsdTextAlignStart) }),
	string(config.OsdTextAlignCenter): drawn(func(cv *render.Canvas, r render.Rect) { alignedBars(cv, r, config.OsdTextAlignCenter) }),
	string(config.OsdTextAlignEnd):    drawn(func(cv *render.Canvas, r render.Rect) { alignedBars(cv, r, config.OsdTextAlignEnd) }),
}

// fitLabel is label cut to width w at px with an ellipsis, whole when
// it fits.
func fitLabel(face render.Font, label string, w int, px float64) string {
	fits := func(s string) bool { return face.Shape(s, px).Advance() <= float64(w) }
	if fits(label) {
		return label
	}
	runes := []rune(label)
	for n := len(runes) - 1; n > 0; n-- {
		if cut := strings.TrimRightFunc(string(runes[:n]), unicode.IsSpace) + "…"; fits(cut) {
			return cut
		}
	}
	return "…"
}
