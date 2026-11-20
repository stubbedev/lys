package settings

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
)

const themeCardW = 152

// paletteKeys are the [styling.palette] keys a theme sets, in the
// palette's order.
var paletteKeys = []string{"bg", "surface", "elevated", "fg", "fg-muted", "primary", "red", "yellow", "green", "blue"}

// availableThemes lists the selectable themes (styling.available):
// the built-ins and the config's themes/ files (a seam for tests).
var availableThemes = func(s store) []config.ThemeEntry {
	themes, skipped := config.LoadThemes(filepath.Join(s.svc.Dir(), "themes"))
	for _, err := range skipped {
		log.Printf("settings: theme: %v", err)
	}
	return themes
}

func paletteHexes(p config.Palette) []string {
	return []string{p.Bg, p.Surface, p.Elevated, p.Fg, p.FgMuted, p.Primary, p.Red, p.Yellow, p.Green, p.Blue}
}

// paletteColors is a palette as colors, in paletteKeys' order.
func paletteColors(p config.Palette) ([]render.Color, error) {
	hexes := paletteHexes(p)
	out := make([]render.Color, len(hexes))
	for i, hex := range hexes {
		col, ok := styling.ParseHex(hex)
		if !ok {
			return nil, fmt.Errorf("the palette's %s %q is no hex color", paletteKeys[i], hex)
		}
		out[i] = col
	}
	return out, nil
}

// themeTitle reads a theme's file name as words: "rose-pine-moon" is
// "Rose Pine Moon".
func themeTitle(name string) string {
	words := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' })
	for i, w := range words {
		r, size := utf8.DecodeRuneInString(w)
		words[i] = string(unicode.ToUpper(r)) + w[size:]
	}
	return strings.Join(words, " ")
}

// miniDesktop draws a desktop in a theme's colors: its bar on the
// surface with the primary in it, a window on the elevated color over
// the background, and the four signal colors.
func miniDesktop(colors []render.Color) func(cv *render.Canvas, r render.Rect) {
	bg, surface, elevated, fg, fgMuted, primary := colors[0], colors[1], colors[2], colors[3], colors[4], colors[5]
	signals := colors[6:]
	return func(cv *render.Canvas, r render.Rect) {
		cv.RoundedRect(r, tileRadius/2, bg)
		bar := render.Rect{X: r.X, Y: r.Y, W: r.W, H: max(4, r.H/6)}
		cv.RoundedRect(bar, tileRadius/2, surface)
		dot := bar.H / 2
		cv.RoundedRect(render.Rect{X: bar.X + dot, Y: bar.Y + (bar.H-dot)/2, W: dot * 3, H: dot}, dot/2, primary)
		cv.RoundedRect(render.Rect{X: bar.X + bar.W - dot*4, Y: bar.Y + (bar.H-dot)/2, W: dot * 3, H: dot}, dot/2, fg)
		win := render.Rect{X: r.X + r.W/8, Y: bar.Y + bar.H + r.H/8, W: r.W * 5 / 8, H: r.H - bar.H - r.H/4}
		cv.RoundedRect(win, tileRadius/2, elevated)
		windowLines(cv, win, fgMuted)
		size := max(3, r.H/8)
		for i, col := range signals {
			x := r.X + r.W - size - r.W/16
			y := win.Y + i*(size+size/2)
			cv.RoundedRect(render.Rect{X: x, Y: y, W: size, H: size}, size/2, col)
		}
	}
}

// themeCards is the theme presets as cards, each drawing a desktop in
// its colors: choosing one names it the palette's base theme and writes
// its colors.
func themeCards(k *kit) *choiceCards {
	themes := availableThemes(k.store)
	names := make([]string, 0, len(themes))
	titles := make([]string, 0, len(themes))
	pictures := make(map[string]picture, len(themes))
	byName := make(map[string]config.ThemeEntry, len(themes))
	for _, th := range themes {
		colors, err := paletteColors(th.Palette)
		if err != nil {
			log.Printf("settings: theme %s: %v", th.Name, err)
			continue
		}
		names = append(names, th.Name)
		titles = append(titles, themeTitle(th.Name))
		pictures[th.Name] = drawn(miniDesktop(colors))
		byName[th.Name] = th
	}
	c := newCards(k, slot{
		get: func() any { return k.store.value("styling.palette_base_theme") },
		set: func(v any) error {
			name, ok := v.(string)
			th, known := byName[name]
			if !ok || !known {
				return fmt.Errorf("settings: %v is no theme preset", v)
			}
			return applyTheme(k.store, th)
		},
		unset: func() {},
	}, names, titles, pictures)
	c.cardW = themeCardW
	c.AddClass("theme-cards")
	return c
}

// applyTheme names th the palette's base theme and writes its colors.
func applyTheme(s store, th config.ThemeEntry) error {
	if err := s.set("styling.palette_base_theme", th.Name); err != nil {
		return err
	}
	for i, hex := range paletteHexes(th.Palette) {
		if err := s.set("styling.palette."+paletteKeys[i], hex); err != nil {
			return err
		}
	}
	return nil
}

// themePreset is the row of theme preset cards.
func themePreset() rowSpec {
	return rowSpec{
		path: "styling.palette_base_theme", key: "settings-theme-preset", action: true, fullWidth: true,
		editor: func(k *kit, _ slot, _ config.FieldMeta) control { return themeCards(k) },
	}
}
