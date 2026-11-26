package docrender

import (
	"errors"
	"image"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/dochelper"
)

func lineText(l dochelper.Line) string {
	var b strings.Builder
	for _, c := range l.Chars {
		b.WriteRune(c.Rune)
	}
	return b.String()
}

// The reading fixture says how many pages it has and their size, the
// labels it gives them and its outline; a book its pages and the
// outline its headings make, with no labels.
func TestADocumentSaysWhatItHolds(t *testing.T) {
	info, err := open(t, filepath.Join("testdata", "reading.pdf")).Info()
	if err != nil {
		t.Fatal(err)
	}
	letter := dochelper.Size{W: 612, H: 792}
	want := dochelper.Info{
		Sizes:  []dochelper.Size{letter, letter, letter},
		Labels: []string{"i", "1", "2"},
		Outline: []dochelper.Outline{
			{Title: "Start", Page: 0},
			{Title: "Middle", Page: 1, Children: []dochelper.Outline{{Title: "Needles", Page: 2}}},
		},
	}
	if !reflect.DeepEqual(info, want) {
		t.Errorf("reading.pdf holds %+v, want %+v", info, want)
	}
	book, err := open(t, filepath.Join("testdata", "blocks.epub")).Info()
	if err != nil {
		t.Fatal(err)
	}
	if book.Pages() != 2 || len(book.Labels) != 0 || len(book.Outline) != 2 || book.Outline[1].Title != "Two" || book.Outline[1].Page != 1 {
		t.Errorf("blocks.epub holds %+v, want two pages, no labels and an outline of One and Two", book)
	}
}

// A page's text comes line by line in reading order, every character's
// box on the page and the lines running down it; a scanned page says
// nothing.
func TestAPageSaysItsTextWithItsBoxes(t *testing.T) {
	d := open(t, filepath.Join("testdata", "reading.pdf"))
	text, err := d.Text(0)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, l := range text.Lines {
		lines = append(lines, lineText(l))
	}
	if want := []string{"Reading test, page one", "Go to the last page", "Visit example.org"}; !reflect.DeepEqual(lines, want) {
		t.Fatalf("page 1 says %q, want %q", lines, want)
	}
	page := dochelper.Box{X1: 612, Y1: 792}
	lastY := -1.0
	for _, l := range text.Lines {
		lastX := -1.0
		for _, c := range l.Chars {
			b := c.Box
			if b.X0 < page.X0 || b.Y0 < page.Y0 || b.X1 > page.X1 || b.Y1 > page.Y1 || b.X1 <= b.X0 || b.Y1 <= b.Y0 {
				t.Fatalf("%q covers %+v, not an area of the page", c.Rune, b)
			}
			if b.X0 < lastX {
				t.Errorf("%q at %g comes after a character at %g", c.Rune, b.X0, lastX)
			}
			lastX = b.X0
		}
		if y := l.Chars[0].Box.Y0; y <= lastY {
			t.Errorf("line %q at %g is not below the one before at %g", lineText(l), y, lastY)
		}
		lastY = l.Chars[0].Box.Y0
	}
	if first := text.Lines[0].Chars[0].Box; first.Y1 > 100 || first.X0 != 72 {
		t.Errorf("the first character covers %+v, want it at the page's top left margin, 72 pt in", first)
	}
	scan, err := open(t, filepath.Join("testdata", "jbig2.pdf")).Text(0)
	if err != nil || len(scan.Lines) != 0 {
		t.Errorf("a scanned page says %+v, %v, want nothing", scan, err)
	}
	if _, err := d.Text(3); err == nil {
		t.Error("page 4 of 3 has text")
	}
}

// A page's links lead to a page or out to a URI, over the lines they
// cover; a page without links has none.
func TestAPageSaysWhereItLeads(t *testing.T) {
	d := open(t, filepath.Join("testdata", "reading.pdf"))
	links, err := d.Links(0)
	if err != nil {
		t.Fatal(err)
	}
	want := []dochelper.Link{
		{Box: dochelper.Box{X0: 72, Y0: 80, X1: 300, Y1: 102}, Page: 2},
		{Box: dochelper.Box{X0: 72, Y0: 104, X1: 300, Y1: 126}, Page: -1, URI: "https://example.org/"},
	}
	if !reflect.DeepEqual(links, want) {
		t.Errorf("page 1 leads %+v, want %+v", links, want)
	}
	text, err := d.Text(0)
	if err != nil {
		t.Fatal(err)
	}
	for i, l := range want {
		c := text.Lines[i+1].Chars[0].Box
		if !l.Box.Contains((c.X0+c.X1)/2, (c.Y0+c.Y1)/2) {
			t.Errorf("the link %+v does not cover line %q at %+v", l, lineText(text.Lines[i+1]), c)
		}
	}
	if none, err := d.Links(1); err != nil || len(none) != 0 {
		t.Errorf("page 2 leads %+v, %v, want nowhere", none, err)
	}
}

// A tile is the part of the page it names, pixel for pixel as the page
// drawn whole at its scale, and a tile of the margin is blank.
func TestATileIsThatPartOfThePage(t *testing.T) {
	d := open(t, filepath.Join("testdata", "cairo.pdf"))
	info, err := d.Info()
	if err != nil {
		t.Fatal(err)
	}
	const scale = 0.5
	whole := image.Rectangle{Max: info.Sizes[0].At(scale)}
	page, err := d.RenderTile(dochelper.Tile{Scale: scale, Rect: whole})
	if err != nil {
		t.Fatal(err)
	}
	if page.Rect.Size() != whole.Size() {
		t.Fatalf("the whole page at %g is %v, want %v", scale, page.Rect.Size(), whole.Size())
	}
	if ink := inkShare(page); ink < minInkShare {
		t.Fatalf("the whole page is %.1f%% ink", 100*ink)
	}
	part := image.Rect(40, 60, 140, 180)
	tile, err := d.RenderTile(dochelper.Tile{Scale: scale, Rect: part})
	if err != nil {
		t.Fatal(err)
	}
	if tile.Rect != (image.Rectangle{Max: part.Size()}) {
		t.Fatalf("the tile is %v, want %v at the origin", tile.Rect, part.Size())
	}
	if share := blockShare(tile, page.SubImage(part)); share > maxBlockShare {
		t.Errorf("%.1f%% of the tile's blocks differ from the page drawn whole", 100*share)
	}
	margin, err := d.RenderTile(dochelper.Tile{Scale: scale, Rect: image.Rect(0, whole.Max.Y-8, 8, whole.Max.Y)})
	if err != nil {
		t.Fatal(err)
	}
	if ink := inkShare(margin); ink != 0 {
		t.Errorf("a tile of the bottom left margin is %.1f%% ink, want blank", 100*ink)
	}
}

// A Reader opens one document and says what it holds; a second one, or
// one that is no document, it refuses.
func TestAReaderOpensOneDocument(t *testing.T) {
	var r Reader
	info, err := r.Open(filepath.Join("testdata", "jbig2.pdf"))
	if err != nil || info.Pages() != 3 {
		t.Fatalf("opening jbig2.pdf = %+v, %v, want 3 pages", info, err)
	}
	if _, err := r.Open(filepath.Join("testdata", "cairo.pdf")); err == nil {
		t.Error("a second document opened")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Open(filepath.Join("testdata", "encrypted.pdf")); !errors.Is(err, dochelper.ErrEncrypted) {
		t.Errorf("opening an encrypted PDF = %v, want ErrEncrypted", err)
	}
}
