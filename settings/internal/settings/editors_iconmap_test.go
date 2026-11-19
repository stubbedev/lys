package settings

import (
	"reflect"
	"testing"
)

func TestAnIconMapPicksTheIconForEachPattern(t *testing.T) {
	for page, path := range map[string]string{
		"window-title": "modules.window-title.icon-mappings",
		"media":        "modules.media.player-icons",
	} {
		k := testKit(t, "")
		_ = k.store.set(path, map[string]any{"firefox*": "ld-globe-symbolic"})
		c, ok := fieldControl(k, k.store, pageRow(t, page, path)).(*listEditor)
		if !ok || len(c.items) != 1 {
			t.Fatalf("%s is not a pattern list: %T", path, fieldControl(k, k.store, pageRow(t, page, path)))
		}
		row := c.items[0].(mapItem)
		icon, ok := row.editor.(*iconEditor)
		if !ok {
			t.Fatalf("%s: the value is a %T, not the icon picker", path, row.editor)
		}
		if err := icon.slot.set("ld-flame-symbolic"); err != nil {
			t.Fatal(err)
		}
		if got := k.store.value(path); !reflect.DeepEqual(got, map[string]any{"firefox*": "ld-flame-symbolic"}) {
			t.Errorf("%s: stored %v", path, got)
		}
		if err := icon.slot.set(42); err == nil {
			t.Errorf("%s: a value that is not an icon name was taken", path)
		}
		c.add.OnClick()
		_ = c.items[1].(mapItem).editor.(*iconEditor).slot.set("ld-x-symbolic")
		if got, _ := k.store.value(path).(map[string]any); len(got) != 1 {
			t.Errorf("%s: an icon without a pattern was written: %v", path, got)
		}
	}
}
