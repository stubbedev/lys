package launcherview

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
)

// cssPriority puts a session's font and style CSS above the shell's own
// stylesheet: at anything below, a font simply lost to [styling] and
// looked like it did nothing.
const cssPriority = 100

// Look is how a session's panel looks and where it sits: what
// [launcher] sets, which a session's own options may override.
type Look struct {
	Location         config.LauncherLocation
	OffsetX, OffsetY int
	// Width is in pixels.
	Width     int
	Lines     int
	Fixed     bool
	Sidebar   bool
	ShowIcons bool
	// Font is a Pango description, "" for the shell's.
	Font string
	// Style is a style preset's CSS, "" for none.
	Style string
}

// LookOf is the look [launcher] in cfg sets, with no style preset.
func LookOf(cfg *config.Config) Look {
	l := cfg.Launcher
	return Look{
		Location:  l.Location,
		Width:     WidthPx(cfg),
		Lines:     int(l.Lines),
		Fixed:     l.FixedNumLines,
		Sidebar:   l.SidebarMode,
		ShowIcons: l.ShowIcons,
		Font:      l.Font,
	}
}

// CSS is what the look adds to the shell's stylesheet: the font scoped to
// the panel, then the style preset; "" when it adds nothing.
func (l Look) CSS() string {
	var css strings.Builder
	if font := strings.TrimSpace(l.Font); font != "" {
		fmt.Fprintf(&css, ".launcher-surface { %s }\n", fontProperties(font))
	}
	css.WriteString(l.Style)
	if strings.TrimSpace(css.String()) == "" {
		return ""
	}
	return css.String()
}

// Stylesheet is CSS as a stylesheet above a host's sheet at base; false
// when the look adds nothing.
func (l Look) Stylesheet(base int) (*widget.Stylesheet, bool) {
	css := l.CSS()
	if css == "" {
		return nil, false
	}
	return widget.NewStylesheet(css, base+cssPriority), true
}

// fontDescription is a parsed Pango font description.
type fontDescription struct {
	family string
	// size is in points, or pixels when absolute.
	size     float64
	absolute bool
	italic   bool
	// weight is the CSS numeric weight; 0 leaves it unset.
	weight int
}

// pangoWeights are the weight words pango_font_description_from_string
// takes, to their numeric weights.
var pangoWeights = map[string]int{
	"thin": 100, "ultra-light": 200, "extra-light": 200, "light": 300, "semi-light": 350,
	"demi-light": 350, "book": 380, "regular": 400, "medium": 500, "semi-bold": 600,
	"demi-bold": 600, "bold": 700, "ultra-bold": 800, "extra-bold": 800, "heavy": 900,
	"black": 900, "ultra-heavy": 1000, "extra-heavy": 1000,
}

// parseFontDescription reads "[FAMILY-LIST] [STYLE-OPTIONS] [SIZE]":
// the size last (a trailing "px" makes it absolute), style and weight
// words before it, the rest the family.
func parseFontDescription(desc string) fontDescription {
	words := strings.Fields(strings.ReplaceAll(desc, ",", " "))
	var fd fontDescription
	if n := len(words); n > 0 {
		last := words[n-1]
		absolute := strings.HasSuffix(last, "px")
		if v, err := strconv.ParseFloat(strings.TrimSuffix(last, "px"), 64); err == nil && v > 0 {
			fd.size, fd.absolute = v, absolute
			words = words[:n-1]
		}
	}
	for len(words) > 0 {
		w := strings.ToLower(words[len(words)-1])
		switch {
		case w == "italic" || w == "oblique":
			fd.italic = true
		case w == "normal" || w == "roman":
		case pangoWeights[w] != 0:
			fd.weight = pangoWeights[w]
		default:
			fd.family = strings.Join(words, " ")
			return fd
		}
		words = words[:len(words)-1]
	}
	return fd
}

// fontProperties turns a Pango description into CSS properties. The
// syntaxes are not interchangeable: Pango puts the size last
// ("Monospace 20"), CSS's font shorthand first, so handing CSS the
// Pango string silently drops the declaration.
func fontProperties(desc string) string {
	fd := parseFontDescription(desc)
	var props []string
	if fd.family != "" {
		props = append(props, fmt.Sprintf("font-family: %q;", fd.family))
	}
	if fd.size > 0 {
		unit := "pt"
		if fd.absolute {
			unit = "px"
		}
		props = append(props, "font-size: "+strconv.FormatFloat(fd.size, 'f', -1, 64)+unit+";")
	}
	if fd.italic {
		props = append(props, "font-style: italic;")
	}
	if fd.weight > 0 && fd.weight != 400 {
		props = append(props, fmt.Sprintf("font-weight: %d;", fd.weight))
	}
	return strings.Join(props, " ")
}
