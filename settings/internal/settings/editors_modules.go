package settings

import (
	"github.com/stubbedev/lys/desktop/config"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// The module pages' own card lists (the tray overrides, the calendars,
// the mail accounts).

// trayOverrideList edits the tray overrides: a card per override, its item
// name or pattern in the header, the icon picked and the color chosen; a
// nameless override is not written, and an empty icon is unset.
var trayOverrideList = cardRow(cardSpec{
	identity: &cardField{key: "name", label: "settings-tray-override-name", editor: func(k *kit, s slot) control {
		return newLiveText(k, s, locale.T().Get("settings-tray-name-placeholder"))
	}},
	fields: []cardField{
		{key: "icon", label: "settings-tray-override-icon", editor: func(k *kit, s slot) control { return newIconEditor(k, s) }},
		{key: "color", label: "settings-tray-override-color", editor: optionalColor},
	},
	blank: func() map[string]any { return map[string]any{"name": ""} },
	keep:  func(item map[string]any) bool { name, _ := item["name"].(string); return name != "" },
	clean: func(item map[string]any) map[string]any {
		if icon, _ := item["icon"].(string); icon == "" {
			delete(item, "icon")
		}
		return item
	},
	listCls: "card-form-list", rowClass: "card-form-row", labelCls: "card-form-label",
})

var calendarList = cardRow(cardSpec{
	identity: &cardField{key: "path", label: "settings-calendar-path", editor: func(k *kit, s slot) control { return newFolderEditor(k, s) }},
	fields:   []cardField{{key: "color", label: "settings-calendar-color", editor: optionalColor}},
	blank:    func() map[string]any { return map[string]any{"path": ""} },
	keep:     func(item map[string]any) bool { path, _ := item["path"].(string); return path != "" },
	listCls:  "card-form-list", rowClass: "card-form-row", labelCls: "card-form-label",
})

// mailAccountList edits the mail accounts: a card per account, its name in the header; a nameless account is not written,
// an empty icon is unset (the provider's default).
var mailAccountList = cardRow(cardSpec{
	identity: &cardField{key: "name", label: "settings-mail-account-name", editor: func(k *kit, s slot) control {
		return newLiveText(k, s, "name")
	}},
	fields: []cardField{
		{key: "query", label: "settings-mail-account-query", fill: true, editor: func(k *kit, s slot) control {
			return newMailQueryEditor(k, s, false)
		}},
		{key: "provider", label: "settings-mail-account-provider", editor: func(k *kit, s slot) control {
			return enumEditor(k, s, config.MetaOf[config.MailProvider]())
		}},
		{key: "icon", label: "settings-mail-account-icon", editor: func(k *kit, s slot) control { return newIconEditor(k, s) }},
	},
	blank: func() map[string]any {
		return map[string]any{"name": "", "query": ""}
	},
	keep: func(item map[string]any) bool { name, _ := item["name"].(string); return name != "" },
	clean: func(item map[string]any) map[string]any {
		if icon, _ := item["icon"].(string); icon == "" {
			delete(item, "icon")
		}
		return item
	},
	listCls: "card-form-list", rowClass: "card-form-row", labelCls: "card-form-label",
})

func presetBlank(preset any) func() map[string]any {
	return func() map[string]any {
		m, _ := config.Encode(preset).(map[string]any)
		return m
	}
}

func presetNamed(item map[string]any) bool { name, _ := item["name"].(string); return name != "" }

func presetName() *cardField {
	return &cardField{key: "name", label: "settings-generator-preset-name", editor: func(k *kit, s slot) control {
		return newLiveText(k, s, "name")
	}}
}

func presetSpin(meta config.FieldMeta) func(k *kit, s slot) control {
	return func(k *kit, s slot) control { return newNumber(k, s, meta.Min, meta.Max, 1, 0) }
}

func presetToggle(k *kit, s slot) control { return newToggle(k, s) }

var passwordPresetList = cardRow(cardSpec{
	identity: presetName(),
	fields: []cardField{
		{key: "length", label: "settings-generator-preset-length", editor: presetSpin(config.MetaOf[config.PasswordLength]())},
		{key: "charsets", label: "settings-generator-preset-charsets", fill: true, editor: func(k *kit, s slot) control {
			return newEnumChips(k, s, config.MetaOf[config.PasswordCharset](), 1)
		}},
		{key: "avoid-ambiguous", label: "settings-generator-preset-avoid-ambiguous", editor: presetToggle},
		generatorSampleField(passwordRules),
	},
	blank:   presetBlank(config.DefaultsBitwardenPasswordPreset()),
	keep:    presetNamed,
	listCls: "card-form-list", rowClass: "card-form-row", labelCls: "card-form-label",
})

var passphrasePresetList = cardRow(cardSpec{
	identity: presetName(),
	fields: []cardField{
		{key: "words", label: "settings-generator-preset-words", editor: presetSpin(config.MetaOf[config.PassphraseWords]())},
		{key: "separator", label: "settings-generator-preset-separator", editor: func(k *kit, s slot) control {
			return newLiveText(k, s, "-")
		}},
		{key: "capitalize", label: "settings-generator-preset-capitalize", editor: presetToggle},
		{key: "number", label: "settings-generator-preset-number", editor: presetToggle},
		generatorSampleField(passphraseRules),
	},
	blank:   presetBlank(config.DefaultsBitwardenPassphrasePreset()),
	keep:    presetNamed,
	listCls: "card-form-list", rowClass: "card-form-row", labelCls: "card-form-label",
})
