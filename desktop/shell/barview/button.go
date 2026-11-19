// Package barview draws lys-shell's bar from a config: the bar root and
// its zones, a module's button and the separator rule, the classes and
// the per-button variables the bar stylesheet reads. It holds no
// service, bus or surface, so lys-shell's bar builds its live modules
// on it and lys-settings previews the bar from a draft config with the
// same code.
package barview

import (
	"strings"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/styling"
)

// Button is a module's button: lys-ui's BarButton with the module's
// icon and label as its own, so the bar-button CSS styles it —
//
//	button.bar-button.<variant>[.icon-only][.label-only][.vertical][.border-*][.icon-end]
//	└ box
//	  ├ image.bar-button-icon
//	  └ label.bar-button-label
//
// (the label first with button-icon-position end), with the module's
// colors injected as the per-button custom properties
// (styling.ButtonCSS) on the button. The module writes its label
// through SetLabel, which the button shows unless label-show is off,
// the text is blank or the label is frozen.
type Button struct {
	*widget.BarButton
	labelFloor int

	icon      *widget.Icon
	iconShown bool
	text      string
	frozen    bool
	title     string
	detail    string
	failure   string

	cfg        *config.Config
	button     config.ButtonConfig
	thresholds config.ThresholdColors
	ownClasses []string
}

// NewButton builds a module's button of its icon (nil for none) and its
// label text (empty for none yet), styled for cfg; a nil cfg leaves it
// unstyled.
func NewButton(cfg *config.Config, font render.Font, icon *widget.Icon, label string) *Button {
	if icon != nil {
		icon.AddClass("bar-button-icon")
	}
	b := newButtonOf(cfg, font, nil)
	b.icon, b.text = icon, label
	b.refresh()
	return b
}

// NewButtonAround wraps arbitrary module content (a canvas, a row of
// its own) in the same button, in the icon's place.
func NewButtonAround(cfg *config.Config, font render.Font, content widget.Widget) *Button {
	b := newButtonOf(cfg, font, content)
	b.refresh()
	return b
}

func newButtonOf(cfg *config.Config, font render.Font, lead widget.Widget) *Button {
	b := &Button{cfg: cfg, BarButton: widget.NewBarButton(font, lead, "")}
	b.Label().AddClass("bar-button-label")
	if cfg != nil {
		b.button = config.ButtonConfig{IconShow: true, LabelShow: true, Tooltip: true, AutoIconColor: config.TokenFgDefault}
		if cfg.Bar.ButtonIconPosition == config.IconEnd {
			b.SetIconPosition(widget.IconEnd)
		}
	}
	return b
}

// SetLabel is the module's label text: shown at once, or on thaw while
// the label is frozen.
func (b *Button) SetLabel(text string) {
	b.text = text
	b.showLabel()
	b.describe()
}

// SetDescription is the module's state in words for the accessible
// name and the tooltip, after its title: what the label alone does not
// say (charging, muted, the network's name). Empty describes the module
// by its label text.
func (b *Button) SetDescription(detail string) {
	b.detail = detail
	b.describe()
}

// Detail is the state SetDescription last gave.
func (b *Button) Detail() string { return b.detail }

// Text is the module's latest label, shown or not.
func (b *Button) Text() string { return b.text }

// Title is the module's translated name Configure gave.
func (b *Button) Title() string { return b.title }

// Icon is the module's icon, nil for none and for a content button.
func (b *Button) Icon() *widget.Icon { return b.icon }

// Frozen reports whether HoldOpen holds the label still.
func (b *Button) Frozen() bool { return b.frozen }

// Thresholds are the threshold color overrides in force.
func (b *Button) Thresholds() config.ThresholdColors { return b.thresholds }

// SetFailure marks the button failed with message, which its accessible
// name and tooltip say in place of the module's state and a blank label
// stands in for; empty clears it.
func (b *Button) SetFailure(message string) {
	b.failure = message
	if message == "" {
		b.RemoveClass(FailedClass)
	} else {
		b.AddClass(FailedClass)
	}
	b.showLabel()
	b.describe()
}

// FailedClass marks a module's button while it fails.
const FailedClass = "failed"

func (b *Button) describe() {
	detail := b.detail
	if b.failure != "" {
		detail = b.failure
	}
	name := ModuleDescription(b.title, detail, b.text)
	b.SetAccessibleName(name)
	if !b.button.Tooltip {
		name = ""
	}
	b.SetTooltip(name)
}

// ModuleDescription is title, then detail or else the label text, comma
// separated, each left out when blank.
func ModuleDescription(title, detail, label string) string {
	state := strings.TrimSpace(detail)
	if state == "" {
		state = strings.TrimSpace(label)
	}
	switch {
	case state == "":
		return title
	case title == "":
		return state
	}
	return title + ", " + state
}

func (b *Button) showLabel() {
	if b.frozen {
		return
	}
	shown := b.text
	if strings.TrimSpace(shown) == "" && b.failure != "" {
		shown = i18n.T("bar-module-failed")
	}
	if !b.button.LabelShow || strings.TrimSpace(shown) == "" {
		shown = ""
	}
	b.BarButton.SetLabel(shown)
	b.ratchetLabel(shown)
}

func (b *Button) ratchetLabel(shown string) {
	if !strings.ContainsAny(shown, "0123456789") {
		if b.labelFloor != 0 {
			b.labelFloor = 0
			b.Label().SetSizeRequest(0, 0)
		}
		return
	}
	width := b.Label().Measure(widget.Row, widget.Unknown).Nat
	if width > b.labelFloor {
		b.labelFloor = width
		b.Label().SetSizeRequest(width, 0)
	}
}

// LabelFloor is the width a numeric label keeps: the widest it has
// been, so a digit changing repaints the label and moves nothing beside
// it; zero for a label without digits.
func (b *Button) LabelFloor() int { return b.labelFloor }

func (b *Button) showIcon() {
	show := b.icon != nil && b.button.IconShow
	if show == b.iconShown {
		return
	}
	b.iconShown = show
	if show {
		b.SetIcon(b.icon)
		return
	}
	b.SetIcon(nil)
}

// HoldOpen marks the button while its dropdown is open: :checked, as a
// BarButton stays while its own popover is, and with freezeLabel
// (dropdown-freeze-label) its label held still so the popover stays
// anchored. The returned func ends both, showing what the module set
// meanwhile.
func (b *Button) HoldOpen(freezeLabel bool) func() {
	b.SetState(widget.StateChecked, true)
	b.frozen = freezeLabel
	return func() {
		b.SetState(widget.StateChecked, false)
		b.frozen = false
		b.showLabel()
	}
}

// Configure applies a module's title and button config: which of the
// icon and label show, the label's cap, the classes and the variables.
func (b *Button) Configure(title string, button config.ButtonConfig) {
	b.title, b.button = title, button
	b.Label().SetMaxWidthChars(button.LabelMaxLength)
	if button.LabelMaxLength > 0 {
		b.Label().SetEllipsize(widget.EllipsizeEnd)
	} else {
		b.Label().SetEllipsize(widget.EllipsizeNone)
	}
	b.refresh()
	b.describe()
}

// SetThresholds applies threshold color overrides over the config
// colors: the per-button variables rebuild when they changed.
func (b *Button) SetThresholds(t config.ThresholdColors) {
	if ThresholdsEqual(t, b.thresholds) {
		return
	}
	b.thresholds = t
	b.refresh()
}

// ApplyThresholds evaluates value against the module's thresholds and
// applies the matching colors.
func (b *Button) ApplyThresholds(value float64, entries []config.ThresholdEntry) {
	b.SetThresholds(config.EvaluateThresholds(value, entries))
}

func (b *Button) refresh() {
	b.showIcon()
	b.showLabel()
	if b.cfg == nil {
		return
	}
	b.RemoveClass(b.ownClasses...)
	b.ownClasses = b.classes()
	b.AddClass(b.ownClasses...)
	b.applyVariables()
}

func (b *Button) applyVariables() {
	b.SetInlineStyle(InlineDecls(styling.ButtonCSS(b.button, b.cfg.Bar, b.cfg.Styling, b.thresholds)))
}

// RestyleFor gives the button cfg's variables in place, for a reload
// that only changes high contrast or reduce transparency.
func (b *Button) RestyleFor(cfg *config.Config) {
	if b.cfg == nil {
		return
	}
	b.cfg = cfg
	b.applyVariables()
}

// Restyle restyles every bar button under w for cfg, a hidden
// revealer's content included.
func Restyle(w widget.Widget, cfg *config.Config) {
	if b, ok := w.(interface{ RestyleFor(*config.Config) }); ok {
		b.RestyleFor(cfg)
	}
	if r, ok := w.(*widget.Revealer); ok {
		Restyle(r.Child(), cfg)
		return
	}
	if c, ok := w.(interface{ Children() []widget.Widget }); ok {
		for _, k := range c.Children() {
			Restyle(k, cfg)
		}
	}
}

func (b *Button) classes() []string {
	bar := b.cfg.Bar
	out := []string{bar.ButtonVariant.CSSClass()}
	if !b.button.LabelShow {
		out = append(out, "icon-only")
	}
	if !b.button.IconShow {
		out = append(out, "label-only")
	}
	if b.button.BorderShow {
		if c, ok := bar.ButtonBorderLocation.CSSClass(); ok {
			out = append(out, c)
		}
	}
	if c, ok := bar.ButtonIconPosition.CSSClass(); ok {
		out = append(out, c)
	}
	return out
}

// InlineDecls strips a `selector { ... }` rule down to its declaration
// list, the widget's inline declarations.
func InlineDecls(rule string) string {
	open, end := strings.IndexByte(rule, '{'), strings.LastIndexByte(rule, '}')
	if open < 0 || end < open {
		return rule
	}
	return strings.TrimSpace(rule[open+1 : end])
}

// ThresholdsEqual compares two override sets by value.
func ThresholdsEqual(a, b config.ThresholdColors) bool {
	eq := func(x, y *config.ColorValue) bool {
		if x == nil || y == nil {
			return x == y
		}
		return *x == *y
	}
	return eq(a.IconColor, b.IconColor) && eq(a.LabelColor, b.LabelColor) &&
		eq(a.IconBgColor, b.IconBgColor) && eq(a.ButtonBgColor, b.ButtonBgColor) &&
		eq(a.BorderColor, b.BorderColor)
}
