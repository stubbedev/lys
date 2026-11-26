package filebrowser

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

// Quick Look's Open opens the file shown through the opener the host
// gives the browser; without one Quick Look offers no Open.
func TestQuicklookOpensTheFileThroughTheOpener(t *testing.T) {
	for _, opener := range []bool{true, false} {
		hs := newHarness(t, "")
		var opened []string
		if opener {
			hs.do(func() { hs.c.d.OpenFile = func(path string) { opened = append(opened, path) } })
		}
		dir := filepath.Join(hs.root, "docs")
		path := filepath.Join(dir, "note.txt")
		writeFile(t, path, []byte("a note\n"))
		hs.quicklookOn(t, dir, "note.txt")
		var buttons []*widget.Button
		hs.do(func() { buttons = treetest.All[*widget.Button](hs.c.ui.ql.Root()) })
		if !opener {
			if len(buttons) != 0 {
				t.Errorf("without an opener Quick Look offers %d Open buttons", len(buttons))
			}
			hs.hide()
			continue
		}
		if len(buttons) != 1 {
			t.Fatalf("Quick Look offers %d Open buttons, want one", len(buttons))
		}
		hs.do(func() { buttons[0].OnClick() })
		if !slices.Equal(opened, []string{path}) {
			t.Errorf("Open opened %q, want %q", opened, path)
		}
		hs.hide()
	}
}
