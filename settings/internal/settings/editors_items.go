package settings

import (
	"log"
	"reflect"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
)

type itemForm[T any] struct {
	defaults map[string]any
}

func formOf[T any](defaults T) itemForm[T] {
	m, ok := config.Encode(defaults).(map[string]any)
	if !ok {
		log.Panicf("settings: %T does not encode as a map", defaults)
	}
	return itemForm[T]{defaults: m}
}

func (f itemForm[T]) meta(key string) config.FieldMeta {
	m, ok := config.ItemField[T](key)
	if !ok {
		log.Panicf("settings: %s has no field %q", reflect.TypeFor[T](), key)
	}
	return m
}

func (f itemForm[T]) label(key string) string {
	l, ok := config.ItemI18nKey[T](key)
	if !ok {
		log.Panicf("settings: %s has no labeled field %q", reflect.TypeFor[T](), key)
	}
	return l
}

func (f itemForm[T]) shown(s slot, key string) slot {
	return slot{
		get: func() any {
			if v := s.get(); v != nil {
				return v
			}
			return f.defaults[key]
		},
		set: s.set, unset: s.unset,
	}
}

func (f itemForm[T]) with(key string, editor func(k *kit, s slot, meta config.FieldMeta) control) cardField {
	meta := f.meta(key)
	return cardField{key: key, label: f.label(key), editor: func(k *kit, s slot) control { return editor(k, f.shown(s, key), meta) }}
}

func (f itemForm[T]) field(key string) cardField {
	return f.with(key, func(k *kit, s slot, meta config.FieldMeta) control {
		c, err := autoEditor(k, s, "", meta)
		if err != nil {
			log.Panicf("settings: %s.%s: %v", reflect.TypeFor[T](), key, err)
		}
		return c
	})
}

func (f itemForm[T]) identity(key string) *cardField {
	c := f.with(key, func(k *kit, s slot, _ config.FieldMeta) control { return newLiveText(k, s, "") })
	return &c
}

func filled(c cardField) cardField {
	c.fill = true
	return c
}

func liveTextEditor(k *kit, s slot, _ config.FieldMeta) control { return newLiveText(k, s, "") }

func iconEditorOf(k *kit, s slot, _ config.FieldMeta) control { return newIconEditor(k, s) }

func keyed(key string) func(map[string]any) bool {
	return func(m map[string]any) bool { s, _ := m[key].(string); return s != "" }
}

type liveTextArea struct {
	*widget.TextArea
	slot    slot
	syncing bool
}

func newLiveTextArea(k *kit, s slot, _ config.FieldMeta) control {
	c := &liveTextArea{TextArea: widget.NewTextArea(k.face, k.textPx(), 0), slot: s}
	c.SetWrap(true)
	c.AddClass("live-text-area")
	c.OnChanged = func() {
		if !c.syncing {
			_ = c.slot.set(c.Text())
		}
	}
	c.refresh()
	return c
}

func (c *liveTextArea) refresh() {
	if text := c.slot.text(); text != c.Text() {
		c.syncing = true
		c.SetText(text)
		c.syncing = false
	}
}
