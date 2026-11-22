package settings

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/spawn"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const customModulesPath = "modules.custom"

var (
	customForm      = formOf(config.DefaultsCustomModuleDefinition())
	customColorKeys = []string{"icon-color", "label-color", "icon-bg-color", "button-bg-color", "border-color"}
	customCards     = customCardSpec()
)

func listOf(spec func() listSpec) func(k *kit, s slot, _ config.FieldMeta) control {
	return func(k *kit, s slot, _ config.FieldMeta) control { return newListEditor(k, s, spec()) }
}

func customCardSpec() cardSpec {
	f := customForm
	colorLabels := make([]string, len(customColorKeys))
	for i, key := range customColorKeys {
		colorLabels[i] = f.label(key)
	}
	return cardSpec{
		identity: f.identity("id"),
		fields: []cardField{
			filled(f.field("command")),
			f.group("settings-custom-run", []string{"command"}, func(k *kit, slots []slot) control { return newCommandRun(k, slots[0], "settings-custom-run-empty") }),
			f.field("mode"),
			f.field("interval-ms"),
			f.field("restart-policy"),
			f.field("restart-interval-ms"),
			filled(f.field("format")),
			filled(f.field("tooltip-format")),
			f.field("hide-if-empty"),
			f.with("icon-name", iconEditorOf),
			filled(f.with("icon-names", listOf(iconListSpec))),
			filled(f.with("icon-map", listOf(iconMapSpec))),
			f.field("icon-show"),
			f.field("label-show"),
			f.field("label-max-length"),
			f.field("border-show"),
			filled(f.group("settings-bar-item-colors", customColorKeys, func(k *kit, slots []slot) control {
				return newColorStrip(k, slots, colorLabels)
			})),
			filled(f.group("settings-bar-item-mouse", mouseButtons, func(k *kit, slots []slot) control {
				return newMouseActions(k, slots, nil)
			})),
			filled(f.field("on-action")),
			filled(f.field("class-format")),
			f.group("settings-custom-bar", []string{"id"}, func(k *kit, slots []slot) control {
				return newPlacementRow(k, func() (config.BarModule, bool) {
					id := slots[0].text()
					return config.CustomModule(id), id != ""
				})
			}),
		},
		blank:      func() map[string]any { return map[string]any{"id": ""} },
		keep:       keyed("id"),
		unique:     "id",
		addKey:     "settings-custom-add",
		searchable: true,
		collapsed:  true,
		listCls:    "card-form-list", rowClass: "card-form-row", labelCls: "card-form-label",
	}
}

func beyondCards(v any, edited []string) []string {
	items, _ := v.([]any)
	beyond := map[string]bool{}
	for _, it := range items {
		m, _ := it.(map[string]any)
		for key, value := range m {
			if value != nil && !slices.Contains(edited, key) {
				beyond[key] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(beyond))
}

type customModulesEditor struct {
	*widget.Box
	slot    slot
	edited  []string
	cards   *cardList
	notice  *widget.Label
	raw     *yamlEditor
	toggle  *widget.Button
	toggled *widget.Label
	editing bool
}

var customModulesRow rowOpt = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newCustomModulesEditor(k, s) }
}

func newCustomModulesEditor(k *kit, s slot) *customModulesEditor {
	e := &customModulesEditor{Box: gapBox(widget.Column), slot: s, edited: customCards.editedKeys()}
	e.cards = newCardList(k, s, customCards)
	e.notice = k.note("")
	e.raw = newYAMLEditor(k, s, "custom")
	e.toggled = k.label("")
	e.toggle = k.roleButton(roleQuiet, e.toggled, func() {
		e.editing = !e.editing
		e.refresh()
	})
	e.Append(e.notice, false)
	e.Append(e.cards, false)
	e.AppendAligned(e.toggle, false, widget.AlignStart)
	e.Append(e.raw, false)
	e.refresh()
	return e
}

func (e *customModulesEditor) dirtyBadge() *widget.Badge { return e.raw.dirtyBadge() }

func (e *customModulesEditor) refresh() {
	t := locale.T()
	beyond := beyondCards(e.slot.get(), e.edited)
	e.notice.SetVisible(len(beyond) > 0)
	e.notice.SetText(t.Get("settings-custom-handwritten", i18n.Str("keys", strings.Join(beyond, ", "))))
	e.toggle.SetVisible(len(beyond) == 0)
	e.raw.SetVisible(len(beyond) > 0 || e.editing)
	key := "settings-format-edit-text"
	if e.editing {
		key = "settings-format-edit-parts"
	}
	e.toggled.SetText(t.Get(key))
	e.cards.refresh()
	e.raw.refresh()
}

type commandOutcome struct {
	output string
	err    error
}

func runCustomCommand(command string) commandOutcome {
	ctx, cancel := context.WithTimeout(context.Background(), config.CustomCommandTimeout)
	defer cancel()
	cmd := spawn.Owned(ctx, command)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return commandOutcome{output: strings.TrimSpace(out.String()), err: fmt.Errorf("run %q: %w", command, err)}
	}
	return commandOutcome{output: strings.TrimSpace(out.String())}
}

type commandRun struct {
	*widget.Box
	k        *kit
	command  slot
	emptyKey string
	output   *widget.Label
	runs     int
}

func newCommandRun(k *kit, command slot, emptyKey string) *commandRun {
	c := &commandRun{Box: gapBox(widget.Column), k: k, command: command, emptyKey: emptyKey}
	c.output = k.note("", "format-preview")
	c.output.SetVisible(false)
	c.AppendAligned(k.secondaryButton(locale.T().Get("settings-custom-run-once"), c.run), false, widget.AlignStart)
	c.Append(c.output, false)
	return c
}

func (c *commandRun) run() {
	t := locale.T()
	c.output.SetVisible(true)
	command := c.command.text()
	if command == "" {
		c.output.SetText(t.Get("settings-custom-run-no-command"))
		return
	}
	c.runs++
	asked := c.runs
	c.output.SetText(t.Get("settings-custom-running"))
	async(c.k, func() commandOutcome { return runCustomCommand(command) }, func(o commandOutcome) {
		if asked != c.runs {
			return
		}
		c.output.SetText(o.message(c.emptyKey))
	})
}

func (o commandOutcome) message(emptyKey string) string {
	t := locale.T()
	switch {
	case o.err != nil:
		return t.Get("settings-custom-run-failed", i18n.Str("reason", o.err.Error()))
	case o.output == "":
		return t.Get(emptyKey)
	}
	return t.Get("settings-custom-run-output", i18n.Str("output", o.output))
}

func (c *commandRun) refresh() {}
