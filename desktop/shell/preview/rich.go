package preview

import (
	"unicode/utf8"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/desktop/i18n"
)

// headingScale is how much larger than the text a heading of each
// level (from 1) sets.
var headingScale = [...]float64{1.6, 1.4, 1.2, 1.1, 1, 1}

// What a rendered file sets: a list item's indent per level, how many
// rows of a table its columns' widths are measured over, and the
// fewest and most characters a column is wide.
const (
	itemIndentPx    = 16
	tableMeasured   = 200
	columnMinChars  = 4
	columnMaxChars  = 32
	charWidthPerPx  = 0.6
	columnPaddingPx = 16
)

// specimenSizes are the sizes a font's sample line shows at.
var specimenSizes = [...]float64{48, 32, 24, 16, 12}

// specimenLines are the lines a font's specimen sets under its sample
// line, at specimenLinePx.
var specimenLines = [...]string{"ABCDEFGHIJKLMNOPQRSTUVWXYZ", "abcdefghijklmnopqrstuvwxyz", "0123456789 .,:;!?&@#%()[]{}"}

const specimenLinePx = 20

// scrolled is a column of rows scrolling up and down, from the top.
type scrolled struct {
	scroll *widget.Scroll
	column *widget.Box
}

func newScrolled(class string, spacing int) scrolled {
	column := widget.NewBox(widget.Column, spacing, 0)
	s := scrolled{scroll: widget.NewScroll(column), column: column}
	s.scroll.AddClass(class)
	s.scroll.ShowBars, s.scroll.VerticalOnly, s.scroll.FillY = true, true, true
	return s
}

// reset empties the column and scrolls back to its top.
func (s scrolled) reset() {
	s.column.Clear()
	s.scroll.SetOffset(0, 0)
}

// showMarkdown lays blocks out in s, text in font at px, its bold and
// italics in the family's own faces.
func (s scrolled) showMarkdown(font render.Font, px float64, blocks []mdBlock) {
	s.reset()
	variants := app.FontVariantsOf(font)
	for i := 0; i < len(blocks); i++ {
		b := blocks[i]
		if b.kind != mdCode {
			s.column.Append(markdownBlock(font, px, variants, b), false)
			continue
		}
		code := widget.NewBox(widget.Column, 0, 0)
		code.AddClass("preview-markdown-code")
		for ; i < len(blocks) && blocks[i].kind == mdCode; i++ {
			line := blocks[i].markup
			if line == "" {
				line = " "
			}
			code.Append(widget.NewRichLabel(font, px, "<tt>"+line+"</tt>", 0), false)
		}
		i--
		s.column.Append(code, false)
	}
}

// markdownBlock is the widget block b shows as, text in font at px,
// bold and italics in variants.
func markdownBlock(font render.Font, px float64, variants widget.VariantFunc, b mdBlock) widget.Widget {
	text := func(markup string, size float64) *widget.RichLabel {
		l := widget.NewRichLabel(font, size, markup, 0)
		l.SetVariants(variants)
		l.SetWrap(true)
		return l
	}
	switch b.kind {
	case mdHeading:
		l := text("<b>"+b.markup+"</b>", px*headingScale[b.level-1])
		l.AddClass("preview-markdown-heading")
		return l
	case mdQuote:
		l := text("<i>"+b.markup+"</i>", px)
		l.AddClass("preview-markdown-quote", css.DimLabel)
		return l
	case mdItem:
		row := widget.NewBox(widget.Row, int(px/2), 0)
		row.SetPadding(render.Insets{Left: b.level * itemIndentPx})
		row.Append(widget.NewLabel(font, px, b.marker, 0), false)
		row.Append(text(b.markup, px), true)
		return row
	case mdRule:
		return widget.NewSeparator(widget.Horizontal)
	}
	return text(b.markup, px)
}

// showSpecimen sets the font face's specimen in s: its family's name in
// font at px, the desktop language's sample line at specimenSizes, then
// specimenLines.
func (s scrolled) showSpecimen(font render.Font, px float64, face *render.Typeface) {
	s.reset()
	name := widget.NewLabel(font, px, face.Family(), 0)
	name.AddClass("preview-font-family", css.DimLabel)
	s.column.Append(name, false)
	line := func(text string, size float64) {
		l := widget.NewLabel(face, size, text, 0)
		l.SetWrap(true)
		s.column.Append(l, false)
	}
	sample := i18n.T("preview-font-sample")
	for _, size := range specimenSizes {
		line(sample, size)
	}
	for _, text := range specimenLines {
		line(text, specimenLinePx)
	}
}

// newTable is a table of t's rows in font at px, each column as wide as
// the widest of its first tableMeasured cells within columnMinChars and
// columnMaxChars, the last taking the width left; expand is the column
// that takes it instead, -1 for the last.
func newTable(font render.Font, px float64, t table, expand int) *widget.ColumnView[[]string] {
	if expand < 0 {
		expand = len(t.header) - 1
	}
	cols := make([]widget.TableColumn[[]string], len(t.header))
	for i, title := range t.header {
		chars := utf8.RuneCountInString(title)
		for _, row := range t.rows[:min(len(t.rows), tableMeasured)] {
			if i < len(row) {
				chars = max(chars, utf8.RuneCountInString(row[i]))
			}
		}
		chars = min(max(chars, columnMinChars), columnMaxChars)
		cols[i] = widget.TableColumn[[]string]{
			Title: title, Width: int(float64(chars)*px*charWidthPerPx) + columnPaddingPx, Expand: i == expand,
			Cell: func(row []string) widget.Widget {
				cell := ""
				if i < len(row) {
					cell = row[i]
				}
				l := widget.NewLabel(font, px, cell, 0)
				l.SetEllipsize(widget.EllipsizeEnd)
				l.SetMaxWidthChars(columnMaxChars)
				return l
			},
		}
	}
	v := widget.NewColumnView(font, px, widget.SliceModel[[]string](t.rows), cols)
	v.AddClass("preview-table")
	return v
}
