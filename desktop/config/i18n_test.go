package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
)

// leafPaths lists every leaf field path under t with the struct type
// holding it.
func leafPaths(t reflect.Type, prefix string, visit func(path string, holder reflect.Type)) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	for _, f := range fieldsOf(t) {
		path := f.key
		if prefix != "" {
			path = prefix + "." + f.key
		}
		if isContainer(f.typ) {
			leafPaths(f.typ, path, visit)
			continue
		}
		visit(path, t)
	}
}

// Every field of a prefixed struct has its label in the settings FTL.
func TestEveryLabeledFieldHasItsFTLKey(t *testing.T) {
	loader := i18n.Settings()
	labeled := 0
	leafPaths(typeOf[Config](), "", func(path string, holder reflect.Type) {
		key, ok := I18nKey(path)
		_, prefixed := i18nPrefixes[schemaName(holder)]
		_, f, _ := fieldAtPath(typeOf[Config](), path)
		if prefixed && !f.noI18n && !ok {
			t.Errorf("%s: no key though %s has a prefix", path, schemaName(holder))
		}
		if !ok {
			return
		}
		labeled++
		if !loader.Has(key) {
			t.Errorf("%s: %s is not in the settings FTL", path, key)
		}
	})
	if labeled < 300 {
		t.Errorf("only %d labeled fields; the walk missed the schema", labeled)
	}
}

func TestI18nKey(t *testing.T) {
	for path, want := range map[string]string{
		"osd.enabled":                        "settings-osd-enabled",
		"modules.battery.level-icons":        "settings-modules-battery-level-icons",
		"bar.location":                       "settings-bar-location",
		"idle.dim.ac":                        "settings-idle-timeouts-ac",
		"idle.lock-before-sleep":             "settings-idle-lock-before-sleep",
		"modules.notifications.dnd.schedule": "settings-modules-notifications-dnd-schedule",
	} {
		if got, ok := I18nKey(path); !ok || got != want {
			t.Errorf("I18nKey(%s) = %q %v, want %q", path, got, ok, want)
		}
	}
	for _, path := range []string{"osd", "modules.battery", "nope", "osd.enabled.deeper", "styling.palette_base_theme", ""} {
		if got, ok := I18nKey(path); ok {
			t.Errorf("I18nKey(%s) = %q, want none", path, got)
		}
	}
}

// A field's description reads as words in every language, never as the
// config's or rofi's own syntax: no app_id, no "primary" in quotes, no
// rofi flag or me-*/ml-* action glob, no raw enum values.
func TestNoFieldDescriptionReadsAsRawSyntax(t *testing.T) {
	raw := []string{"app_id", `"primary"`, "« primary »", `"auto"`, "« auto »", ": focused", "-matching-negate-char", "-style <", "me-*", "ml-*", "MouseDPrimary", "spinning-disc", "application-mapped", "is false"}
	for _, lang := range []string{"en-US", "de", "es", "fr", "it", "nl", "pt"} {
		loader, err := i18n.Select(i18n.ConfigAssets(), []i18n.LangID{i18n.MustLangID(lang)})
		if err != nil {
			t.Fatal(err)
		}
		described := 0
		leafPaths(typeOf[Config](), "", func(path string, _ reflect.Type) {
			key, ok := I18nKey(path)
			if !ok || !loader.HasAttr(key, "description") {
				return
			}
			described++
			text := loader.Attr(key, "description")
			for _, r := range raw {
				if strings.Contains(text, r) {
					t.Errorf("%s: %s's description holds %q: %s", lang, path, r, text)
				}
			}
		})
		if described < 300 {
			t.Errorf("%s: only %d described fields; the walk missed the schema", lang, described)
		}
	}
}
