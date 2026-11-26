// Package docrender opens, renders and reads documents with folio: PDF,
// EPUB, MOBI and AZW3, FB2, CHM, DOCX, PPTX, XLSX, HTML and plain text,
// and SVG: a page fitted into a box or a tile of it at a scale, what
// the document says of itself (page sizes, labels, outline), and a
// page's text with the box of every character and its links. It parses untrusted input with no deadline of its own, so only
// lys-docthumb links it, in a sandboxed process lys kills when it runs
// too long or past its memory; no long-running lys process imports it.
// Its failures are dochelper's errors.
package docrender

import (
	"errors"
	"fmt"
	"image"
	"math"

	"github.com/gen2brain/folio/doc"
	"github.com/gen2brain/folio/font"
	"github.com/gen2brain/folio/html"
	"github.com/gen2brain/folio/pdf"
	"github.com/gen2brain/folio/raster"
	"github.com/gen2brain/folio/svg"

	"github.com/stubbedev/lys/desktop/dochelper"
)

// MaxPixels is the largest area a page renders into.
const MaxPixels = 1 << 25

// AddFontDirs adds the directories the fonts a document names but does
// not carry are looked up in. It must be called before a document is
// opened.
func AddFontDirs(dirs []string) {
	for _, dir := range dirs {
		font.AddFontDir(dir)
	}
}

// Document is an open document.
type Document struct {
	d    doc.Document
	path string
}

// Open opens the document at path, whatever its format: a file folio
// does not read is ErrUnsupported, one it cannot make sense of
// ErrDamaged, and one that needs a password ErrEncrypted.
func Open(path string) (_ *Document, err error) {
	defer recovered(&err, "opening "+path)
	d, err := doc.Open(path)
	if err != nil {
		return nil, classify("opening "+path, err)
	}
	return &Document{d: d, path: path}, nil
}

// Pages is how many pages the document has; a book is laid out at its
// default page size to count them.
func (d *Document) Pages() (_ int, err error) {
	defer recovered(&err, "counting the pages of "+d.path)
	return d.d.NumPages(), nil
}

// deviceBounds is what every kind of folio page reports its size at a
// resolution by.
type deviceBounds interface {
	DeviceBounds(dpi float64) raster.Rect
}

// boundsDPI is the resolution page sizes are measured at.
const boundsDPI = 72

// Render draws page i, counting from zero, fitted into a w x h box with
// its aspect kept: as large as the box allows, never past MaxPixels
// (ErrTooLarge). A page drawn only in part is returned with the error
// saying what was left out.
func (d *Document) Render(i int, w, h int) (_ *image.RGBA, err error) {
	what := fmt.Sprintf("rendering page %d of %s into %dx%d", i+1, d.path, w, h)
	defer recovered(&err, what)
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("docrender: %s: an empty box", what)
	}
	if int64(w)*int64(h) > MaxPixels {
		return nil, fmt.Errorf("%w: %s: over %d pixels", dochelper.ErrTooLarge, what, MaxPixels)
	}
	p, err := d.page(i, what)
	if err != nil {
		return nil, err
	}
	size, err := d.pageSize(p, what)
	if err != nil {
		return nil, err
	}
	scale := min(float64(w)/size.W, float64(h)/size.H) * (1 - 1e-6)
	img, err := p.ImageDPI(boundsDPI * scale)
	if err != nil {
		err = classify(what, err)
	}
	if img == nil && err == nil {
		err = fmt.Errorf("docrender: %s: nothing was drawn", what)
	}
	return img, err
}

// page is page i, counting from zero, for doing what.
func (d *Document) page(i int, what string) (doc.Page, error) {
	if n := d.d.NumPages(); i < 0 || i >= n {
		return nil, fmt.Errorf("docrender: %s: the document has %d pages", what, n)
	}
	p, err := d.d.Page(i)
	if err != nil {
		return nil, classify(what, err)
	}
	return p, nil
}

// pageSize is p's size in points, measured at boundsDPI, for doing
// what: ErrDamaged for a page of no size or an infinite one.
func (d *Document) pageSize(p doc.Page, what string) (dochelper.Size, error) {
	sized, ok := p.(deviceBounds)
	if !ok {
		return dochelper.Size{}, fmt.Errorf("docrender: %s: a %s page reports no size", what, d.d.Kind())
	}
	b := sized.DeviceBounds(boundsDPI)
	pw, ph := float64(b.X1-b.X0), float64(b.Y1-b.Y0)
	if !(pw > 0 && ph > 0) || math.IsInf(pw, 0) || math.IsInf(ph, 0) {
		return dochelper.Size{}, fmt.Errorf("%w: %s: the page is %gx%g", dochelper.ErrDamaged, what, pw, ph)
	}
	return dochelper.Size{W: pw, H: ph}, nil
}

// Close releases the document.
func (d *Document) Close() error {
	if err := d.d.Close(); err != nil {
		return fmt.Errorf("docrender: closing %s: %w", d.path, err)
	}
	return nil
}

// failures are folio's errors by the dochelper error each one is.
var failures = []struct {
	folio error
	is    error
}{
	{pdf.ErrPassword, dochelper.ErrEncrypted},
	{pdf.ErrInvalid, dochelper.ErrDamaged},
	{html.ErrInvalid, dochelper.ErrDamaged},
	{svg.ErrInvalid, dochelper.ErrDamaged},
	{doc.ErrUnsupported, dochelper.ErrUnsupported},
	{pdf.ErrUnsupported, dochelper.ErrUnsupported},
	{html.ErrUnsupported, dochelper.ErrUnsupported},
	{svg.ErrUnsupported, dochelper.ErrUnsupported},
}

// classify is err from doing what, as the dochelper error it is when it
// is one of folio's.
func classify(what string, err error) error {
	for _, f := range failures {
		if errors.Is(err, f.folio) {
			return fmt.Errorf("%w: %s: %w", f.is, what, err)
		}
	}
	return fmt.Errorf("docrender: %s: %w", what, err)
}

// recovered turns a panic in folio while doing what into ErrDamaged in
// *err: the input led the parser somewhere it does not handle.
func recovered(err *error, what string) {
	if v := recover(); v != nil {
		*err = fmt.Errorf("%w: %s: the renderer panicked: %v", dochelper.ErrDamaged, what, v)
	}
}
