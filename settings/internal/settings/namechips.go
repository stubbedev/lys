package settings

import (
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

var skippedFolderExamples = []string{"Downloads", "Games", "Android", "snap", "go", "bin", "obj", "out", "coverage", "tmp"}

// nameChipsSpec shapes a list of names edited as chips: the names offered,
// the work reading them off the loop, and an entry adding any other, refused
// for the message key problem names.
type nameChipsSpec struct {
	offered     func(k *kit) func() offeredNames
	placeholder string
	addKey      string
	problem     func(name string, listed []string) string
}

// offeredNames are the names a chips list offers, or why they cannot be
// read.
type offeredNames struct {
	names []string
	err   error
}

var fileExcludeRow = nameChipsRow(nameChipsSpec{
	offered: func(*kit) func() offeredNames {
		return func() offeredNames { return offeredNames{names: skippedFolderExamples} }
	},
	placeholder: "settings-folder-name-placeholder",
	addKey:      "settings-folder-name-add",
	problem:     folderNameProblem,
})

func nameChipsRow(spec nameChipsSpec) rowOpt {
	return func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newNameChips(k, s, spec) }
	}
}

type nameChips struct {
	*widget.Box
	k       *kit
	slot    slot
	spec    nameChipsSpec
	offered []string
	chips   *widget.ChoiceChips
	labels  []string
	entry   *widget.Entry
	problem *widget.Label
	unread  *widget.Label
}

func newNameChips(k *kit, s slot, spec nameChipsSpec) *nameChips {
	t := locale.T()
	c := &nameChips{Box: gapBox(widget.Column), k: k, slot: s, spec: spec}
	c.chips = newChipGroup(k, widget.ChooseMany, nil)
	c.chips.OnChanged = func(i int, on bool) { c.toggle(c.labels[i], on) }
	c.entry = k.entry()
	c.entry.SetPlaceholder(t.Get(spec.placeholder))
	c.entry.SetAccessibleName(t.Get(spec.addKey))
	c.entry.OnActivate = func(string) { c.add() }
	c.problem = k.label("", "format-preview", css.Caption, css.Error)
	c.problem.SetVisible(false)
	c.unread = wrapped(k.label("", "setting-description", css.Caption, css.DimLabel))
	c.unread.SetVisible(false)
	add := gapBox(widget.Row)
	add.AppendAligned(c.entry, true, widget.AlignCenter)
	add.AppendAligned(k.addButton(spec.addKey, c.add), false, widget.AlignCenter)
	c.Append(c.chips, false)
	c.Append(c.unread, false)
	c.Append(add, false)
	c.Append(c.problem, false)
	c.refresh()
	async(k, spec.offered(k), func(o offeredNames) {
		if o.err != nil {
			c.unread.SetText(locale.T().Get("settings-names-unread", i18n.Str("reason", o.err.Error())))
			c.unread.SetVisible(true)
		}
		c.offered = o.names
		c.refresh()
	})
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
	if key := c.spec.problem(name, names); key != "" {
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
	out := slices.Clone(c.offered)
	for _, n := range c.names() {
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

func (c *nameChips) refresh() {
	names := c.names()
	if shown := c.shown(); !slices.Equal(c.labels, shown) {
		c.labels = shown
		setChips(c.k, c.chips, shown)
	}
	markChips(c.chips, func(i int) bool { return slices.Contains(names, c.labels[i]) })
}
