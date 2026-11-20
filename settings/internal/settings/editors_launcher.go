package settings

import "github.com/stubbedev/lys/desktop/config"

var (
	quicklinkForm = formOf(config.LauncherQuicklink{})
	snippetForm   = formOf(config.LauncherSnippet{})
)

var negateCharRow = valueSelectRow(
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
	listCls:    "card-form-list", rowClass: "card-form-row", labelCls: "card-form-label",
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
	listCls:    "card-form-list", rowClass: "card-form-row", labelCls: "card-form-label",
}
