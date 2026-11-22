package settings

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const screenshotShortcutsKey = "settings-screenshot-shortcuts"

type screenshotCapture struct{ sub, key string }

var screenshotCaptures = []screenshotCapture{
	{"region", "settings-action-capture-region"},
	{"output", "settings-action-capture-output"},
	{"window", "settings-action-capture-window"},
	{"text", "settings-action-capture-text"},
	{"qr", "settings-action-capture-qr"},
}

func (c screenshotCapture) command() string { return "lys-shell screenshot " + c.sub }

func (c screenshotCapture) bind() string { return execAction + " " + c.command() }

func screenshotShortcutsRow() rowSpec {
	return wmField(bindsKey, withKey(screenshotShortcutsKey), func(r *rowSpec) {
		r.action = true
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newScreenshotShortcuts(k, s) }
	})
}

type screenshotShortcuts struct {
	*widget.Box
	binds  slot
	chords []*actionChord
}

func newScreenshotShortcuts(k *kit, binds slot) *screenshotShortcuts {
	t := locale.T()
	names := bindActionNames(k)
	s := &screenshotShortcuts{Box: gapBox(widget.Column), binds: binds}
	rows := widget.NewItemGroup()
	for _, c := range screenshotCaptures {
		chord := newActionChord(k, binds, names, c.bind(), t.Get(c.key))
		s.chords = append(s.chords, chord)
		row := k.row(t.Get(c.key), "").withControl(chord.recorder)
		row.AddDetail(chord.clash)
		rows.Append(row)
	}
	if link, ok := allShortcutsLink(k); ok {
		rows.Append(k.row(t.Get("settings-shortcuts-all"), "").withControl(link))
	}
	s.Append(rows, false)
	s.refresh()
	return s
}

func (s *screenshotShortcuts) refresh() {
	for _, c := range s.chords {
		c.refresh(s.binds)
	}
}
