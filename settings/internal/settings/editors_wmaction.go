package settings

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

const wmArgType = "WmArg"

// actionForm is how an action's arguments are edited.
type actionForm int

const (
	formNone actionForm = iota
	formFields
	formText
	formOutside
)

// wmActionEditor edits a lys-wm action: the action picked from lys-wm's
// list, its arguments by the form lys-wm describes for it, and as text
// where the arguments are outside that form or it has none the editor
// builds.
type wmActionEditor struct {
	*widget.Box
	k        *kit
	slot     slot
	names    bindNames
	grammars map[string]argGrammar
	pick     *widget.Button
	picked   *widget.Label
	outside  *widget.Badge
	args     *widget.Box
	controls []control
	name     string
	form     actionForm
	values   argValues
	shape    string
	written  string
}

func newWMActionEditor(k *kit, s slot, actions []wmconfig.Action) *wmActionEditor {
	t := locale.T()
	e := &wmActionEditor{Box: gapBox(widget.Column), k: k, slot: s, names: bindNames{actions: actions}, grammars: map[string]argGrammar{}}
	for _, a := range actions {
		if a.Args == "" {
			e.grammars[a.Name] = argGrammar{}
			continue
		}
		if g, err := parseArgGrammar(a.Args); err == nil {
			e.grammars[a.Name] = g
		}
	}
	e.picked = k.label("", "action-picked")
	e.picked.SetEllipsize(widget.EllipsizeEnd)
	content := gapBox(widget.Row)
	content.AppendAligned(e.picked, true, widget.AlignCenter)
	content.AppendAligned(k.icon("ld-chevron-down-symbolic", "action-pick-icon"), false, widget.AlignCenter)
	e.pick = k.button(content, e.open, css.Outline)
	e.pick.SetAccessibleName(t.Get("wm-action-pick"))
	e.pick.SetTooltip(t.Get("wm-action-pick"))
	e.outside = k.badge(t.Get("wm-action-args-outside"), "warning")
	e.outside.SetTooltip(t.Attr("wm-action-args-outside", "description"))
	head := gapBox(widget.Row)
	head.AppendAligned(e.pick, true, widget.AlignCenter)
	head.AppendAligned(e.outside, false, widget.AlignCenter)
	e.args = gapBox(widget.Column)
	e.Append(head, false)
	e.Append(e.args, false)
	e.refresh()
	return e
}

func (e *wmActionEditor) open() {
	if e.k.pickers == nil {
		return
	}
	openChoicePicker(e.k, e.pick, e.choices(), e.choose)
}

// choices are lys-wm's actions by group, each read as what it does and
// found by its name too.
func (e *wmActionEditor) choices() []choice {
	type entry struct {
		choice
		category bindCategory
	}
	entries := make([]entry, 0, len(e.names.actions))
	for _, a := range e.names.actions {
		label, _ := e.names.actionLabel(a.Name)
		entries = append(entries, entry{choice{value: a.Name, label: label, group: a.Name}, e.names.category(map[string]any{actionKey: a.Name})})
	}
	slices.SortStableFunc(entries, func(x, y entry) int {
		return cmp.Or(cmp.Compare(x.category, y.category), strings.Compare(x.label, y.label))
	})
	out := make([]choice, len(entries))
	for i, en := range entries {
		out[i] = en.choice
	}
	return out
}

// choose takes the picked action with the arguments its form starts with.
func (e *wmActionEditor) choose(name string) {
	if name == e.name {
		return
	}
	text := name
	if g, ok := e.grammars[name]; ok {
		text = strings.TrimSpace(name + " " + g.format(g.blank()))
	}
	e.write(text)
}

func (e *wmActionEditor) write(text string) {
	e.written = text
	_ = e.slot.set(text)
	e.refresh()
}

func (e *wmActionEditor) refresh() {
	text := e.slot.text()
	a := splitAction(text)
	if text != e.written || a.name != e.name || e.form != formFields {
		e.parse(a)
	}
	e.written = text
	label, _ := e.names.actionLabel(text)
	if a.name == "" {
		label = locale.T().Get("wm-action-pick")
	}
	e.picked.SetText(label)
	e.outside.SetVisible(e.form == formOutside)
	if shape := e.shapeKey(); shape != e.shape {
		e.shape = shape
		e.build()
		return
	}
	for _, c := range e.controls {
		c.refresh()
	}
}

func (e *wmActionEditor) parse(a boundAction) {
	e.name, e.values = a.name, nil
	g, ok := e.grammars[a.name]
	switch {
	case a.name == "":
		e.form = formNone
	case !e.names.known(a.name):
		e.form = formText
	case !ok:
		e.form = formText
	case len(g) == 0 && len(a.args) == 0:
		e.form = formNone
	default:
		values, matched := g.match(a.argText())
		e.form, e.values = formFields, values
		if !matched {
			e.form, e.values = formOutside, nil
		}
	}
}

func (e *wmActionEditor) shapeKey() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s|%d", e.name, e.form)
	for _, v := range e.values {
		fmt.Fprintf(&b, "|%d", v.alt)
	}
	return b.String()
}

func (e *wmActionEditor) build() {
	e.args.Clear()
	e.controls = nil
	switch e.form {
	case formText, formOutside:
		e.add(nil, newText(e.k, e.argsText(), false))
	case formFields:
		g := e.grammars[e.name]
		for i, s := range g {
			e.buildSlot(i, s)
		}
	}
	e.args.SetVisible(len(e.controls) > 0)
}

func (e *wmActionEditor) add(lead widget.Widget, controls ...control) {
	row := gapBox(widget.Row)
	if lead != nil {
		row.AppendAligned(lead, false, widget.AlignCenter)
	}
	for _, c := range controls {
		row.AppendAligned(c, true, widget.AlignCenter)
		e.controls = append(e.controls, c)
	}
	e.args.Append(row, false)
}

// argsText is the arguments as one text, the action's name kept.
func (e *wmActionEditor) argsText() slot {
	return slot{
		get: func() any { return splitAction(e.slot.text()).argText() },
		set: func(v any) error {
			s, _ := v.(string)
			e.write(strings.TrimSpace(e.name + " " + strings.Join(strings.Fields(s), " ")))
			return nil
		},
		unset: func() { e.write(e.name) },
	}
}

func (e *wmActionEditor) store() {
	g := e.grammars[e.name]
	e.write(strings.TrimSpace(e.name + " " + g.format(e.values)))
}

func argMeta(variants []string) config.FieldMeta {
	return config.FieldMeta{Kind: config.FieldEnum, Variants: variants, Type: wmArgType}
}

func argLabel(key string) string {
	return enumLabels(argMeta([]string{key}))[0]
}

func (e *wmActionEditor) choiceControl(variants []string, s slot) control {
	meta := argMeta(variants)
	if fitsSegments(enumLabels(meta)) {
		return newSegmentedEnum(e.k, s, meta)
	}
	return newEnumSelect(e.k, s, meta)
}

func (e *wmActionEditor) buildSlot(i int, s argSlot) {
	if lits, ok := s.literals(); ok && s.optional && len(lits) == 1 {
		on := slot{
			get: func() any { return e.values[i].alt == 0 },
			set: func(v any) error {
				e.values[i] = slotValue{alt: -1}
				if on, _ := v.(bool); on {
					e.values[i] = s.choose(0)
				}
				e.store()
				return nil
			},
			unset: func() {},
		}
		e.add(e.k.label(argLabel(lits[0]), "action-arg-label"), newToggle(e.k, on))
		return
	}
	keys := s.altKeys()
	pick := slot{
		get: func() any {
			if e.values[i].alt < 0 {
				return argOmitted
			}
			return argAltKey(s.alts[e.values[i].alt])
		},
		set: func(v any) error {
			key, _ := v.(string)
			e.values[i] = slotValue{alt: -1}
			if j := slices.IndexFunc(s.alts, func(a argAlt) bool { return argAltKey(a) == key }); j >= 0 {
				e.values[i] = s.choose(j)
			}
			e.store()
			return nil
		},
		unset: func() {},
	}
	var controls []control
	if len(keys) > 1 {
		controls = append(controls, e.choiceControl(keys, pick))
	}
	if alt := e.values[i].alt; alt >= 0 {
		for j, w := range s.alts[alt] {
			for h, hole := range w.holes() {
				controls = append(controls, e.holeControl(i, j, h, w, hole))
			}
		}
	}
	if len(controls) > 0 {
		e.add(nil, controls...)
	}
}

func (e *wmActionEditor) holeSlot(i, j, h int, w argWord, parse func(any) string, read func(string) any) slot {
	return slot{
		get: func() any { return read(e.values[i].words[j].holes[h]) },
		set: func(v any) error {
			word := &e.values[i].words[j]
			word.holes[h] = parse(v)
			word.present = !w.optional || word.holes[h] != ""
			e.store()
			return nil
		},
		unset: func() {
			word := &e.values[i].words[j]
			word.holes[h] = ""
			word.present = !w.optional
			e.store()
		},
	}
}

func asText(v any) string { s, _ := v.(string); return strings.TrimSpace(s) }

// holeBounds are the spin bounds of a numeric hole: lys-wm states none, so
// they span what a bind sensibly sets.
var holeBounds = map[holeKind]struct {
	lo, hi, step float64
	digits       int
}{
	holeWhole:    {0, 100_000, 1, 0},
	holeSigned:   {-10_000, 10_000, 1, 0},
	holeFraction: {0, 16, 0.1, 2},
}

func (e *wmActionEditor) holeControl(i, j, h int, w argWord, hole argHole) control {
	kind := holeKinds[hole]
	if b, numeric := holeBounds[kind]; numeric {
		whole := b.digits == 0
		s := e.holeSlot(i, j, h, w, func(v any) string {
			switch n := v.(type) {
			case int64:
				return strconv.FormatInt(n, 10)
			case float64:
				return strconv.FormatFloat(n, 'f', -1, 64)
			}
			return ""
		}, func(text string) any {
			f, _ := strconv.ParseFloat(text, 64)
			if whole {
				return int64(f)
			}
			return f
		})
		return newNumber(e.k, s, b.lo, b.hi, b.step, b.digits)
	}
	s := e.holeSlot(i, j, h, w, asText, func(text string) any { return text })
	if hole == holeCommand {
		return newChoiceEditor(e.k, s, choiceSpec{list: fixed(), free: true, browse: (*kit).appCommandButton})
	}
	c := newText(e.k, s, w.optional)
	c.SetPlaceholder(argLabel(string(hole)))
	return c
}
