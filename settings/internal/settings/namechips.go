package settings

import (
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

var skippedFolderExamples = []string{"Downloads", "Games", "Android", "snap", "go", "bin", "obj", "out", "coverage", "tmp"}

var fileExcludeRow rowOpt = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newNameChips(k, s, skippedFolderExamples) }
}

type nameChips struct {
	*widget.Box
	k        *kit
	slot     slot
	examples []string
	chips    *widget.FlowBox
	labels   []string
	toggles  []*widget.ToggleButton
	entry    *widget.Entry
	problem  *widget.Label
	syncing  bool
}

func newNameChips(k *kit, s slot, examples []string) *nameChips {
	t := locale.T()
	c := &nameChips{Box: gapBox(widget.Column), k: k, slot: s, examples: examples}
	c.chips = widget.NewWrapBox(choiceCardGap, choiceCardGap, widget.JustifyStart)
	c.entry = k.entry()
	c.entry.SetPlaceholder(t.Get("settings-folder-name-placeholder"))
	c.entry.SetAccessibleName(t.Get("settings-folder-name-add"))
	c.entry.OnActivate = func(string) { c.add() }
	c.problem = k.label("", "format-preview", css.Caption, css.Error)
	c.problem.SetVisible(false)
	add := gapBox(widget.Row)
	add.AppendAligned(c.entry, true, widget.AlignCenter)
	add.AppendAligned(k.iconButton("ld-plus-symbolic", "settings-folder-name-add", c.add, css.Outline), false, widget.AlignCenter)
	c.Append(c.chips, false)
	c.Append(add, false)
	c.Append(c.problem, false)
	c.refresh()
	return c
}

func (c *nameChips) names() []string {
	listed, _ := c.slot.get().([]any)
	out := make([]string, 0, len(listed))
	for _, v := range listed {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func (c *nameChips) write(names []string) {
	out := make([]any, len(names))
	for i, n := range names {
		out[i] = n
	}
	_ = c.slot.set(out)
}

func (c *nameChips) toggle(name string, on bool) {
	if c.syncing {
		return
	}
	names := c.names()
	switch has := slices.Contains(names, name); {
	case on && !has:
		names = append(names, name)
	case !on && has:
		names = slices.DeleteFunc(names, func(n string) bool { return n == name })
	default:
		return
	}
	c.write(names)
	c.refresh()
}

func folderNameProblem(name string, listed []string) string {
	switch {
	case name == "":
		return "settings-folder-name-empty"
	case strings.ContainsRune(name, '/'):
		return "settings-folder-name-slash"
	case slices.Contains(listed, name):
		return "settings-folder-name-listed"
	}
	return ""
}

func (c *nameChips) add() {
	name := strings.TrimSpace(c.entry.Text())
	names := c.names()
	if key := folderNameProblem(name, names); key != "" {
		c.problem.SetText(locale.T().Get(key))
		c.problem.SetVisible(true)
		return
	}
	c.problem.SetVisible(false)
	c.write(append(names, name))
	c.entry.SetText("")
	c.refresh()
}

func (c *nameChips) shown() []string {
	out := slices.Clone(c.examples)
	for _, n := range c.names() {
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

func (c *nameChips) refresh() {
	names := c.names()
	shown := c.shown()
	c.syncing = true
	defer func() { c.syncing = false }()
	if !slices.Equal(c.labels, shown) {
		c.chips.Clear()
		c.labels, c.toggles = shown, make([]*widget.ToggleButton, len(shown))
		for i, name := range shown {
			chip := widget.NewToggleButton(c.k.label(name), widget.Current().Padding, chipRadius)
			chip.AddClass(css.Small)
			chip.OnToggled = func(on bool) { c.toggle(name, on) }
			c.toggles[i] = chip
			c.chips.Append(chip)
		}
	}
	for i, name := range c.labels {
		c.toggles[i].SetActive(slices.Contains(names, name))
	}
}
