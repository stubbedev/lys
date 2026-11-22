package settings

import (
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// modeToggle switches an editor between two views of one value, its
// parts and its text, or a form's fields and the rest of its keys: a
// link reading off while off and on while on. The stylesheet takes its
// padding across away, so its text starts where the editor's content
// does, at the row's inset.
type modeToggle struct {
	*widget.Button
	label   *widget.Label
	off, on string
	active  bool
	flip    func(on bool)
}

// modeToggle is a toggle reading off, that runs flip with the new state
// on every click.
func (k *kit) modeToggle(off, on string, flip func(on bool)) *modeToggle {
	m := &modeToggle{label: k.label(off), off: off, on: on, flip: flip}
	m.Button = k.roleButton(roleLink, m.label, func() { m.set(!m.active) })
	m.AddClass("mode-toggle")
	return m
}

// textModeToggle is the toggle between a builder's parts and its value
// as text.
func (k *kit) textModeToggle(flip func(on bool)) *modeToggle {
	t := locale.T()
	return k.modeToggle(t.Get("settings-format-edit-text"), t.Get("settings-format-edit-parts"), flip)
}

// isOn reports whether the second view shows.
func (m *modeToggle) isOn() bool { return m.active }

// set switches to the second view, on, or the first, and runs flip.
func (m *modeToggle) set(on bool) {
	m.active = on
	text := m.off
	if on {
		text = m.on
	}
	m.label.SetText(text)
	m.flip(on)
}
