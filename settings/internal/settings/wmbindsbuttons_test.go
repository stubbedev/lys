package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// Keyboard Shortcuts stacked two identical unlabelled "+" buttons, one
// adding a bind and one a mode, and left App Shortcuts' Look again a
// bare glyph beside framed buttons: Add a mode and Look again read
// their names on their faces, and the binds' add button is named for
// what it adds, unlike the mode's.
func TestKeyboardShortcutsButtonsSayWhatTheyDo(t *testing.T) {
	win := laidOutPage(t, "", wmBindsID, windowWidth)
	tr := locale.T()
	for _, key := range []string{"wm-mode-add", "wm-app-shortcuts-reload"} {
		b := treetest.Button(t, win.page, tr.Get(key))
		if !slices.Contains(treetest.Labels(b), tr.Get(key)) {
			t.Errorf("the %q button reads %q on its face", tr.Get(key), treetest.Labels(b))
		}
	}
	bind := treetest.Button(t, win.page, tr.Get("settings-binds-add"))
	if mode := treetest.Button(t, win.page, tr.Get("wm-mode-add")); widget.Describe(bind).Name == widget.Describe(mode).Name {
		t.Error("adding a bind and adding a mode share one name")
	}
}
