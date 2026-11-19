package settings

import (
	"reflect"
	"testing"

	"github.com/stubbedev/lys/ui/widget"
)

func TestAFolderListAddsWhatTheFolderDialogPicks(t *testing.T) {
	for page, path := range map[string]string{
		"launcher-modes": "launcher.search.file-roots",
		"wallpaper":      "wallpaper.picker-directories",
	} {
		k := testKit(t, "")
		if err := k.store.set(path, []any{"~/Documents"}); err != nil {
			t.Fatal(err)
		}
		c, ok := fieldControl(k, k.store, pageRow(t, page, path)).(*listEditor)
		if !ok {
			t.Fatalf("%s is not a folder list", path)
		}
		f := &fakePickers{}
		k.pickers = f
		c.add.ClickAt(widget.Point{})
		if f.folderFn == nil {
			t.Fatalf("%s: add opened no folder dialog", path)
		}
		if got := k.store.value(path); !reflect.DeepEqual(got, []any{"~/Documents"}) {
			t.Fatalf("%s: opening the dialog wrote %v", path, got)
		}
		f.folderFn("/srv/media")
		if got := k.store.value(path); !reflect.DeepEqual(got, []any{"~/Documents", "/srv/media"}) {
			t.Errorf("%s: picking a folder stored %v", path, got)
		}
		row, ok := c.items[0].(folderItem)
		if !ok {
			t.Fatalf("%s: a row is a %T, not a folder field", path, c.items[0])
		}
		row.entry.OnActivate("~/Notes")
		if got := k.store.value(path); !reflect.DeepEqual(got, []any{"~/Notes", "/srv/media"}) {
			t.Errorf("%s: editing a row stored %v", path, got)
		}
	}
}
