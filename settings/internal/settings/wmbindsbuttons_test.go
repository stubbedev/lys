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

// With no mode, an empty box of modes stood between the binds' add
// button and the mode's, so 16px parted them where 8px parted the binds'
// from the line above it. The binds' add button sits as far from the
// mode's as from what comes above it (EvenlySpaced).
func TestTheBindsAddButtonsAreEvenlySpaced(t *testing.T) {
	k, w, _, _ := wmKit(t, "binds: []\n")
	tb := newShortcutTable(k, w.slot(bindsKey))
	laidOut(tb)
	tr := locale.T()
	bind := treetest.Button(t, tb, tr.Get("settings-binds-add")).Bounds()
	mode := treetest.Button(t, tb, tr.Get("wm-mode-add")).Bounds()
	above := treetest.LabelReading(t, tb, tr.Get("settings-list-empty")).Bounds()
	if over, under := bind.Y-(above.Y+above.H), mode.Y-(bind.Y+bind.H); over != under {
		t.Errorf("the binds' add button sits %dpx under the line above and %dpx over the mode's", over, under)
	}
}
