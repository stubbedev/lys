package settings

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type captureIcon struct{ key, label string }

type captureTile struct {
	title string
	icons []captureIcon
}

var audioCaptureTiles = []captureTile{
	{"settings-capture-microphone", []captureIcon{{"mic-icon", "settings-capture-live"}, {"mic-icon-muted", "settings-capture-muted"}}},
	{"settings-capture-camera", []captureIcon{{"capture-camera-icon", "settings-capture-in-use"}}},
	{"settings-capture-screen", []captureIcon{{"capture-screen-icon", "settings-capture-in-use"}}},
}

const (
	micHideWhenIdle = "mic-hide-when-idle"
	captureColor    = "capture-color"
)

func captureTilesRow(module string) rowSpec {
	var paths []string
	for _, tile := range audioCaptureTiles {
		for _, icon := range tile.icons {
			paths = append(paths, module+"."+icon.key)
		}
	}
	paths = append(paths, module+"."+micHideWhenIdle, module+"."+captureColor)
	return field(paths[0], withKey("settings-capture-tiles"), func(r *rowSpec) {
		r.fullWidth = true
		r.also = paths[1:]
		r.editor = func(k *kit, _ slot, _ config.FieldMeta) control { return newCaptureTiles(k, module) }
	})
}

type captureTiles struct {
	*widget.Box
	icons  []*iconEditor
	hide   *choicePick
	hideAt slot
	color  *colorValueEditor
}

func newCaptureTiles(k *kit, module string) *captureTiles {
	t := locale.T()
	c := &captureTiles{Box: gapBox(widget.Column), hideAt: k.store.slot(module + "." + micHideWhenIdle)}
	c.AddClass("capture-tiles")
	row := widget.NewWrapBox(choiceCardGap, choiceCardGap, widget.JustifyStart)
	for i, tile := range audioCaptureTiles {
		card := gapBox(widget.Column)
		card.AddClass(css.Card, "capture-tile")
		card.Append(k.label(t.Get(tile.title)), false)
		icons := gapBox(widget.Row)
		for _, icon := range tile.icons {
			ed := newIconEditor(k, k.store.slot(module+"."+icon.key))
			icons.Append(k.captioned(t.Get(icon.label), ed), false)
			c.icons = append(c.icons, ed)
		}
		card.Append(icons, false)
		if i == 0 {
			c.hide = newChoicePick(k, []string{t.Get("settings-capture-mic-always"), t.Get("settings-capture-mic-recording")}, 0)
			c.hide.OnSelect = func(at int) { _ = c.hideAt.set(at == 1) }
			card.Append(c.hide, false)
		}
		row.Append(card)
	}
	c.Append(row, false)
	colorPath := module + "." + captureColor
	label, _ := config.I18nKey(colorPath)
	c.color = newColorValueEditor(k, k.store.slot(colorPath))
	c.Append(k.captioned(t.Get(label), c.color), false)
	c.refresh()
	return c
}

func (c *captureTiles) refresh() {
	for _, ed := range c.icons {
		ed.refresh()
	}
	hidden, _ := c.hideAt.get().(bool)
	if hidden {
		c.hide.show(1)
	} else {
		c.hide.show(0)
	}
	c.color.refresh()
}
