package settings

import (
	"fmt"
	"maps"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// barState is a real state a bar item preview shows, named for the chip
// that picks it.
type barState struct {
	name  string
	state barview.State
}

// barStates reads the states a module's preview steps through under cfg.
type barStates func(k *kit, cfg *config.Config) ([]barState, error)

// steppedBarItemSection is barItemSection with a preview stepping
// through the states read gives.
func steppedBarItemSection(module string, choices []actionChoice, read barStates) sectionSpec {
	s := barItemSection(module, choices)
	m := moduleOfSection(module)
	s.preview = func(k *kit) widget.Widget { return newSteppedPreview(k, m, read) }
	return s
}

// steppedPreview is a module's bar item preview in one of its real
// states: a chip per state picks the one shown, and a press on the
// module's own chip shows the next, as the bar steps through them.
type steppedPreview struct {
	*widget.Box
	k       *kit
	module  config.BarModule
	read    barStates
	states  []barState
	at      int
	shown   *livePreview
	chips   *widget.ChoiceChips
	problem *widget.Label
}

func newSteppedPreview(k *kit, module config.BarModule, read barStates) *steppedPreview {
	p := &steppedPreview{Box: gapBox(widget.Column), k: k, module: module, read: read}
	p.chips = newChipGroup(k, widget.ChooseOne, nil)
	p.chips.OnChanged = func(i int, _ bool) { p.show(i) }
	p.problem = k.errorText("")
	p.problem.SetVisible(false)
	p.readStates()
	p.shown = newLivePreview(k, "settings-bar-item-preview", p.draw)
	p.Append(p.shown, false)
	p.Append(p.chips, false)
	p.Append(p.problem, false)
	return p
}

func (p *steppedPreview) draw(_ *kit, cfg *config.Config, font render.Font) widget.Widget {
	var states []barview.State
	if p.at < len(p.states) {
		states = append(states, p.states[p.at].state.OnPress(p.next))
	}
	return barview.SampleBar(cfg, font, config.BarLayout{Monitor: barview.AllMonitors, Show: true, Center: []config.BarItem{{Module: p.module}}}, sessionCompositor(), states...)
}

func (p *steppedPreview) readStates() {
	held := ""
	if p.at < len(p.states) {
		held = p.states[p.at].name
	}
	states, err := p.read(p.k, p.k.store.svc.Config())
	p.problem.SetVisible(err != nil)
	if err != nil {
		p.problem.SetText(locale.T().Get("settings-bar-states-unread", i18n.Str("reason", err.Error())))
	}
	p.states, p.at = states, max(0, slices.IndexFunc(states, func(s barState) bool { return s.name == held }))
	names := make([]string, len(states))
	for i, s := range states {
		names[i] = s.name
	}
	setChips(p.k, p.chips, names)
	p.chips.SetVisible(len(states) > 1)
	p.mark()
}

func (p *steppedPreview) mark() {
	markChips(p.chips, func(i int) bool { return i == p.at })
}

func (p *steppedPreview) show(i int) {
	p.at = i
	p.mark()
	p.shown.configChanged()
}

func (p *steppedPreview) next() { p.show((p.at + 1) % len(p.states)) }

func (p *steppedPreview) configChanged() {
	p.readStates()
	p.shown.configChanged()
}

// submapStates are keybind-mode in the default submap and in each of
// lys-wm's submaps, by name.
func submapStates(k *kit, _ *config.Config) ([]barState, error) {
	if !k.parts.wm {
		return nil, nil
	}
	w, err := k.wmBackend()
	if err != nil {
		return nil, fmt.Errorf("read lys-wm's submaps: %w", err)
	}
	submaps, _ := w.value(submapsKey).(map[string]any)
	out := []barState{{name: locale.T().Get("settings-keybind-mode-default"), state: barview.KeybindModeState("")}}
	for _, name := range slices.Sorted(maps.Keys(submaps)) {
		out = append(out, barState{name: name, state: barview.KeybindModeState(name)})
	}
	return out, nil
}

// layoutStates are keyboard-input on each of lys-wm's keyboard layouts in
// their order, by the name the compositor reports, under cfg's aliases.
func layoutStates(k *kit, cfg *config.Config) ([]barState, error) {
	if !k.parts.wm {
		return nil, nil
	}
	w, err := k.wmBackend()
	if err != nil {
		return nil, fmt.Errorf("read lys-wm's keyboard layouts: %w", err)
	}
	names := systemLists[listLayoutNames]()
	codes := layoutCodes(keyboardNames{layout: wmText(w, wmLayoutPath), variant: wmText(w, wmVariantPath)})
	out := make([]barState, 0, len(codes))
	for _, code := range codes {
		i := slices.IndexFunc(names, func(c choice) bool { return c.group == code })
		if i < 0 {
			return out, fmt.Errorf("keyboard layout %s is not in the xkb registry", code)
		}
		out = append(out, barState{name: names[i].value, state: barview.KeyboardInputState(names[i].value, cfg.KeyboardInput.LayoutAliasMap)})
	}
	return out, nil
}
