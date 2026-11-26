package viewer

import (
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/dochelper"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/viewer/internal/locale"
)

func docFixture(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "desktop", "docrender", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return writeFile(t, filepath.Join(dir, name), data)
}

func (h *harness) settled(v *Viewer) {
	h.t.Helper()
	d := v.doc
	h.until("the page shown with its text", func() bool {
		return d.active() && d.hasShown && d.shown.Page == d.page && d.textPage == d.page && !d.rendering
	})
	layOut(v)
	h.until("the page drawn at its layout", func() bool {
		t, ok := d.view.tile(d.page, d.box)
		return ok && d.shown == t && !d.rendering
	})
}

func openDocument(t *testing.T, name string) (*harness, *Viewer) {
	t.Helper()
	h := newHarness(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.png"), pngData(t, 8, 8))
	doc := docFixture(t, dir, name)
	writeFile(t, filepath.Join(dir, "z.png"), pngData(t, 8, 8))
	v := h.viewer()
	h.open(v, doc)
	layOut(v)
	return h, v
}

func TestADocumentOpensAtItsFirstPageInTheDocumentView(t *testing.T) {
	h, v := openDocument(t, "reading.pdf")
	h.settled(v)
	d := v.doc
	if d.root.Visible() == false || v.view.Root().Visible() {
		t.Fatal("the document shows in the preview, not the document view")
	}
	if d.info.Pages() != 3 || d.page != 0 || d.pageEntry.Text() != "i" {
		t.Errorf("the document opened at page %d of %d, labelled %q", d.page, d.info.Pages(), d.pageEntry.Text())
	}
	if want := locale.T().Get("viewer-page-count", i18n.Int("pages", 3)); d.pageCount.Text() != want {
		t.Errorf("the page count reads %q, want %q", d.pageCount.Text(), want)
	}
	if !d.view.fit || d.shown.Rect.Min != (image.Point{}) || d.shown.Rect.Dy() != d.box.Y {
		t.Errorf("page 1 shows as %+v in a %v box, want it fitted whole", d.shown, d.box)
	}
	if b := d.area.picture.Bounds(); b.Empty() {
		t.Error("the page's picture has no room")
	}
	if h.titles[len(h.titles)-1] != "reading.pdf" {
		t.Errorf("the window is titled %q", h.titles[len(h.titles)-1])
	}
	h.press(v, "Right")
	h.settled(v)
	if d.page != 1 {
		t.Errorf("Right on a fitted page goes to page %d, want 2", d.page+1)
	}
	h.press(v, "Home")
	h.press(v, "Left")
	if current(t, v) != "a.png" || d.active() || !v.view.Root().Visible() {
		t.Errorf("Left at the first page shows %s, want the file before, in the preview", current(t, v))
	}
}

func TestThePageKeysAndTheFieldStepThroughThePages(t *testing.T) {
	h, v := openDocument(t, "reading.pdf")
	h.settled(v)
	d := v.doc
	steps := []struct {
		key   string
		page  int
		label string
	}{
		{"Next", 1, "1"},
		{"Next", 2, "2"},
		{"Prior", 1, "1"},
		{"End", 2, "2"},
		{"Home", 0, "i"},
		{"Down", 1, "1"},
		{"Up", 0, "i"},
	}
	for _, s := range steps {
		if !h.press(v, s.key) {
			t.Fatalf("%s was not taken", s.key)
		}
		h.settled(v)
		if d.page != s.page || d.pageEntry.Text() != s.label {
			t.Errorf("%s shows page %d labelled %q, want %d labelled %q", s.key, d.page+1, d.pageEntry.Text(), s.page+1, s.label)
		}
	}
	for _, e := range []struct {
		text string
		page int
	}{{"2", 2}, {"1", 1}, {"i", 0}, {" 3 ", 2}} {
		d.enterPage(e.text)
		if d.page != e.page {
			t.Errorf("entering %q shows page %d, want %d", e.text, d.page+1, e.page+1)
		}
	}
	d.enterPage("ix")
	if d.page != 2 || d.pageEntry.Text() != "2" || !strings.Contains(h.toasts[len(h.toasts)-1].text, "ix") {
		t.Errorf("entering a page there is none of shows page %d, reads %q and says %+v", d.page+1, d.pageEntry.Text(), h.toasts)
	}
	h.press(v, "Next")
	if current(t, v) != "z.png" {
		t.Errorf("Next at the last page shows %s, want the next file", current(t, v))
	}
}

func TestZoomDrawsATileOfThePageAtTheScaleAndFitsBack(t *testing.T) {
	h, v := openDocument(t, "reading.pdf")
	h.settled(v)
	d := v.doc
	fitted := d.shown
	for range 4 {
		h.press(v, "plus")
	}
	h.settled(v)
	full := d.info.Sizes[0].At(d.shown.Scale)
	if d.view.fit || d.shown.Scale <= fitted.Scale || d.shown.Rect.Dy() >= full.Y {
		t.Fatalf("zoomed in, the page shows as %+v of %v, fitted %+v", d.shown, full, fitted)
	}
	if d.shown.Rect.Dx() > d.box.X || d.shown.Rect.Dy() > d.box.Y {
		t.Errorf("the tile %v outgrows the %v box", d.shown.Rect, d.box)
	}
	top := d.shown.Rect.Min.Y
	h.press(v, "Down")
	h.settled(v)
	if d.page != 0 || d.shown.Rect.Min.Y <= top {
		t.Errorf("Down on a zoomed page shows page %d from %d, want page 1 further down than %d", d.page+1, d.shown.Rect.Min.Y, top)
	}
	h.press(v, "0")
	h.settled(v)
	if !d.view.fit || d.shown != fitted {
		t.Errorf("0 shows %+v, want the fitted %+v", d.shown, fitted)
	}
	h.press(v, "1")
	h.settled(v)
	if d.shown.Scale != actualScale {
		t.Errorf("1 shows the page at %g, want %g", d.shown.Scale, actualScale)
	}
}

func (h *harness) search(v *Viewer, query string) {
	h.t.Helper()
	if !h.press(v, "ctrl+f") {
		h.t.Fatal("ctrl+f was not taken")
	}
	v.doc.searchFor(query)
}

func TestFindStepsThroughEveryMatchAndHighlightsThem(t *testing.T) {
	h, v := openDocument(t, "reading.pdf")
	h.settled(v)
	d := v.doc
	h.search(v, "Needle")
	if !d.searchBar.Visible() || len(h.focused) == 0 || h.focused[len(h.focused)-1] != d.searchEntry {
		t.Fatal("ctrl+f does not show and focus the search field")
	}
	want := []struct{ page, match int }{{1, 0}, {2, 0}, {2, 1}, {1, 0}}
	for _, w := range want {
		h.press(v, "F3")
		h.until("the match shown", func() bool { return d.page == w.page && d.match == w.match && d.textPage == w.page })
		h.settled(v)
		marks := d.marks()
		if n := len(d.matches); len(marks) != n*len("needle") {
			t.Errorf("page %d highlights %d boxes for %d matches", w.page+1, len(marks), n)
		}
		if got := d.text.text(d.matches[d.match]); got != "needle" {
			t.Errorf("the match reads %q", got)
		}
	}
	h.press(v, "shift+F3")
	h.until("the previous match", func() bool { return d.page == 2 && d.match == 1 })
	if want := locale.T().Get("viewer-match", i18n.Int("match", 2), i18n.Int("matches", 2)); d.searchStatus.Text() != want {
		t.Errorf("the status reads %q, want %q", d.searchStatus.Text(), want)
	}
	d.searchFor("haystack")
	h.press(v, "F3")
	h.until("the search done", func() bool { return d.searchStatus.Text() == locale.T().Get("viewer-not-found") })
	if d.page != 2 || len(d.marks()) != 0 {
		t.Errorf("a search that finds nothing moved to page %d or marks %d boxes", d.page+1, len(d.marks()))
	}
	h.press(v, "Escape")
	if d.searchBar.Visible() || d.query != "" {
		t.Error("Escape leaves the search open")
	}
}

func TestKeysTypeIntoAFocusedField(t *testing.T) {
	h, v := openDocument(t, "reading.pdf")
	h.settled(v)
	d := v.doc
	d.typing = 1
	for _, k := range []string{"m", "Left", "Home", "1", "space", "ctrl+c"} {
		if h.press(v, k) {
			t.Errorf("%s was taken from the focused field", k)
		}
	}
	if !h.press(v, "ctrl+f") || !h.press(v, "Escape") {
		t.Error("ctrl+f and Escape do not act from the focused field")
	}
}

func (h *harness) pointerAt(v *Viewer, box dochelper.Box) (*widget.Router, widget.Point) {
	h.t.Helper()
	d := v.doc
	r, _ := treetest.Pointer(v.Root(), d.area, 1200, 900)
	x, y := d.area.picture.ViewAt(((box.X0+box.X1)/2)*d.shown.Scale-float64(d.shown.Rect.Min.X), ((box.Y0+box.Y1)/2)*d.shown.Scale-float64(d.shown.Rect.Min.Y))
	return r, widget.Point{X: int(x), Y: int(y)}
}

func TestADragSelectsTextAndCtrlCCopiesIt(t *testing.T) {
	h, v := openDocument(t, "reading.pdf")
	h.settled(v)
	d := v.doc
	if h.press(v, "ctrl+c"); len(h.copied) != 0 {
		t.Fatalf("ctrl+c with nothing selected copied %d things", len(h.copied))
	}
	line := d.text.chars
	r, from := h.pointerAt(v, line[0].Box)
	_, to := h.pointerAt(v, line[len("Reading test")-1].Box)
	r.Move(from)
	r.Press(widget.BTNLeft, from, 0)
	r.Move(to)
	r.Release(widget.BTNLeft, to)
	if got := d.selectedText(); got != "Reading test" {
		t.Fatalf("the drag selected %q", got)
	}
	if len(d.marks()) != len("Reading test") {
		t.Errorf("the selection marks %d boxes", len(d.marks()))
	}
	h.press(v, "ctrl+c")
	if len(h.copied) != 1 {
		t.Fatalf("ctrl+c copied %d things", len(h.copied))
	}
	text, err := h.copied[0].Bytes("text/plain;charset=utf-8")
	if err != nil || string(text) != "Reading test" {
		t.Errorf("the clipboard holds %q, %v", text, err)
	}
	r.Press(widget.BTNLeft, from, 0)
	r.Release(widget.BTNLeft, from)
	if d.selectedText() != "" {
		t.Errorf("a click keeps %q selected", d.selectedText())
	}
}

func TestALinkLeadsToItsPageOrOutOfTheDocument(t *testing.T) {
	h, v := openDocument(t, "reading.pdf")
	h.settled(v)
	d := v.doc
	r, plain := h.pointerAt(v, d.text.chars[0].Box)
	r.Press(widget.BTNLeft, plain, 0)
	r.Release(widget.BTNLeft, plain)
	if d.page != 0 || len(h.openedURIs) != 0 {
		t.Fatalf("a click off the links went to page %d and opened %q", d.page+1, h.openedURIs)
	}
	_, out := h.pointerAt(v, d.links[1].Box)
	r.Move(out)
	if d.area.CursorName() != "pointer" {
		t.Errorf("over a link the cursor is %q", d.area.CursorName())
	}
	r.Press(widget.BTNLeft, out, 0)
	r.Release(widget.BTNLeft, out)
	if !slices.Equal(h.openedURIs, []string{"https://example.org/"}) {
		t.Errorf("the link out opened %q", h.openedURIs)
	}
	_, in := h.pointerAt(v, d.links[0].Box)
	r.Press(widget.BTNLeft, in, 0)
	r.Release(widget.BTNLeft, in)
	h.settled(v)
	if d.page != 2 {
		t.Errorf("the link to page 3 went to page %d", d.page+1)
	}
}

func TestTheOutlineListsItsEntriesAndGoesToTheirPages(t *testing.T) {
	h, v := openDocument(t, "reading.pdf")
	h.settled(v)
	d := v.doc
	if d.outline.Visible() {
		t.Fatal("the outline shows before it is asked for")
	}
	h.press(v, "F9")
	if !d.outline.Visible() || !d.outlineToggle.Active() {
		t.Fatal("F9 does not show the outline")
	}
	labels := treetest.Labels(d.outline)
	for _, want := range []string{"Start", "Middle", "Needles"} {
		if !slices.Contains(labels, want) {
			t.Errorf("the outline lists %q, not %s", labels, want)
		}
	}
	treetest.Button(t, d.outline, "Needles").OnClick()
	h.settled(v)
	if d.page != 2 {
		t.Errorf("Needles goes to page %d, want 3", d.page+1)
	}
	h.press(v, "F9")
	if d.outline.Visible() {
		t.Error("F9 again leaves the outline shown")
	}
}

func TestABookOpensWithItsChaptersAsItsOutline(t *testing.T) {
	h, v := openDocument(t, "blocks.epub")
	h.settled(v)
	d := v.doc
	if d.info.Pages() != 2 || len(d.info.Outline) != 2 || d.pageEntry.Text() != "1" {
		t.Errorf("blocks.epub opened as %+v at %q", d.info, d.pageEntry.Text())
	}
}

func TestADocumentThatCannotBeShownSaysWhy(t *testing.T) {
	cut, err := os.ReadFile(filepath.Join("..", "..", "..", "desktop", "docrender", "testdata", "cairo.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	damaged := writeFile(t, filepath.Join(t.TempDir(), "damaged.pdf"), cut[:3000])
	for _, c := range []struct{ file, id string }{
		{"encrypted.pdf", "viewer-document-encrypted"},
		{damaged, "viewer-document-damaged"},
	} {
		h := newHarness(t)
		doc := c.file
		if !filepath.IsAbs(doc) {
			doc = docFixture(t, t.TempDir(), c.file)
		}
		v := h.viewer()
		h.open(v, doc)
		d := v.doc
		h.until("the failure", func() bool { return d.message.Visible() })
		if d.message.Text() != locale.T().Get(c.id) || d.area.Visible() {
			t.Errorf("%s says %q", c.file, d.message.Text())
		}
	}
	h := newHarness(t)
	h.helper = filepath.Join(t.TempDir(), dochelper.Program)
	dir := t.TempDir()
	doc := docFixture(t, dir, "reading.pdf")
	v := h.viewer()
	h.open(v, doc)
	h.until("the failure", func() bool { return v.doc.message.Visible() })
	if v.doc.message.Text() != locale.T().Get("viewer-document-failed") {
		t.Errorf("a document without its helper says %q", v.doc.message.Text())
	}
}
