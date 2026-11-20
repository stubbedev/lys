// Package launcherview draws lys-shell's launcher from a config: the
// panel and where it sits on an output, the field and footer around the
// list, the virtualized list and its rows, the mode tabs, and the CSS a
// font or a style preset adds. It holds no engine, socket or surface, so
// lys-shell hosts it on a layer surface and lys-settings previews it from
// a draft config with the same code.
package launcherview

import (
	"math"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
)

// The launcher's measures, in logical pixels at scale 1: every session is
// these, whichever mode it shows.
const (
	topFraction     = 0.22
	edgeGap         = 32
	InputRowHeight  = 62
	FooterHeight    = 36
	SearchRowHeight = 46
	HeaderRowHeight = 28
	SearchIconSize  = 26
	entryFontFactor = 1.45
	GlyphFontFactor = 0.8
	RowEdgeGap      = 8
	RowHalfGap      = 2
	searchIconPx    = 22
	SearchIconName  = "ld-search-symbolic"
)

// ScaledPx is the styling scale's measure of logical pixels.
func ScaledPx(cfg *config.Config) func(int) int {
	scale := float64(cfg.Styling.Scale)
	return func(v int) int { return int(math.Round(float64(v) * scale)) }
}

// WidthPx is launcher.width in pixels.
func WidthPx(cfg *config.Config) int {
	return int(math.Round(cfg.Launcher.Width.ResolvePx(config.LauncherWidthBaseRem*16, float64(cfg.Styling.Scale))))
}

// FontPx is the body size, which the launcher's labels take before the
// stylesheet or a font sizes them.
func FontPx() float64 { return widgets.TextPx(styling.Body, 1) }

// SmallPx is the small text size: footers, subtitles, facts.
func SmallPx() float64 { return math.Round(FontPx() * 0.85) }

// RowMargin is the gap around every list row.
func RowMargin(px func(int) int) render.Insets {
	return render.Insets{Top: px(RowHalfGap), Bottom: px(RowHalfGap), Left: px(RowEdgeGap), Right: px(RowEdgeGap)}
}

// NewRowBox is a list row's box, in any session.
func NewRowBox() *widget.Box {
	th := widget.Current()
	r := widget.NewBox(widget.Row, th.Spacing+4, 0)
	r.SetPadding(render.Insets{Left: th.Padding + 6, Right: th.Padding + 6})
	r.AddClass("launcher-search-row")
	return r
}

// NewPanelBox is the rounded panel every session's content sits in.
func NewPanelBox() *widget.Box {
	b := widget.NewBox(widget.Column, 0, 0)
	b.AddClass("launcher-panel")
	return b
}

// Place is where a location puts a panel: centre is the search panel's
// place, above the middle with its top fixed so it grows down; the rest
// sit against their edges, offset by x and y (a positive x moves right, a
// positive y down).
func Place(out render.Rect, w, h int, loc config.LauncherLocation, x, y int) render.Rect {
	r := render.Rect{X: out.X + (out.W-w)/2, Y: out.Y + (out.H-h)/2, W: w, H: h}
	switch loc {
	case config.LauncherCenter:
		r.Y = out.Y + int(float64(out.H)*topFraction)
		return r
	case config.LauncherNorth, config.LauncherNorthEast, config.LauncherNorthWest:
		r.Y = out.Y + y
	case config.LauncherSouth, config.LauncherSouthEast, config.LauncherSouthWest:
		r.Y = out.Y + out.H - h + y
	}
	switch loc {
	case config.LauncherWest, config.LauncherNorthWest, config.LauncherSouthWest:
		r.X = out.X + x
	case config.LauncherEast, config.LauncherNorthEast, config.LauncherSouthEast:
		r.X = out.X + out.W - w + x
	}
	return r
}

// WidthIn is width held inside an output, whose size may not be known
// yet.
func WidthIn(out render.Rect, width int) int {
	if out.W > 0 {
		width = min(width, out.W-2*edgeGap)
	}
	return max(width, 1)
}

// RoomBelow is the tallest a panel may grow in an output of a known size:
// from where the centred panel's top sits to a gap above the bottom.
func RoomBelow(out render.Rect) int {
	return int(float64(out.H)*(1-topFraction)) - edgeGap
}
