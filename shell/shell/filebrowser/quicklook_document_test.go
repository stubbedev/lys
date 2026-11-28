package filebrowser

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/stubbedev/lys/desktop/dochelper"
	"github.com/stubbedev/lys/desktop/dochelper/dochelpertest"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/widget"
)

// clients records every document client Quick Look opens.
type clients struct {
	mu   sync.Mutex
	open []*dochelper.Client
}

// readDocuments gives hs the test binary as lys-docthumb, serving
// documents through docrender, and records the clients it opens.
func readDocuments(t *testing.T, hs *harness) *clients {
	t.Helper()
	open := dochelper.Opener(dochelpertest.Installed(t))
	cs := &clients{}
	hs.do(func() {
		hs.c.d.Documents = func(path string) *dochelper.Client {
			c := open(path)
			cs.mu.Lock()
			cs.open = append(cs.open, c)
			cs.mu.Unlock()
			return c
		}
	})
	return cs
}

func (cs *clients) all() []*dochelper.Client {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return slices.Clone(cs.open)
}

// docFixture copies the docrender test document name into the docs
// folder and is that folder.
func docFixture(t *testing.T, hs *harness, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "desktop", "docrender", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(hs.root, "docs")
	writeFile(t, filepath.Join(dir, name), data)
	return dir
}

// untilDrawn lays the window out until Quick Look shows page drawn.
func (hs *harness) untilDrawn(t *testing.T, page int) {
	t.Helper()
	waittest.Until(t, func() bool {
		var drawn bool
		hs.do(func() {
			lay(hs.cfg.Root, 1400, 1000)
			drawn = hs.c.ui.ql.PageDrawn(page)
		})
		return drawn
	}, "Quick Look to draw a page")
}

// A multi-page PDF shows whole in Quick Look: every page laid out one
// under another, drawn as it scrolls near and not before.
func TestQuicklookScrollsThroughAWholeDocument(t *testing.T) {
	hs := newHarness(t, "")
	readDocuments(t, hs)
	dir := docFixture(t, hs, "reading.pdf")
	hs.quicklookOn(t, dir, "reading.pdf")
	var shown preview.Renderer
	var pages int
	hs.do(func() { shown, pages = hs.c.ui.ql.Renderer(), hs.c.ui.ql.Pages() })
	if shown != preview.Document || pages != 3 {
		t.Fatalf("Quick Look shows reading.pdf as %v with %d pages, want its 3 pages", shown, pages)
	}
	hs.untilDrawn(t, 1)
	var early bool
	hs.do(func() { early = hs.c.ui.ql.PageDrawn(2) })
	if early {
		t.Error("the last page drew before it scrolled near")
	}
	var scroll *widget.Scroll
	var slots []*widget.Image
	hs.do(func() {
		scroll = treetest.WithClass(hs.c.ui.ql.Root(), "preview-pages")[0].(*widget.Scroll)
		slots = treetest.All[*widget.Image](scroll)
	})
	for page, slot := range slots {
		hs.do(func() {
			lay(hs.cfg.Root, 1400, 1000)
			_, y := scroll.Offset()
			scroll.SetOffset(0, y+slot.Bounds().Y-scroll.Bounds().Y)
		})
		hs.untilDrawn(t, page)
	}
	hs.hide()
}

// An EPUB shows its laid out pages.
func TestQuicklookShowsAnEPUBsPages(t *testing.T) {
	hs := newHarness(t, "")
	readDocuments(t, hs)
	dir := docFixture(t, hs, "blocks.epub")
	hs.quicklookOn(t, dir, "blocks.epub")
	var shown preview.Renderer
	var pages int
	hs.do(func() { shown, pages = hs.c.ui.ql.Renderer(), hs.c.ui.ql.Pages() })
	if shown != preview.Document || pages == 0 {
		t.Fatalf("Quick Look shows blocks.epub as %v with %d pages", shown, pages)
	}
	hs.untilDrawn(t, 0)
	hs.hide()
}

// A damaged document falls back to its icon (here, with no thumbnail)
// and says why, as the viewer does; an encrypted one says it is
// protected.
func TestQuicklookOnADocumentThatDoesNotOpenSaysWhy(t *testing.T) {
	hs := newHarness(t, "")
	readDocuments(t, hs)
	dir := docFixture(t, hs, "encrypted.pdf")
	writeFile(t, filepath.Join(dir, "broken.pdf"), []byte("%PDF-1.7\nnot a document at all"))
	for name, id := range map[string]string{"broken.pdf": "preview-document-damaged", "encrypted.pdf": "preview-document-encrypted"} {
		hs.quicklookOn(t, dir, name)
		var shown preview.Renderer
		var labels []string
		hs.do(func() { shown, labels = hs.c.ui.ql.Renderer(), treetest.Labels(hs.c.ui.ql.Root()) })
		if shown != preview.Icon || !slices.Contains(labels, i18n.T(id)) {
			t.Errorf("%s shows as %v saying %q, want its icon and %q", name, shown, labels, i18n.T(id))
		}
		hs.key(xkb.KeyEscape, 0, nil)
	}
	hs.hide()
}

// Quick Look opens one client for the document it shows and closes it
// once it closes or moves to another file.
func TestQuicklookClosesTheDocumentItShowed(t *testing.T) {
	hs := newHarness(t, "")
	cs := readDocuments(t, hs)
	dir := docFixture(t, hs, "reading.pdf")
	docFixture(t, hs, "cairo.pdf")
	hs.quicklookOn(t, dir, "cairo.pdf")
	hs.key(xkb.KeyRight, 0, nil)
	hs.waitPreview(t, "reading.pdf")
	opened := cs.all()
	if len(opened) != 2 {
		t.Fatalf("Quick Look opened %d clients over two documents", len(opened))
	}
	closed := func(c *dochelper.Client) bool {
		_, err := c.Info()
		return err != nil
	}
	waittest.Until(t, func() bool { return closed(opened[0]) }, "the first document's client to close")
	if closed(opened[1]) {
		t.Error("the document showing has its client closed")
	}
	hs.key(xkb.KeyEscape, 0, nil)
	waittest.Until(t, func() bool { return closed(opened[1]) }, "the closed Quick Look's client to close")
	hs.hide()
}
