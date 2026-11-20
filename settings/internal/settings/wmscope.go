package settings

import (
	"log"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"

	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

// wmScope is where a lys-wm row's paths land: lys-wm's config itself, or
// one input device's overrides of a class section, whose rows keep the
// section's paths.
type wmScope interface {
	backend
	value(path string) any
	editing(path string, v any) wmconfig.Edit
	clearing(path string) wmconfig.Edit
	edit(edits ...wmconfig.Edit) error
}

// scopedEditor is a lys-wm row's editor that reaches keys besides its own,
// through the scope its row edits.
type scopedEditor func(k *kit, sc wmScope, s slot, meta config.FieldMeta) control

func (w *wmBackend) editing(path string, v any) wmconfig.Edit {
	return wmconfig.Edit{Path: wmPath(path), Value: v}
}

// clearIn takes the key at path out of what lys-wm reads in sc through s,
// the key's slot.
func clearIn(sc wmScope, s slot, path string) {
	if sc.clearing(path).Unset {
		s.unset()
		return
	}
	_ = s.set(nil)
}

// deviceScope is one input device's overrides of a class section: a row's
// path under section lands under at, the device's entry in input.devices
// or a key of it. A key it does not set inherits the section's.
type deviceScope struct {
	w       *wmBackend
	section string
	at      []string
}

func newDeviceScope(w *wmBackend, section, name string, within ...string) deviceScope {
	return deviceScope{w: w, section: section, at: append([]string{"input", "devices", name}, within...)}
}

func (d deviceScope) keys(path string) []string {
	rest, ok := strings.CutPrefix(path, d.section+".")
	if !ok {
		log.Panicf("settings: %s is outside the %s a device overrides", path, d.section)
	}
	return append(slices.Clone(d.at), wmPath(rest)...)
}

func (d deviceScope) field(path string) (config.FieldMeta, bool) { return d.w.metaAt(d.keys(path)) }

func (d deviceScope) value(path string) any { return d.w.valueAt(d.keys(path)) }

func (d deviceScope) slot(path string) slot {
	keys := d.keys(path)
	raw := slot{
		get:   func() any { return d.w.valueAt(keys) },
		set:   func(v any) error { return d.w.edit(wmconfig.Edit{Path: keys, Value: v}) },
		unset: func() { d.reset(path) },
	}
	f, ok := d.w.schema.Field(keys)
	if !ok {
		log.Panicf("settings: lys-wm's devices have no %s", strings.Join(keys, "."))
	}
	return wmChoiceSlot(f, raw)
}

func (d deviceScope) reset(path string) {
	if err := d.w.edit(d.clearing(path)); err != nil {
		log.Printf("settings: reset %s: %v", strings.Join(d.keys(path), "."), err)
	}
}

func (d deviceScope) defaultOf(string) (any, bool) { return nil, false }

func (d deviceScope) source(path string) sourceInfo {
	keys := d.keys(path)
	info := d.w.badgeAt(keys)
	info.reset = d.w.valueAt(keys) != nil
	return info
}

func (d deviceScope) editing(path string, v any) wmconfig.Edit {
	return wmconfig.Edit{Path: d.keys(path), Value: v}
}

func (d deviceScope) clearing(path string) wmconfig.Edit { return d.w.clearingAt(d.keys(path)) }

func (d deviceScope) edit(edits ...wmconfig.Edit) error { return d.w.edit(edits...) }

// sets reports whether the device sets any key of rows.
func (d deviceScope) sets(rows []rowSpec) bool {
	return slices.ContainsFunc(rows, func(r rowSpec) bool {
		return slices.ContainsFunc(append([]string{r.path}, r.also...), func(p string) bool { return d.value(p) != nil })
	})
}
