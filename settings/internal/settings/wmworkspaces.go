package settings

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

const (
	workspacesPath  = "workspaces"
	scratchpadsPath = "scratchpads"
	itemNameKey     = "name"
)

// wmWorkspacesRow is lys-wm's workspace rules as cards: each titled by
// its name, its number or the workspaces it matches, a new one taking the
// lowest number no rule holds yet.
func wmWorkspacesRow() rowSpec {
	return wmField(workspacesPath, func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			check := k.wm.checker(workspacesPath)
			return newCardList(k, s, cardSpec{
				check:      func(items []any) func() error { return check(items) },
				title:      workspaceTitle,
				fields:     wmItemFields(k.wm, []string{workspacesPath, wmconfig.Each}, nil, workspacesPath),
				blank:      func() map[string]any { return map[string]any{"id": nextWorkspaceID(s.get())} },
				searchable: true,
				collapsed:  true,
			})
		}
	})
}

func workspaceTitle(item map[string]any, number int) string {
	t := locale.T()
	if name, _ := item[itemNameKey].(string); name != "" {
		return name
	}
	if id, ok := item["id"]; ok {
		return t.Get("wm-workspace-number", i18n.Int("number", int64(toFloat(id))))
	}
	if match, ok := item["match"].(string); ok {
		if key := config.EnumLabelKey(wmChoiceType("workspaces.match"), match); t.Has(key) {
			return t.Get(key)
		}
		return match
	}
	return t.Get("wm-card-number", i18n.Int("number", int64(number)))
}

func nextWorkspaceID(rules any) int64 {
	taken := map[int64]bool{}
	items, _ := rules.([]any)
	for _, it := range items {
		if id, ok := asMap(it)["id"]; ok {
			taken[int64(toFloat(id))] = true
		}
	}
	id := int64(1)
	for taken[id] {
		id++
	}
	return id
}

// wmScratchpadsRow is lys-wm's named scratchpads as cards, the name
// in each card's header.
func wmScratchpadsRow() rowSpec {
	return wmNamedCards(scratchpadsPath, "wm-scratchpads-name", nil)
}

// namedCardBody is what a named card holds below its name, and what a new
// one starts as.
type namedCardBody struct {
	fields []cardField
	blank  func() map[string]any
}

// wmNamedCards is the lys-wm mapping at path as cards: each entry a card
// with its name in the header and, unless body says otherwise, its
// value's keys below it.
func wmNamedCards(path, nameKey string, body func(k *kit) namedCardBody) rowSpec {
	return wmField(path, func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			check := k.wm.checker(path)
			var b namedCardBody
			if body != nil {
				b = body(k)
			} else {
				b = namedCardBody{fields: wmItemFields(k.wm, []string{path, wmconfig.Each}, nil, path), blank: func() map[string]any { return map[string]any{} }}
			}
			return newCardList(k, namedItemsSlot(s), cardSpec{
				check: func(items []any) func() error {
					named, err := namedItems(items)
					if err != nil {
						return func() error { return err }
					}
					return check(named)
				},
				identity: &cardField{key: itemNameKey, label: nameKey, fill: true, editor: func(k *kit, s slot) control {
					return newLiveText(k, s, locale.T().Get(nameKey))
				}},
				fields: b.fields,
				blank:  b.blank,
				keep:   presetNamed,
				unique: itemNameKey,
			})
		}
	})
}

// namedItemsSlot shows a mapping of names to objects as a list of
// objects, each holding its name under name, in the order of the names;
// a list written back is the mapping again.
func namedItemsSlot(s slot) slot {
	return slot{
		get: func() any {
			m := asMap(s.get())
			out := make([]any, 0, len(m))
			for _, name := range slices.Sorted(maps.Keys(m)) {
				item := maps.Clone(asMap(m[name]))
				if item == nil {
					item = map[string]any{}
				}
				item[itemNameKey] = name
				out = append(out, item)
			}
			return out
		},
		set: func(v any) error {
			items, ok := v.([]any)
			if !ok {
				return fmt.Errorf("settings: %v is no list of named items", v)
			}
			named, err := namedItems(items)
			if err != nil {
				return err
			}
			if len(named) == 0 {
				s.unset()
				return nil
			}
			return s.set(named)
		},
		unset: s.unset,
	}
}

// namedItems is the mapping a list of named objects stands for, refusing
// an item without a name and a name given twice.
func namedItems(items []any) (map[string]any, error) {
	out := make(map[string]any, len(items))
	for _, it := range items {
		item := maps.Clone(asMap(it))
		name, _ := item[itemNameKey].(string)
		if name == "" {
			return nil, fmt.Errorf("settings: %v has no name", it)
		}
		if _, dup := out[name]; dup {
			return nil, fmt.Errorf("settings: %q names two items", name)
		}
		delete(item, itemNameKey)
		out[name] = item
	}
	return out, nil
}

// percentSizeEditor is a size written as <width>%x<height>%: a number
// for each, from 1 to 100, named by its leading addon and in percent by
// its trailing one; a value of another form is kept and marked
// until one of them is changed.
type percentSizeEditor struct {
	*widget.Box
	slot    slot
	sides   [2]*widget.SpinButton
	unknown *widget.Badge
	syncing bool
}

var percentSizeLabels = [2]string{"wm-size-width", "wm-size-height"}

func newPercentSizeEditor(k *kit, s slot, _ []string) control {
	t := locale.T()
	c := &percentSizeEditor{Box: gapBox(widget.Row), slot: s}
	c.unknown = k.unknownBadge()
	c.AppendAligned(c.unknown, false, widget.AlignCenter)
	for i := range c.sides {
		sb := widget.NewSpinButton(k.face, k.textPx(), 0, 1, 100, 1, 0)
		name := t.Get(percentSizeLabels[i])
		sb.SetOptional(t.Get("settings-inherited"))
		sb.SetTooltip(name)
		sb.SetAccessibleName(name)
		sb.OnCleared = s.unset
		sb.OnValueChanged = func(float64) { c.write() }
		c.sides[i] = sb
		field := k.inUnit(sb, unitPercent)
		field.Prepend(k.label(name))
		c.AppendAligned(field, false, widget.AlignCenter)
	}
	c.refresh()
	return c
}

func (c *percentSizeEditor) write() {
	if c.syncing {
		return
	}
	var parts [2]string
	for i, sb := range c.sides {
		if sb.Empty() {
			sb.SetValue(100)
		}
		parts[i] = strconv.Itoa(int(math.Round(sb.Value()))) + "%"
	}
	_ = c.slot.set(parts[0] + "x" + parts[1])
}

func (c *percentSizeEditor) refresh() {
	c.syncing = true
	defer func() { c.syncing = false }()
	text, _ := c.slot.get().(string)
	w, h, ok := parsePercentSize(text)
	c.unknown.SetVisible(text != "" && !ok)
	if !ok {
		for _, sb := range c.sides {
			sb.Clear()
		}
		return
	}
	c.sides[0].SetValue(float64(w))
	c.sides[1].SetValue(float64(h))
}

func parsePercentSize(text string) (w, h int, ok bool) {
	ws, hs, found := strings.Cut(text, "x")
	if !found {
		return 0, 0, false
	}
	w, okW := percentPart(ws)
	h, okH := percentPart(hs)
	return w, h, okW && okH
}

func percentPart(s string) (int, bool) {
	digits, ok := strings.CutSuffix(s, "%")
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(digits)
	return n, err == nil && n >= 1 && n <= 100
}
