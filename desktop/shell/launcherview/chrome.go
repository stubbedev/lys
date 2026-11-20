package launcherview

import (
	"math"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/pango"
)

// Chrome is every session's field and footer: the search icon, the chip
// (the level's title or the prompt), the entry, the message and the
// separator above the list, the notice and key hints below it.
type Chrome struct {
	Input      *widget.Box
	SearchIcon *AsyncIcon
	Chip       *widget.Label
	Entry      *widget.Entry
	Message    *widget.RichLabel
	Sep        *widget.Box
	Footer     *widget.Box
	Notice     *widget.Label
	Hints      *widget.RichLabel
	markup     []byte
}

// NewChrome builds the field and footer at cfg's scale, its labels in
// font with its bold and italic variants; changed hears every edit of
// the entry.
func NewChrome(font render.Font, variants widget.VariantFunc, cfg *config.Config, changed func(string)) *Chrome {
	px := ScaledPx(cfg)
	body := FontPx()
	c := &Chrome{}
	c.Input = widget.NewBox(widget.Row, px(10), 0)
	c.Input.AddClass("launcher-search-input")
	c.Input.SetSizeRequest(0, px(InputRowHeight))
	c.Input.SetPadding(render.Insets{Left: px(18), Right: px(18)})
	c.SearchIcon = NewAsyncIcon(px(searchIconPx), "launcher-search-icon")
	c.Input.AppendAligned(c.SearchIcon.Slot, false, widget.AlignCenter)
	c.Chip = widget.NewLabel(font, body, "", 0)
	c.Chip.AddClass("launcher-search-chip")
	c.Chip.SetVisible(false)
	c.Input.AppendAligned(c.Chip, false, widget.AlignCenter)
	c.Entry = widget.NewEntry(font, math.Round(body*entryFontFactor), 0)
	c.Entry.AddClass("launcher-search-entry")
	c.Entry.OnChanged = changed
	c.Input.AppendAligned(c.Entry, true, widget.AlignCenter)

	c.Message = widget.NewRichLabel(font, body, "", 0)
	c.Message.AddClass("launcher-search-message", css.DimLabel)
	c.Message.SetMargin(render.Insets{Left: px(20), Right: px(20), Bottom: px(8)})
	c.Message.SetVariants(variants)
	c.Message.SetWrap(true)
	c.Message.SetVisible(false)

	c.Sep = widget.NewBox(widget.Column, 0, 0)
	c.Sep.AddClass("launcher-search-separator")
	c.Sep.SetSizeRequest(0, 1)
	c.Sep.SetVisible(false)

	c.Footer = widget.NewBox(widget.Row, px(12), 0)
	c.Footer.AddClass("launcher-search-footer")
	c.Footer.SetSizeRequest(0, px(FooterHeight))
	c.Footer.SetPadding(render.Insets{Left: px(16), Right: px(16)})
	c.Notice = widget.NewLabel(font, SmallPx(), "", 0)
	c.Notice.AddClass("launcher-search-notice")
	c.Notice.SetEllipsize(widget.EllipsizeEnd)
	c.Footer.AppendAligned(c.Notice, true, widget.AlignCenter)
	c.Hints = widget.NewRichLabel(font, SmallPx(), "", 0)
	c.Hints.AddClass("launcher-search-hints", css.DimLabel)
	c.Hints.SetVariants(variants)
	c.Footer.AppendAligned(c.Hints, false, widget.AlignCenter)
	c.Footer.SetVisible(false)
	return c
}

// FooterHint is one key and what it does.
type FooterHint struct{ Key, Does string }

// CloseHint is the footer's way out of a level.
func CloseHint() FooterHint { return FooterHint{"Esc", i18n.T("launcher-footer-close")} }

// ShowHints writes the footer's hints.
func (c *Chrome) ShowHints(hints ...FooterHint) {
	b := c.markup[:0]
	for i, h := range hints {
		if i > 0 {
			b = append(b, "     "...)
		}
		b = append(b, "<b>"...)
		b = AppendEscaped(b, h.Key)
		b = append(b, "</b> "...)
		b = AppendEscaped(b, h.Does)
	}
	c.markup = b
	if string(b) != c.Hints.Markup() {
		c.Hints.SetMarkup(string(b))
	}
}

// SetMessage shows the message, Pango markup, hidden without one.
func (c *Chrome) SetMessage(markup string) {
	if markup == "" {
		c.Message.SetVisible(false)
		return
	}
	c.Message.SetMarkup(messageMarkup(markup))
	c.Message.SetVisible(true)
}

// SetChip shows the title or prompt, hidden when empty.
func (c *Chrome) SetChip(text string) {
	c.Chip.SetText(text)
	c.Chip.SetVisible(text != "")
}

// SetMasked shows the entry as dots while it holds a secret.
func (c *Chrome) SetMasked(on bool) {
	echo := widget.EchoNormal
	if on {
		echo = widget.EchoPassword
	}
	c.Entry.SetEcho(echo)
}

// messageMarkup is the message line's markup in lys-ui's dialect: the
// Pango markup translated, its weights and colors kept; markup that does
// not parse shows as the literal text, like a row's.
func messageMarkup(markup string) string {
	if translated, ok := pango.Translate(markup); ok {
		return translated
	}
	return pango.Escape(markup)
}

// AppendEscaped appends text escaped for lys-ui markup, as pango.Escape
// does, with no intermediate string.
func AppendEscaped(dst []byte, text string) []byte {
	for i := range len(text) {
		switch c := text[i]; c {
		case '&':
			dst = append(dst, "&amp;"...)
		case '<':
			dst = append(dst, "&lt;"...)
		case '>':
			dst = append(dst, "&gt;"...)
		case '\'':
			dst = append(dst, "&apos;"...)
		case '"':
			dst = append(dst, "&quot;"...)
		default:
			dst = append(dst, c)
		}
	}
	return dst
}
