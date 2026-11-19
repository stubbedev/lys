package config

import (
	"reflect"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
)

func itemKeys[T any](t *testing.T, loader *i18n.Loader) int {
	t.Helper()
	ty := typeOf[T]()
	for _, f := range fieldsOf(ty) {
		key, ok := ItemI18nKey[T](f.key)
		if !ok {
			t.Errorf("%s.%s: no label key", schemaName(ty), f.key)
			continue
		}
		if !loader.Has(key) {
			t.Errorf("%s.%s: %s is not in the settings FTL", schemaName(ty), f.key, key)
		}
	}
	return len(fieldsOf(ty))
}

func TestEveryEditedItemFieldHasItsFTLKey(t *testing.T) {
	loader := i18n.Settings()
	n := itemKeys[LauncherQuicklink](t, loader) + itemKeys[LauncherSnippet](t, loader) + itemKeys[CustomModuleDefinition](t, loader)
	if n < 30 {
		t.Errorf("only %d item fields; the walk missed the structs", n)
	}
}

func TestACustomModuleIsNamedByItsID(t *testing.T) {
	m := CustomModule("gpu")
	if id, ok := m.CustomID(); m != "custom-gpu" || !ok || id != "gpu" {
		t.Errorf("CustomModule(gpu) = %q, its id %q %v", m, id, ok)
	}
	var parsed BarModule
	if err := parsed.UnmarshalConfig(string(m)); err != nil || parsed != m {
		t.Errorf("the layout reads %q as %q, %v", m, parsed, err)
	}
	if _, ok := ModuleClock.CustomID(); ok {
		t.Error("a built-in module named a custom id")
	}
}

func TestItemFieldDescribesAFieldOfAListItem(t *testing.T) {
	url, ok := ItemField[LauncherQuicklink]("url")
	if !ok || url.Kind != FieldText || len(url.Placeholders) != 1 || url.Placeholders[0].Token != "{query}" {
		t.Fatalf("quicklink url = %+v %v, want text with the {query} placeholder", url, ok)
	}
	if key := url.Placeholders[0].Key; key != "settings-launcher-quicklink-url-placeholder-query" || !i18n.Settings().Has(key) {
		t.Errorf("the query placeholder's message is %q", key)
	}
	mode, ok := ItemField[CustomModuleDefinition]("mode")
	if !ok || !reflect.DeepEqual(mode, MetaOf[ExecutionMode]()) {
		t.Errorf("custom mode = %+v %v, want the ExecutionMode enum", mode, ok)
	}
	if cmd, ok := ItemField[CustomModuleDefinition]("command"); !ok || cmd.Type != "Command" || !cmd.Optional {
		t.Errorf("custom command = %+v %v, want an optional Command", cmd, ok)
	}
	if key, ok := ItemI18nKey[CustomModuleDefinition]("interval-ms"); !ok || key != "settings-modules-custom-interval-ms" {
		t.Errorf("interval-ms label key = %q %v", key, ok)
	}
	for _, key := range []string{"nope", "", "url"} {
		if _, ok := ItemField[CustomModuleDefinition](key); ok {
			t.Errorf("ItemField[CustomModuleDefinition](%q) described a field it lacks", key)
		}
		if _, ok := ItemI18nKey[CustomModuleDefinition](key); ok {
			t.Errorf("ItemI18nKey[CustomModuleDefinition](%q) named a key for a field it lacks", key)
		}
	}
	if _, ok := ItemI18nKey[FitMode]("x"); ok {
		t.Error("a type without a prefix named a label key")
	}
}
