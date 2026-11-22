package settings

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// launcherStylesRow is launcher.styles: a look per row, named as -style takes
// it, its CSS in a text area of its own.
var launcherStylesRow = listRow(func(config.FieldMeta) listSpec {
	return mapSpec(func(k *kit, pair [2]string, changed func()) listItem {
		t := locale.T()
		row := mapItem{Box: gapBox(widget.Column), key: k.entry(), value: &pair[1]}
		row.key.SetPlaceholder(t.Get("settings-launcher-style-name"))
		row.key.SetAccessibleName(t.Get("settings-launcher-style-name"))
		row.key.SetText(pair[0])
		row.key.OnChanged = func(string) { changed() }
		area := liveTextAreaOf(k, stringSlot(row.value, changed))
		area.SetWrap(false)
		area.AddClass(css.Monospace)
		area.SetPlaceholder(t.Get("settings-launcher-style-css"))
		area.SetAccessibleName(t.Get("settings-launcher-style-css"))
		row.editor = area
		row.Append(row.key, false)
		row.Append(area, false)
		return row
	}, func(key string) bool { return key != "" })
})

var (
	quicklinkForm = formOf(config.LauncherQuicklink{})
	snippetForm   = formOf(config.LauncherSnippet{})
)

var negateCharRow = namedChoiceRow(
	namedValue{"-", "settings-negate-minus"},
	namedValue{"!", "settings-negate-bang"},
	namedValue{"~", "settings-negate-tilde"},
)

var quicklinkCards = cardSpec{
	identity: quicklinkForm.identity("name"),
	fields: []cardField{
		filled(quicklinkForm.field("url")),
		quicklinkForm.with("keyword", liveTextEditor),
		quicklinkForm.with("icon", iconEditorOf),
	},
	blank:      func() map[string]any { return map[string]any{"name": ""} },
	keep:       keyed("name"),
	addKey:     "settings-quicklink-add",
	searchable: true,
}

var snippetCards = cardSpec{
	identity: snippetForm.identity("name"),
	fields: []cardField{
		filled(snippetForm.with("text", newLiveTextArea)),
		snippetForm.with("keyword", liveTextEditor),
	},
	blank:      func() map[string]any { return map[string]any{"name": ""} },
	keep:       keyed("name"),
	addKey:     "settings-snippet-add",
	searchable: true,
}
