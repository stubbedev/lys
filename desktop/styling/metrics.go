package styling

import (
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/tokens"
)

type metric struct {
	name string
	rem  tokens.Rem
}

var spaces = [...]metric{
	{"2xs", tokens.Space2XS},
	{"xs", tokens.SpaceXS},
	{"sm", tokens.SpaceSM},
	{"md", tokens.SpaceMD},
	{"lg", tokens.SpaceLG},
	{"xl", tokens.SpaceXL},
}

var radii = [...]metric{
	{"sm", tokens.RadiusSM},
	{"md", tokens.RadiusMD},
	{"lg", tokens.RadiusLG},
	{"xl", tokens.RadiusXL},
}

// MetricsCSS declares tokens' spacing, radius and field on :root: each
// rem step times --global-scale, its bar twins over --bar-scale, the
// pill radius, and the field's fill and ring as mixes of --palette-fg.
func MetricsCSS() string {
	var b strings.Builder
	decl := func(name, value string) { b.WriteString("    " + name + ": " + value + ";\n") }
	scaled := func(m metric, scale string) string {
		return "calc(" + strconv.FormatFloat(float64(m.rem), 'f', -1, 64) + "rem * var(" + scale + "))"
	}
	mix := func(p tokens.Percent) string {
		return "color-mix(in srgb, var(--palette-fg) " + strconv.Itoa(int(p)) + "%, transparent)"
	}
	b.WriteString(":root {\n")
	for _, m := range spaces {
		decl("--space-"+m.name, scaled(m, "--global-scale"))
		decl("--bar-space-"+m.name, scaled(m, "--bar-scale"))
	}
	for _, m := range radii {
		decl("--radius-"+m.name, scaled(m, "--global-scale"))
		decl("--bar-radius-"+m.name, scaled(m, "--bar-scale"))
		decl("--bar-button-radius-"+m.name, scaled(m, "--bar-scale"))
	}
	decl("--radius-full", strconv.Itoa(tokens.RadiusFullPx)+"px")
	decl("--field-fill", mix(tokens.FieldFill))
	decl("--field-border", mix(tokens.FieldBorder))
	b.WriteString("}")
	return b.String()
}
