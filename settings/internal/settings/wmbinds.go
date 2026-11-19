package settings

import (
	"log"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

const (
	actionKey      = "action"
	descriptionKey = "description"
	execAction     = "exec"
	// emptyIsolate is what an empty argument reads as in a message.
	emptyIsolate = "\u2068\u2069"
	// argOmitted stands for the arguments an action is given without: the
	// first one its label selects on, and an optional slot left out.
	argOmitted = "default"
)

// boundAction is a bind's action split into lys-wm's action name and its
// arguments.
type boundAction struct {
	name string
	args []string
}

func splitAction(text string) boundAction {
	words := strings.Fields(text)
	if len(words) == 0 {
		return boundAction{}
	}
	return boundAction{name: words[0], args: words[1:]}
}

func (a boundAction) argText() string { return strings.Join(a.args, " ") }

// bindNames names binds by what they do: lys-wm's actions, and the
// installed apps by the command that launches each.
type bindNames struct {
	actions  []wmconfig.Action
	launches map[string]string
}

func (n bindNames) known(name string) bool {
	return slices.ContainsFunc(n.actions, func(a wmconfig.Action) bool { return a.Name == name })
}

// actionLabel is what the action text does, and whether lys-wm knows its
// action; an unknown one reads as written.
func (n bindNames) actionLabel(text string) (string, bool) {
	a := splitAction(text)
	if !n.known(a.name) {
		return strings.TrimSpace(text), false
	}
	t := locale.T()
	if app, ok := n.launches[a.argText()]; ok && a.name == execAction {
		return t.Get("wm-action-launch", i18n.Str("app", app)), true
	}
	key := "wm-action-" + a.name
	if !t.Has(key) {
		log.Panicf("settings: lys-wm's action %s has no label", a.name)
	}
	first, rest := argOmitted, ""
	if len(a.args) > 0 {
		first, rest = a.args[0], strings.Join(a.args[1:], " ")
	}
	label := t.Get(key, i18n.Str("first", first), i18n.Str("args", a.argText()), i18n.Str("rest", rest))
	return strings.TrimSpace(strings.ReplaceAll(label, emptyIsolate, "")), true
}

// bindLabel titles a bind: its own description, else what its action
// does, else its number.
func (n bindNames) bindLabel(item map[string]any, number int) string {
	if d, _ := item[descriptionKey].(string); strings.TrimSpace(d) != "" {
		return d
	}
	if text, _ := item[actionKey].(string); strings.TrimSpace(text) != "" {
		label, _ := n.actionLabel(text)
		return label
	}
	return locale.T().Get("wm-card-number", i18n.Int("number", int64(number)))
}

func (n bindNames) unknown(item map[string]any) bool {
	text, _ := item[actionKey].(string)
	return strings.TrimSpace(text) != "" && !n.known(splitAction(text).name)
}

const (
	bindsKey   = "binds"
	submapsKey = "submaps"
)

// bindOwners are the chords of binds, each held by what its bind does,
// in mode when they are a submap's.
func bindOwners(items any, names *bindNames, mode string) []clashOwner {
	list, _ := items.([]any)
	out := make([]clashOwner, 0, len(list))
	for i, it := range list {
		m, _ := it.(map[string]any)
		chord, _ := m[chordKey].(string)
		if strings.TrimSpace(chord) == "" {
			continue
		}
		owner := names.bindLabel(m, i+1)
		if mode != "" {
			owner = locale.T().Get("wm-binds-clash-mode", i18n.Str("label", owner), i18n.Str("mode", mode))
		}
		out = append(out, clashOwner{value: chord, owner: owner})
	}
	return out
}

// submapOwners are the chords every submap binds, the submaps in name
// order.
func submapOwners(w *wmBackend, names *bindNames) []clashOwner {
	submaps, _ := w.value(submapsKey).(map[string]any)
	var out []clashOwner
	for _, mode := range slices.Sorted(maps.Keys(submaps)) {
		out = append(out, bindOwners(submaps[mode], names, mode)...)
	}
	return out
}

// normalBind is a bind compared by meaning: its chord as the clash check
// reads it and its action's words, the rest as written.
func normalBind(m map[string]any) map[string]any {
	out := maps.Clone(m)
	out[chordKey] = bindChord(m)
	out[actionKey] = bindAction(m)
	return out
}

func bindChord(m map[string]any) string {
	chord, _ := m[chordKey].(string)
	return normalChord(chord)
}

// bindDefault is the built-in bind that bind i of items goes back to: the
// one it repeats with other flags or description, else the one doing its
// action on a chord no bind holds, else the one on its chord. A bind that
// is built in, or has none to go back to, has no default.
func bindDefault(items []map[string]any, i int, defaults []any) (map[string]any, bool) {
	own := normalBind(items[i])
	builtIn := make([]map[string]any, 0, len(defaults))
	for _, d := range defaults {
		if m, ok := d.(map[string]any); ok {
			builtIn = append(builtIn, m)
		}
	}
	if slices.ContainsFunc(builtIn, func(d map[string]any) bool { return reflect.DeepEqual(normalBind(d), own) }) {
		return nil, false
	}
	held := make(map[string]bool, len(items))
	for _, m := range items {
		held[bindChord(m)] = true
	}
	sameChord := func(d map[string]any) bool { return bindChord(d) == bindChord(items[i]) }
	sameAction := func(d map[string]any) bool { return bindAction(d) == bindAction(items[i]) }
	for _, match := range []func(d map[string]any) bool{
		func(d map[string]any) bool { return sameChord(d) && sameAction(d) },
		func(d map[string]any) bool { return sameAction(d) && !held[bindChord(d)] },
		sameChord,
	} {
		if j := slices.IndexFunc(builtIn, match); j >= 0 {
			return maps.Clone(builtIn[j]), true
		}
	}
	return nil, false
}

// clashWith is the first of others holding chord, by the clash check the
// card lists use.
func clashWith(chord string, others []clashOwner) (clashOwner, bool) {
	if strings.TrimSpace(chord) == "" {
		return clashOwner{}, false
	}
	values := make([]string, 0, len(others)+1)
	values = append(values, chord)
	for _, o := range others {
		values = append(values, o.value)
	}
	p := chordClashes(values, normalChord)[0]
	if p < 1 {
		return clashOwner{}, false
	}
	return others[p-1], true
}

// launchNames are the installed apps' names by the command each launches.
func launchNames(ix installedApps) map[string]string {
	out := make(map[string]string, len(ix.shown))
	for _, a := range ix.shown {
		cmd, err := appCommand(a)
		if err != nil {
			log.Printf("settings: %v", err)
			continue
		}
		if _, taken := out[cmd]; !taken {
			out[cmd] = a.DisplayName()
		}
	}
	return out
}

// chordKeys is a chord as the keycaps show it.
func chordKeys(chord string) string {
	if c, ok := parseChord(chord); ok {
		return c.accel()
	}
	return strings.Join(strings.Fields(chord), "")
}

// bindAside is a bind card's header beside its title: the unknown-action
// mark and the chord's keycaps.
type bindAside struct {
	*widget.Box
	k       *kit
	names   *bindNames
	unknown *widget.Badge
	keys    *widget.Box
	shown   string
}

func newBindAside(k *kit, names *bindNames) cardAside {
	t := locale.T()
	a := &bindAside{Box: gapBox(widget.Row), k: k, names: names, keys: gapBox(widget.Row)}
	a.unknown = k.badge(t.Get("wm-binds-unknown-action"), "warning")
	a.unknown.SetTooltip(t.Attr("wm-binds-unknown-action", "description"))
	a.AppendAligned(a.unknown, false, widget.AlignCenter)
	a.AppendAligned(a.keys, false, widget.AlignCenter)
	return a
}

func (a *bindAside) show(item map[string]any) {
	a.unknown.SetVisible(a.names.unknown(item))
	chord, _ := item[chordKey].(string)
	keys := ""
	if strings.TrimSpace(chord) != "" {
		keys = chordKeys(chord)
	}
	if keys == a.shown {
		return
	}
	a.shown = keys
	a.keys.Clear()
	if keys != "" {
		a.keys.AppendAligned(newKeycaps(a.k, keys), false, widget.AlignCenter)
	}
}

// keycaps is a chord drawn as lys-ui's keycaps: one picture, its keys set
// apart by the keycaps' own spacing.
type keycaps struct{ *widget.Box }

func newKeycaps(k *kit, keys string) keycaps {
	label := widget.NewShortcutLabel(k.face, keys)
	label.AddClass("bind-keys", css.DimLabel)
	return keycaps{label}
}
