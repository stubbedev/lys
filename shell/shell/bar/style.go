package bar

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
)

// barStyle is what the Go-painted surfaces (the dropdown panels, the
// workspace buttons, the popups) read to match the bar: the resolved
// palette, the text ink and size, and the dropdown button shades. The
// bar itself is styled by the app stylesheet (apptheme.Theme); nothing
// here re-derives its cascade.
type barStyle struct {
	palette *styling.Palette

	// fg is the default ink. Bar modules get a copy with fg zero (unset,
	// so the stylesheet's --bar-btn-label-color applies).
	fg render.Color
	// moduleGap spaces the items of a bar zone, groupGap the modules
	// of a group.
	moduleGap, groupGap int
	labelPx             float64

	buttonBgHover  render.Color
	buttonBgActive render.Color
}

// forModules is the modules' copy: their ink is left to the
// stylesheet (--bar-btn-label-color, the label-color option).
func (s barStyle) forModules() barStyle {
	s.fg = 0
	return s
}

// computeStyle resolves the Go-painted surfaces' parameters: sizes in
// pixels (rem base times bar scale, pixels literal) and colors through
// the token table.
func computeStyle(cfg *config.Config, palette *styling.Palette) barStyle {
	bar := cfg.Bar
	resolve := func(cv config.ColorValue) render.Color {
		color, ok := styling.ResolveColor(cv, palette)
		if !ok {
			return render.Color(0)
		}
		return color
	}

	// The dropdown buttons' hover and active shades deepen the group
	// background at the button bg opacity.
	buttonBase := resolve(bar.ButtonGroupBackground)
	buttonBg := styling.ColorMix(buttonBase, transparentColor, int(bar.ButtonBGOpacity))
	hoverBase, activeBase := buttonBase, buttonBase

	return barStyle{
		palette:        palette,
		fg:             resolve(mustToken(config.TokenFgDefault)),
		moduleGap:      barview.ModuleGap(cfg),
		groupGap:       barview.GroupGap(cfg),
		labelPx:        barview.LabelPx(cfg),
		buttonBgHover:  styling.ColorMix(hoverBase, buttonBg, 50),
		buttonBgActive: styling.ColorMix(activeBase, buttonBg, 80),
	}
}

// mustToken wraps a literal token lookup; a typo in one of these
// constants is a programming error, not a config error.
func mustToken(t config.CssToken) config.ColorValue {
	return config.ColorValue{Kind: config.ColorToken, Token: t}
}

var transparentColor = render.Color(0)
