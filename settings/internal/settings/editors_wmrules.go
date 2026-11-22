package settings

import (
	"cmp"
	"errors"
	"log"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/shell/layers"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
	"github.com/stubbedev/lys/settings/internal/wmstate"
)

// ruleGroup is the part of the rule editor a key of a rule belongs in:
// the switch in its card's header, what the rule applies to, first or
// among the further conditions, or the group of effects it is filed under.
type ruleGroup int

const (
	ruleEnabled ruleGroup = iota
	ruleMatch
	ruleMoreMatch
	rulePlacement
	ruleAppearance
	ruleBehaviour
	rulePerformance
)

var ruleEffectGroups = []ruleGroup{rulePlacement, ruleAppearance, ruleBehaviour, rulePerformance}

func (g ruleGroup) labelKey() string {
	return [...]string{
		ruleEnabled:     wmKey(rulesLabels + "." + ruleEnabledKey),
		ruleMatch:       "wm-rules-applies-to",
		ruleMoreMatch:   "wm-rules-more-conditions",
		rulePlacement:   "wm-rules-group-placement",
		ruleAppearance:  "wm-rules-group-appearance",
		ruleBehaviour:   "wm-rules-group-behaviour",
		rulePerformance: "wm-rules-group-performance",
	}[g]
}

// ruleEnabledKey switches a rule of either list on or off.
const ruleEnabledKey = "enabled"

// rulesLabels is the label prefix every rule list's keys share, so a key
// window and layer rules both take reads alike in each.
const rulesLabels = rulesKey

// ruleKind is a lys-wm list of rules: where it is, the part of the editor
// each key of a rule belongs in, the fields of the keys a rule leads
// with, and what a rule's card shows of it.
type ruleKind struct {
	path   string
	groups map[string]ruleGroup
	match  func(w *wmBackend, item []string) []cardField
	title  func(item map[string]any, number int, apps installedApps) string
	icon   func(item map[string]any, apps installedApps) string
	pick   bool
	// apps says the cards name installed apps, read once the list shows.
	apps bool
}

var windowRules = ruleKind{
	path: rulesKey,
	groups: map[string]ruleGroup{
		ruleEnabledKey: ruleEnabled,
		appIDKey:       ruleMatch, titleKey: ruleMatch,
		"initial_app_id": ruleMoreMatch, "initial_title": ruleMoreMatch, "tag": ruleMoreMatch, "xwayland": ruleMoreMatch,
		"is_floating": ruleMoreMatch, "is_focused": ruleMoreMatch, "is_fullscreen": ruleMoreMatch, "is_urgent": ruleMoreMatch,
		"is_captured": ruleMoreMatch, "exclude": ruleMoreMatch,
		"workspace": rulePlacement, "output": rulePlacement, "float": rulePlacement, "tile": rulePlacement,
		"fullscreen": rulePlacement, "maximized": rulePlacement, "position": rulePlacement, "size": rulePlacement,
		"min_size": rulePlacement, "max_size": rulePlacement, "keep_aspect": rulePlacement, "pin": rulePlacement, "group": rulePlacement,
		"border": ruleAppearance, "rounding": ruleAppearance, "shadow": ruleAppearance, "opacity": ruleAppearance,
		"blur": ruleAppearance, "blur_look": ruleAppearance, "mica": ruleAppearance, "dim_inactive": ruleAppearance,
		"dim_around": ruleAppearance, "animation": ruleAppearance,
		"focus": ruleBehaviour, "on_activate": ruleBehaviour, "suppress": ruleBehaviour, "idle_inhibit": ruleBehaviour,
		"scroll_factor": ruleBehaviour, "touch_gestures": ruleBehaviour, "block_out_from": ruleBehaviour,
		"vrr": rulePerformance, "tearing": rulePerformance, "content": rulePerformance,
	},
	match: func(w *wmBackend, item []string) []cardField {
		app := cardField{key: appIDKey, label: wmKey(rulesLabels + "." + appIDKey), fill: true, editor: func(k *kit, s slot) control {
			return newChoiceEditor(k, s, choiceSpec{list: func() []choice { return ruleAppChoices(k) }, free: true})
		}}
		return append([]cardField{app}, wmKeyFields(w, item, nil, rulesLabels, titleKey)...)
	},
	title: func(item map[string]any, number int, apps installedApps) string {
		return wmCardTitle(item, number, []string{appIDKey, titleKey}, appNames(apps))
	},
	icon: func(item map[string]any, apps installedApps) string {
		pattern, _ := item[appIDKey].(string)
		if id, exact := exactAppID(pattern); exact {
			if a, ok := apps.byID[id]; ok && a.Icon() != "" {
				return a.Icon()
			}
		}
		return appFallbackIcon
	},
	pick: true,
	apps: true,
}

const (
	layerRulesKey = "layer_rules"
	namespaceKey  = "namespace"
)

var layerRules = ruleKind{
	path: layerRulesKey,
	groups: map[string]ruleGroup{
		ruleEnabledKey: ruleEnabled,
		namespaceKey:   ruleMatch,
		"animation":    ruleAppearance, "blur": ruleAppearance, "blur_alpha_threshold": ruleAppearance, "opacity": ruleAppearance,
		"rounding": ruleAppearance, "shadow": ruleAppearance, "dim_around": ruleAppearance,
		"block_out_from": ruleBehaviour,
	},
	match: func(*wmBackend, []string) []cardField {
		return []cardField{{key: namespaceKey, label: wmKey(rulesLabels + "." + namespaceKey), fill: true, editor: func(k *kit, s slot) control {
			return newChoiceEditor(k, s, choiceSpec{list: layerChoices, free: true})
		}}}
	},
	title: func(item map[string]any, number int, _ installedApps) string {
		pattern, _ := item[namespaceKey].(string)
		if s, ok := layerOfPattern(pattern); ok {
			return layerLabel(s)
		}
		return wmCardTitle(item, number, []string{namespaceKey}, nil)
	},
}

// layerLabel names a surface lys-shell maps.
func layerLabel(s layers.Kind) string { return locale.T().Get("wm-layer-" + s.Name()) }

func layerOfPattern(pattern string) (layers.Kind, bool) {
	i := slices.IndexFunc(layers.All, func(s layers.Kind) bool { return s.Pattern() == pattern })
	if i < 0 {
		return nil, false
	}
	return layers.All[i], true
}

// layerChoices are lys-shell's surfaces, each as the pattern a layer rule
// matches it by.
func layerChoices() []choice {
	out := make([]choice, len(layers.All))
	for i, s := range layers.All {
		out[i] = choice{value: s.Pattern(), label: layerLabel(s)}
	}
	return out
}

// ruleAppChoices are the apps a rule can apply to: those installed, then
// those with a window open and no entry, each as the pattern matching its
// app ID exactly.
func ruleAppChoices(k *kit) []choice {
	ix := scanInstalled(desktopentry.ApplicationDirs())
	running, err := wmstate.RunningApps(k.wm.run)
	if err != nil && !errors.Is(err, wmstate.ErrNotRunning) {
		log.Printf("settings: the running apps for the rule editor: %v", err)
	}
	out := make([]choice, 0, len(ix.shown)+len(running))
	for _, a := range ix.shown {
		out = append(out, choice{value: exactPattern(a.AppID()), label: cmp.Or(a.DisplayName(), a.AppID())})
	}
	group := locale.T().Get("wm-rules-apps-running")
	for _, id := range running {
		if _, installed := ix.byID[id]; !installed {
			out = append(out, choice{value: exactPattern(id), label: id, group: group})
		}
	}
	return out
}

// ruleParts are the keys of a rule list's rules by the part of the
// editor they belong in, each part in the schema's order.
type ruleParts struct {
	match, more, effects []string
	group                map[string]ruleGroup
	toggle               cardToggle
}

func splitRuleKeys(w *wmBackend, kind ruleKind) ruleParts {
	item := []string{kind.path, wmconfig.Each}
	keys, ok := w.schema.Keys(item)
	if !ok {
		log.Panicf("settings: lys-wm's %s holds no rules", kind.path)
	}
	parts := ruleParts{group: kind.groups}
	switchable := false
	for _, key := range keys {
		g, ok := kind.groups[key]
		if !ok {
			log.Panicf("settings: lys-wm's %s.%s has no part in the rule editor", kind.path, key)
		}
		switch g {
		case ruleEnabled:
			parts.toggle, switchable = cardToggle{key: key, nameKey: g.labelKey()}, true
		case ruleMatch:
			parts.match = append(parts.match, key)
		case ruleMoreMatch:
			parts.more = append(parts.more, key)
		default:
			parts.effects = append(parts.effects, key)
		}
	}
	if !switchable {
		log.Panicf("settings: lys-wm's %s cannot be switched off", kind.path)
	}
	return parts
}

// wmRules is the row of a lys-wm list of rules: a card per rule, titled
// by what it applies to and showing its effects, its body what it applies
// to and then its effects.
func wmRules(kind ruleKind) rowSpec {
	return wmField(kind.path, func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newRuleList(k, s, kind) }
	})
}

func newRuleList(k *kit, s slot, kind ruleKind) *cardList {
	w := k.wm
	item := []string{kind.path, wmconfig.Each}
	parts := splitRuleKeys(w, kind)
	more := keyFields(w, item, parts.more)
	effects := keyFields(w, item, parts.effects)
	bools := map[string]bool{}
	for _, key := range parts.effects {
		f, _ := w.schema.Field(append(slices.Clone(item), key))
		bools[key] = f.Kind == wmconfig.Bool
	}
	var apps installedApps
	check := w.checker(kind.path)
	spec := cardSpec{
		check:  func(items []any) func() error { return check(items) },
		title:  func(it map[string]any, number int) string { return kind.title(it, number, apps) },
		aside:  func(k *kit) cardAside { return newRuleChips(k, parts) },
		toggle: &parts.toggle,
		fields: []cardField{
			{label: ruleMatch.labelKey(), fill: true, keys: append(slices.Clone(parts.match), parts.more...), item: func(k *kit, field func(key string) slot) control {
				return newRuleMatch(k, kind.match(w, item), parts.more, more, field)
			}},
			{label: "wm-rules-then", fill: true, keys: parts.effects, item: func(k *kit, field func(key string) slot) control {
				return newRuleEffects(k, parts, effects, bools, field)
			}},
		},
		blank:      func() map[string]any { return map[string]any{} },
		searchable: true,
		search:     ruleSearchText(parts),
		collapsed:  true,
	}
	if kind.icon != nil {
		spec.icon = func(it map[string]any) string { return kind.icon(it, apps) }
	}
	if kind.pick {
		spec.pick = rulePick(w)
	}
	list := newCardList(k, s, spec)
	if !kind.apps {
		return list
	}
	dirs := desktopentry.ApplicationDirs()
	async(k, func() installedApps { return scanInstalled(dirs) }, func(ix installedApps) {
		apps = ix
		list.rebuild()
	})
	return list
}

// keyFields are the card fields of each of keys of the object whose
// schema is at item.
func keyFields(w *wmBackend, item, keys []string) map[string][]cardField {
	out := make(map[string][]cardField, len(keys))
	for _, key := range keys {
		out[key] = wmKeyFields(w, item, nil, rulesLabels, key)
	}
	return out
}

// effectLabel names a key of a rule.
func effectLabel(key string) string { return locale.T().Get(wmKey(rulesLabels + "." + key)) }

// fieldRows lays out fields over the item whose key slots field gives,
// their labels sized alike, and answers the controls they hold.
func fieldRows(k *kit, box *widget.Box, fields []cardField, grid formGrid, field func(key string) slot) ([]control, []*widget.Box) {
	controls := make([]control, 0, len(fields))
	rows := make([]*widget.Box, 0, len(fields))
	for _, f := range fields {
		ctl := f.build(k, field)
		row := grid.row(grid.label(locale.T().Get(f.label)), ctl, f.fill)
		row.AddClass("card-form-row")
		box.Append(row, false)
		controls = append(controls, ctl)
		rows = append(rows, row)
	}
	return controls, rows
}

// ruleChips are a rule card's effects, as chips beside its title.
type ruleChips struct {
	*widget.FlowBox
	k     *kit
	parts ruleParts
}

func newRuleChips(k *kit, parts ruleParts) *ruleChips {
	c := &ruleChips{FlowBox: widget.NewWrapBox(choiceCardGap/2, choiceCardGap/2, widget.JustifyEnd), k: k, parts: parts}
	c.AddClass("rule-chips")
	return c
}

func (c *ruleChips) show(item map[string]any) {
	c.Clear()
	for _, key := range c.parts.effects {
		if item[key] != nil {
			c.Append(widget.NewChip(c.k.face, effectLabel(key)))
		}
	}
}

// ruleMatch is what a rule applies to: the keys it leads with, and the
// further conditions behind a toggle that starts open when one is set.
type ruleMatchEditor struct {
	*widget.Box
	controls []control
	more     *widget.Box
	toggle   *widget.Button
	label    *widget.Label
}

func newRuleMatch(k *kit, primary []cardField, moreKeys []string, more map[string][]cardField, field func(key string) slot) *ruleMatchEditor {
	m := &ruleMatchEditor{Box: gapBox(widget.Column)}
	m.AddClass("rule-match")
	grid := k.formGrid()
	m.controls, _ = fieldRows(k, m.Box, primary, grid, field)
	m.more = gapBox(widget.Column)
	open := false
	for _, key := range moreKeys {
		ctls, _ := fieldRows(k, m.more, more[key], grid, field)
		m.controls = append(m.controls, ctls...)
		open = open || field(key).get() != nil
	}
	m.label = k.label("")
	m.toggle = k.roleButton(roleQuiet, m.label, func() { m.open(!m.more.Visible()) })
	m.toggle.AddClass("rule-more-toggle")
	m.AppendAligned(m.toggle, false, widget.AlignStart)
	m.Append(m.more, false)
	m.open(open)
	return m
}

func (m *ruleMatchEditor) open(on bool) {
	m.more.SetVisible(on)
	key := ruleMoreMatch.labelKey()
	if on {
		key = "wm-rules-fewer-conditions"
	}
	m.label.SetText(locale.T().Get(key))
}

func (m *ruleMatchEditor) refresh() {
	for _, c := range m.controls {
		c.refresh()
	}
}

// ruleEffects is what a rule does: the effects it sets, and those added
// and not yet given a value, filed under their groups, each removable,
// and a searchable menu over the rest to add one.
type ruleEffects struct {
	*widget.Box
	k        *kit
	parts    ruleParts
	fields   map[string][]cardField
	bools    map[string]bool
	field    func(key string) slot
	added    []string
	shown    []string
	body     *widget.Box
	none     *widget.Label
	add      *widget.Button
	controls []control
}

func newRuleEffects(k *kit, parts ruleParts, fields map[string][]cardField, bools map[string]bool, field func(key string) slot) *ruleEffects {
	t := locale.T()
	r := &ruleEffects{Box: gapBox(widget.Column), k: k, parts: parts, fields: fields, bools: bools, field: field}
	r.AddClass("rule-effects")
	r.body = gapBox(widget.Column)
	r.none = k.note(t.Get("wm-rules-no-effects"))
	r.add = k.addButton("wm-rules-add-effect", func() {
		if k.pickers != nil {
			openChoicePicker(k, r.add, r.offered(), r.addEffect)
		}
	})
	r.Append(r.none, false)
	r.Append(r.body, false)
	r.AppendAligned(r.add, false, widget.AlignStart)
	r.rebuild()
	return r
}

// current are the effects the editor shows: those set, and those added.
func (r *ruleEffects) current() []string {
	var out []string
	for _, key := range r.parts.effects {
		if r.field(key).get() != nil || slices.Contains(r.added, key) {
			out = append(out, key)
		}
	}
	return out
}

func (r *ruleEffects) rebuild() {
	t := locale.T()
	r.body.Clear()
	r.controls = nil
	r.shown = r.current()
	grid := r.k.formGrid()
	for _, g := range ruleEffectGroups {
		var keys []string
		for _, key := range r.shown {
			if r.parts.group[key] == g {
				keys = append(keys, key)
			}
		}
		if len(keys) == 0 {
			continue
		}
		group := gapBox(widget.Column)
		group.AddClass("rule-effect-group")
		group.Append(r.k.label(t.Get(g.labelKey()), "rule-effect-group-title", css.Heading), false)
		for _, key := range keys {
			ctls, rows := fieldRows(r.k, group, r.fields[key], grid, r.field)
			r.controls = append(r.controls, ctls...)
			remove := r.k.removeButton("wm-rules-remove-effect", func() { r.removeEffect(key) })
			remove.SetAccessibleName(t.Get("wm-rules-remove-effect") + ": " + effectLabel(key))
			rows[0].AppendAligned(remove, false, widget.AlignCenter)
		}
		r.body.Append(group, false)
	}
	r.none.SetVisible(len(r.shown) == 0)
	r.add.SetVisible(len(r.shown) < len(r.parts.effects))
}

// offered are the effects the rule does not show yet, as the add menu's
// choices under their groups.
func (r *ruleEffects) offered() []choice {
	t := locale.T()
	var out []choice
	for _, g := range ruleEffectGroups {
		for _, key := range r.parts.effects {
			if r.parts.group[key] == g && !slices.Contains(r.shown, key) {
				out = append(out, choice{value: key, label: effectLabel(key), group: t.Get(g.labelKey())})
			}
		}
	}
	return out
}

// addEffect shows the effect key, an on/off one switched on, since adding
// it says it should apply.
func (r *ruleEffects) addEffect(key string) {
	if !slices.Contains(r.parts.effects, key) {
		log.Panicf("settings: %s is no effect of %v", key, r.parts.effects)
	}
	if !slices.Contains(r.added, key) {
		r.added = append(r.added, key)
	}
	if r.bools[key] && r.field(key).get() == nil {
		if err := r.field(key).set(true); err != nil {
			log.Printf("settings: add the effect %s: %v", key, err)
		}
	}
	r.rebuild()
}

func (r *ruleEffects) removeEffect(key string) {
	r.added = slices.DeleteFunc(r.added, func(a string) bool { return a == key })
	if r.field(key).get() != nil {
		r.field(key).unset()
	}
	r.rebuild()
}

func (r *ruleEffects) refresh() {
	if !slices.Equal(r.current(), r.shown) {
		r.rebuild()
		return
	}
	for _, c := range r.controls {
		c.refresh()
	}
}

func ruleSearchText(parts ruleParts) func(item map[string]any) string {
	return func(item map[string]any) string {
		words := []string{itemText(item)}
		for _, key := range parts.effects {
			if item[key] != nil {
				words = append(words, effectLabel(key))
			}
		}
		return strings.Join(words, " ")
	}
}
