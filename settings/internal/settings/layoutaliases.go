package settings

import (
	"slices"

	"github.com/stubbedev/lys/desktop/config"
)

const listLayoutNames = "keyboard-layout-name"

// layoutAliasRow gives keyboard layouts short names in the bar: a row
// per layout, by the name the compositor reports for it, picked from the
// xkb layouts and variants, with lys-wm's own layouts always listed.
var layoutAliasRow = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
		var configured []string
		if k.parts.wm {
			configured = layoutCodes(wmKeyboard(k))
		}
		all := systemLists[listLayoutNames]
		return newChoiceListEditor(k, s, func() []choice { return pinLayouts(all(), configured) }, namedChoices)
	}
}

// layoutNames are the xkb layouts and variants by their description, the
// name a compositor reports, each grouped under its xkb code.
func layoutNames(x xkbLists) []choice {
	out := make([]choice, 0, len(x.layouts)+len(x.variants))
	for _, l := range x.layouts {
		out = append(out, choice{value: l.label, label: l.label, group: l.value})
	}
	for _, v := range x.variants {
		out = append(out, choice{value: v.label, label: v.label, group: layoutCode(v.group, v.value)})
	}
	return sortedChoices(out)
}

func layoutCode(layout, variant string) string {
	if variant == "" {
		return layout
	}
	return layout + "(" + variant + ")"
}

// layoutCodes are the codes of the layouts a keyboard is set up with, each
// with its variant.
func layoutCodes(kb keyboardNames) []string {
	variants := splitList(kb.variant)
	out := make([]string, 0, len(variants))
	for i, l := range splitList(kb.layout) {
		v := ""
		if i < len(variants) {
			v = variants[i]
		}
		out = append(out, layoutCode(l, v))
	}
	return out
}

func pinLayouts(names []choice, codes []string) []choice {
	out := slices.Clone(names)
	for i := range out {
		out[i].pinned = slices.Contains(codes, out[i].group)
	}
	return out
}
