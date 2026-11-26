package docrender

import (
	"errors"
	"fmt"
	"image"
	"math"

	"github.com/gen2brain/folio/doc"
	"github.com/gen2brain/folio/gfx"
	"github.com/gen2brain/folio/pdf"
	"github.com/gen2brain/folio/raster"

	"github.com/stubbedev/lys/desktop/dochelper"
)

// matrixed is what every kind of folio page maps its own space onto a
// device at a resolution with.
type matrixed interface {
	Matrix(dpi float64) raster.Matrix
}

// Info is what the document says of itself: the size of each page in
// points, the labels a PDF gives its pages and the outline.
func (d *Document) Info() (_ dochelper.Info, err error) {
	what := "reading what " + d.path + " holds"
	defer recovered(&err, what)
	n := d.d.NumPages()
	if n < 1 || n > dochelper.MaxPages {
		return dochelper.Info{}, fmt.Errorf("%w: %s: %d pages", dochelper.ErrDamaged, what, n)
	}
	info := dochelper.Info{Sizes: make([]dochelper.Size, n)}
	for i := range n {
		p, err := d.page(i, what)
		if err != nil {
			return dochelper.Info{}, err
		}
		if info.Sizes[i], err = d.pageSize(p, what); err != nil {
			return dochelper.Info{}, err
		}
	}
	if pd, ok := d.d.Underlying().(*pdf.Document); ok {
		if labels := pd.PageLabels(); len(labels) == n {
			info.Labels = labels
		}
	}
	info.Outline = outline(d.d.Outline(), n)
	return info, nil
}

// outline is folio's outline as dochelper's, an entry leading to a page
// the document does not have leading nowhere.
func outline(src []doc.Outline, pages int) []dochelper.Outline {
	if len(src) == 0 {
		return nil
	}
	out := make([]dochelper.Outline, len(src))
	for i, o := range src {
		page := o.Page
		if page < -1 || page >= pages {
			page = -1
		}
		out[i] = dochelper.Outline{Title: o.Title, Page: page, URI: o.URI, Children: outline(o.Children, pages)}
	}
	return out
}

// RenderTile draws tile t of its page, which the document's Info
// allows (dochelper.Info.CheckTile). A tile drawn only in part is
// returned with the error saying what was left out.
func (d *Document) RenderTile(t dochelper.Tile) (_ *image.RGBA, err error) {
	what := fmt.Sprintf("rendering %v of page %d of %s at %g", t.Rect, t.Page+1, d.path, t.Scale)
	defer recovered(&err, what)
	p, err := d.page(t.Page, what)
	if err != nil {
		return nil, err
	}
	m, ok := p.(matrixed)
	if !ok {
		return nil, fmt.Errorf("docrender: %s: a %s page has no device matrix", what, d.d.Kind())
	}
	px := raster.NewPixmap(raster.ModelRGB, t.Rect.Dx(), t.Rect.Dy(), false)
	if px == nil {
		return nil, fmt.Errorf("%w: %s: no room for the tile", dochelper.ErrTooLarge, what)
	}
	px.X, px.Y = t.Rect.Min.X, t.Rect.Min.Y
	px.ClearWhite()
	var dev *gfx.DrawDevice
	if pd, ok := d.d.Underlying().(*pdf.Document); ok {
		dev = pdf.NewDrawDevice(pd, px)
	} else {
		dev = gfx.NewDrawDevice(px)
	}
	runErr := p.Run(dev, m.Matrix(boundsDPI*t.Scale))
	img := gfx.ToRGBA(px)
	if runErr != nil {
		return img, classify(what, runErr)
	}
	return img, nil
}

// pageSpace maps the space a page's text and links are given in onto
// points from its top left corner: a PDF's and a book's are that space
// already, a drawing's is its own.
func (d *Document) pageSpace(p doc.Page) raster.Matrix {
	if m, ok := p.(matrixed); ok && d.d.Kind() == doc.KindSVG {
		return m.Matrix(boundsDPI)
	}
	return raster.Identity
}

// box is r mapped by m as a dochelper.Box, false for one that is not
// finite.
func box(m raster.Matrix, r raster.Rect) (dochelper.Box, bool) {
	r = m.ApplyRect(r)
	b := dochelper.Box{X0: float64(r.X0), Y0: float64(r.Y0), X1: float64(r.X1), Y1: float64(r.Y1)}
	for _, v := range []float64{b.X0, b.Y0, b.X1, b.Y1} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return dochelper.Box{}, false
		}
	}
	return b, b.X0 <= b.X1 && b.Y0 <= b.Y1
}

// Text is what page i says, line by line, with the box every character
// covers; a character placed nowhere a box can say is left out.
func (d *Document) Text(i int) (_ dochelper.Text, err error) {
	what := fmt.Sprintf("reading the text of page %d of %s", i+1, d.path)
	defer recovered(&err, what)
	p, err := d.page(i, what)
	if err != nil {
		return dochelper.Text{}, err
	}
	st, err := p.StructuredText()
	if st == nil {
		return dochelper.Text{}, classify(what, err)
	}
	m := d.pageSpace(p)
	var text dochelper.Text
	for _, b := range st.Blocks {
		for _, l := range b.Lines {
			line := dochelper.Line{Chars: make([]dochelper.Char, 0, len(l.Chars))}
			for _, c := range l.Chars {
				if bx, ok := box(m, c.Quad.Bounds()); ok {
					line.Chars = append(line.Chars, dochelper.Char{Rune: c.Rune, Box: bx})
				}
			}
			if len(line.Chars) > 0 {
				text.Lines = append(text.Lines, line)
			}
		}
	}
	if err != nil {
		return text, classify(what, err)
	}
	return text, nil
}

// Links are where page i leads: to a page of the document or out of it
// to a URI; a link leading nowhere, or covering no area a box can say,
// is left out.
func (d *Document) Links(i int) (_ []dochelper.Link, err error) {
	what := fmt.Sprintf("reading the links of page %d of %s", i+1, d.path)
	defer recovered(&err, what)
	p, err := d.page(i, what)
	if err != nil {
		return nil, err
	}
	m := d.pageSpace(p)
	n := d.d.NumPages()
	var links []dochelper.Link
	for _, l := range p.Links() {
		bx, ok := box(m, l.Rect)
		switch {
		case !ok:
		case l.URI != "":
			links = append(links, dochelper.Link{Box: bx, Page: -1, URI: l.URI})
		case l.Page >= 0 && l.Page < n:
			links = append(links, dochelper.Link{Box: bx, Page: l.Page})
		}
	}
	return links, nil
}

// Reader reads one document for a serving helper: the dochelper.Handler
// lys-docthumb serves with.
type Reader struct{ d *Document }

var _ dochelper.Handler = (*Reader)(nil)

// Open opens the document at path and is what it holds.
func (r *Reader) Open(path string) (dochelper.Info, error) {
	if r.d != nil {
		return dochelper.Info{}, fmt.Errorf("docrender: opening %s with %s open", path, r.d.path)
	}
	d, err := Open(path)
	if err != nil {
		return dochelper.Info{}, err
	}
	info, err := d.Info()
	if err != nil {
		return dochelper.Info{}, errors.Join(err, d.Close())
	}
	r.d = d
	return info, nil
}

// Render draws tile t.
func (r *Reader) Render(t dochelper.Tile) (*image.RGBA, error) { return r.d.RenderTile(t) }

// Text is what a page says.
func (r *Reader) Text(page int) (dochelper.Text, error) { return r.d.Text(page) }

// Links are where a page leads.
func (r *Reader) Links(page int) ([]dochelper.Link, error) { return r.d.Links(page) }

// Close closes the document.
func (r *Reader) Close() error {
	if r.d == nil {
		return nil
	}
	d := r.d
	r.d = nil
	return d.Close()
}
