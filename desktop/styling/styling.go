// Package styling resolves lys-shell's design tokens to concrete colors.
// The palette is the
// runtime source of truth and every token is a color-mix derivation
// from it, defined once in Palette.Token: TokensCSS hands the
// stylesheet the same colors lys-ui's theme and the painters use.
package styling

import (
	"math"
	"strconv"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/tokens"
)

// RemBase is the rem the stylesheet's base values multiply.
const RemBase = tokens.RemPx

// Palette is the runtime color palette — the --palette-* custom
// properties. Themes replace it wholesale.
type Palette struct {
	Bg, Surface, Elevated render.Color
	Fg, FgMuted           render.Color
	Primary               render.Color
	Red, Yellow           render.Color
	Green, Blue           render.Color
	// HighContrast derives the tokens for high contrast: muted and subtle
	// text and every border are the text color itself.
	HighContrast bool
}

// Default is the built-in catppuccin-mocha palette, the set the shell
// falls back to when a palette cannot be resolved.
func Default() *Palette {
	hex, ok := config.PaletteByName("catppuccin-mocha")
	if !ok {
		panic("styling: no built-in catppuccin-mocha theme")
	}
	p, err := PaletteFromHex(hex)
	if err != nil {
		panic("styling: built-in catppuccin-mocha: " + err.Error())
	}
	return p
}

var (
	transparent = render.Color(0)
	white       = render.RGB(0xff, 0xff, 0xff)
	black       = render.RGB(0, 0, 0)
)

// ColorMix interpolates two premultiplied colors: color-mix(in srgb, a
// pct%, b). CSS interpolates premultiplied channels, which is exactly
// what interpolating render.Color's bytes does — straight-alpha edge
// cases (mixing with transparent) come out right by construction.
func ColorMix(a, b render.Color, pct int) render.Color {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	w := float64(pct) / 100
	var out uint32
	for shift := uint(0); shift <= 24; shift += 8 {
		ca := float64((uint32(a) >> shift) & 0xFF)
		cb := float64((uint32(b) >> shift) & 0xFF)
		out |= uint32(math.Round(float64(ca*w)+float64(cb*(1-w)))) << shift
	}
	return render.Color(out)
}

// Token resolves one design token against the palette: the color-mix
// table, one entry per token; a token without a mix is a palette color
// passed through. It is the only definition of the tokens.
func (p *Palette) Token(t config.CssToken) (render.Color, bool) {
	if p.HighContrast {
		switch t {
		case config.TokenFgMuted, config.TokenFgSubtle, config.TokenBorderSubtle, config.TokenBorderDefault, config.TokenBorderStrong:
			return p.Fg, true
		}
	}
	switch t {
	case config.TokenBgBase:
		return p.Bg, true
	case config.TokenBgSurface:
		return p.Surface, true
	case config.TokenBgSurfaceElevated:
		return ColorMix(p.Surface, p.Fg, 93), true
	case config.TokenBgElevated:
		return p.Elevated, true
	case config.TokenBgOverlay:
		return ColorMix(p.Elevated, p.Fg, 92), true
	case config.TokenBgHover:
		return ColorMix(p.Elevated, p.Fg, 88), true
	case config.TokenBgActive:
		return ColorMix(p.Elevated, p.Fg, 82), true
	case config.TokenBgSelected:
		return ColorMix(p.Primary, transparent, 15), true

	case config.TokenFgDefault:
		return p.Fg, true
	case config.TokenFgMuted:
		return p.FgMuted, true
	case config.TokenFgSubtle:
		return ColorMix(p.Fg, transparent, 50), true
	case config.TokenFgOnAccent:
		return p.Surface, true

	case config.TokenAccent:
		return p.Primary, true
	case config.TokenAccentSubtle:
		return ColorMix(p.Primary, transparent, 15), true
	case config.TokenAccentHover:
		return ColorMix(p.Primary, white, 80), true

	case config.TokenStatusError:
		return p.Red, true
	case config.TokenStatusWarning:
		return p.Yellow, true
	case config.TokenStatusSuccess:
		return p.Green, true
	case config.TokenStatusInfo:
		return p.Blue, true
	case config.TokenStatusErrorSubtle:
		return ColorMix(p.Red, transparent, 15), true
	case config.TokenStatusWarningSubtle:
		return ColorMix(p.Yellow, transparent, 15), true
	case config.TokenStatusSuccessSubtle:
		return ColorMix(p.Green, transparent, 15), true
	case config.TokenStatusInfoSubtle:
		return ColorMix(p.Blue, transparent, 15), true
	case config.TokenStatusErrorHover:
		return ColorMix(p.Red, black, 85), true

	case config.TokenRed:
		return p.Red, true
	case config.TokenYellow:
		return p.Yellow, true
	case config.TokenGreen:
		return p.Green, true
	case config.TokenBlue:
		return p.Blue, true

	case config.TokenBorderSubtle:
		return ColorMix(p.Fg, transparent, 12), true
	case config.TokenBorderDefault:
		return ColorMix(p.Elevated, p.Fg, 90), true
	case config.TokenBorderStrong:
		return ColorMix(p.Elevated, p.Fg, 82), true
	case config.TokenBorderAccent:
		return p.Primary, true
	case config.TokenBorderError:
		return p.Red, true
	}
	return 0, false
}

// ResolveColor resolves a config ColorValue against the palette: Auto
// defers to the accent,
// transparent is fully transparent, custom hex parses, tokens go
// through the token table. An invalid hex or unknown token is a load-
// time error by construction; the bool return covers the custom hex
// that only fails at resolve time.
func ResolveColor(cv config.ColorValue, p *Palette) (render.Color, bool) {
	switch cv.Kind {
	case config.ColorAuto:
		return p.Primary, true
	case config.ColorTransparent:
		return transparent, true
	case config.ColorCustom:
		return render.ParseHex(cv.Hex)
	case config.ColorToken:
		return p.Token(cv.Token)
	}
	return 0, false
}

// ParseHex is [render.ParseHex], the hex form a custom color value
// holds.
func ParseHex(s string) (render.Color, bool) { return render.ParseHex(s) }

// WidthStyle is the inline style that makes a widget exactly px wide,
// its border included: lys-ui's explicit width for content that fills
// whatever width it is offered (an Item, a progress bar), on a surface
// that would otherwise offer it the whole output.
func WidthStyle(px int) string {
	return "width: " + strconv.Itoa(px) + "px; box-sizing: border-box;"
}

// ToolkitRadiusPx is lys-ui's default corner radius, what the default
// rounding level resolves to.
func ToolkitRadiusPx() int { return widget.DarkTheme().Radius }

// RoundingRadiusPx resolves a rounding level to corner pixels: tokens'
// radius scale at RemBase times the scale, with full pinned at
// tokens.RadiusFullPx unscaled.
func RoundingRadiusPx(level config.RoundingLevel, scale float64) int {
	switch level {
	case config.RoundingFull:
		return tokens.RadiusFullPx
	case config.RoundingDefault:
		return int(math.Round(float64(ToolkitRadiusPx()) * scale))
	case config.RoundingSm:
		return int(math.Round(tokens.RadiusSM.Px(RemBase, scale)))
	case config.RoundingMd:
		return int(math.Round(tokens.RadiusMD.Px(RemBase, scale)))
	case config.RoundingLg:
		return int(math.Round(tokens.RadiusLG.Px(RemBase, scale)))
	default: // none, and anything invalid: sharp corners
		return 0
	}
}

// HexRGBA is [render.FormatHex], the hex form [ParseHex] reads back.
func HexRGBA(c render.Color) string { return render.FormatHex(c) }
