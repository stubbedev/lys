package settings

import (
	"fmt"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	wmLayoutPath  = "input.keyboard.layout"
	wmVariantPath = "input.keyboard.variant"
	listXKBSource = "xkb-source"
)

// xkbSources are the xkb layouts and variants by their layout code, the
// "layout(variant)" an input source is written as.
func xkbSources(x xkbLists) []choice {
	out := make([]choice, 0, len(x.layouts)+len(x.variants))
	for _, l := range x.layouts {
		out = append(out, choice{value: l.value, label: l.label})
	}
	for _, v := range x.variants {
		out = append(out, choice{value: layoutCode(v.group, v.value), label: v.label})
	}
	return sortedChoices(out)
}

func splitLayoutCode(code string) (layout, variant string) {
	layout, rest, ok := strings.Cut(code, "(")
	if !ok {
		return code, ""
	}
	return layout, strings.TrimSuffix(rest, ")")
}

func wmText(sc wmScope, path string) string {
	s, _ := sc.value(path).(string)
	return s
}

// sourcesAligned says whether every variant has its layout, so the pair of
// keys reads as a list of input sources.
func sourcesAligned(sc wmScope) bool {
	return len(splitList(wmText(sc, wmVariantPath))) <= len(splitList(wmText(sc, wmLayoutPath)))
}

// inputSourcesSlot is the layout and variant keys as one list of layout
// codes, written together so the two lists never part.
func inputSourcesSlot(sc wmScope) slot {
	return slot{
		get: func() any {
			codes := layoutCodes(keyboardNames{layout: wmText(sc, wmLayoutPath), variant: wmText(sc, wmVariantPath)})
			out := make([]any, len(codes))
			for i, c := range codes {
				out[i] = c
			}
			return out
		},
		set: func(v any) error {
			items, ok := v.([]any)
			if !ok {
				return fmt.Errorf("settings: input sources %v are not a list", v)
			}
			layouts, variants := make([]string, len(items)), make([]string, len(items))
			for i, it := range items {
				code, ok := it.(string)
				if !ok {
					return fmt.Errorf("settings: input source %v is not a layout code", it)
				}
				layouts[i], variants[i] = splitLayoutCode(code)
			}
			return sc.edit(sc.editing(wmLayoutPath, strings.Join(layouts, ",")), sc.editing(wmVariantPath, joinTrimmed(variants)))
		},
		unset: func() {
			sc.slot(wmVariantPath).unset()
			sc.slot(wmLayoutPath).unset()
		},
	}
}

var inputSourcesRow rowOpt = func(r *rowSpec) {
	r.fullWidth = true
	r.also = []string{wmVariantPath}
	r.scoped = func(k *kit, sc wmScope, s slot, meta config.FieldMeta) control {
		if !sourcesAligned(sc) {
			return newMisalignedSources(k, sc, s, meta)
		}
		return newChoiceListEditor(k, inputSourcesSlot(sc), systemLists[listXKBSource], plainChoices)
	}
}

// misalignedSources is a layout and variant pair that does not read as a
// list, more variants than layouts: both keys as they are, untouched until
// edited.
type misalignedSources struct {
	*widget.Box
	controls []control
}

func newMisalignedSources(k *kit, sc wmScope, layout slot, layoutMeta config.FieldMeta) *misalignedSources {
	m := &misalignedSources{Box: gapBox(widget.Column)}
	m.Append(wrapped(k.label(locale.T().Get("wm-input-sources-misaligned"), "format-preview", css.Caption, css.DimLabel)), false)
	variantMeta, _ := sc.field(wmVariantPath)
	for _, f := range []struct {
		path string
		s    slot
		meta config.FieldMeta
	}{{wmLayoutPath, layout, layoutMeta}, {wmVariantPath, sc.slot(wmVariantPath), variantMeta}} {
		c, ok := semanticEditor(k, f.s, f.path, f.meta)
		if !ok {
			panic(fmt.Sprintf("settings: %s has no xkb list", f.path))
		}
		m.controls = append(m.controls, c)
		m.Append(c, false)
	}
	return m
}

func (m *misalignedSources) refresh() {
	for _, c := range m.controls {
		c.refresh()
	}
}
