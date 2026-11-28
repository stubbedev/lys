package preview

import (
	"image"
	"log"
	"math"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/dochelper"
)

// pageGapPx is the space between two pages, and pagesAhead how many
// screens of pages around the one showing are drawn ahead of a scroll
// (and kept drawn: pages further away let their pixels go).
const (
	pageGapPx  = 8
	pagesAhead = 1
)

// paper is what a page shows until it is drawn.
var paper = render.RGBA(255, 255, 255, 255)

// document is an open document a load found: the client reading it,
// the view's to close, and what it says of itself.
type document struct {
	client *dochelper.Client
	info   dochelper.Info
}

// pages shows a document whole, its pages one under another in a
// scroll, each drawn by the document's helper once it scrolls near and
// let go once it scrolls far away, one page drawing at a time.
type pages struct {
	invoke, spawn func(func())
	scroll        *widget.Scroll
	column        *widget.Box
	slots         []*widget.Image
	doc           document
	// width is the width in logical pixels pages show at, box the
	// size of the scroll.
	width int
	box   image.Point
	// drawn are the pages whose slot shows them, failed those the
	// helper could not draw at all; gen counts the documents shown, so
	// a page drawn for another is dropped, and drawing is the page the
	// helper is drawing, -1 for none.
	drawn   map[int]bool
	failed  map[int]bool
	gen     uint64
	drawing int
}

func newPages(invoke, spawn func(func())) *pages {
	p := &pages{invoke: invoke, spawn: spawn, drawing: -1}
	p.column = widget.NewBox(widget.Column, pageGapPx, 0)
	p.scroll = widget.NewScroll(p.column)
	p.scroll.AddClass("preview-pages")
	p.scroll.ShowBars, p.scroll.VerticalOnly = true, true
	p.scroll.WatchRange(p.refresh)
	return p
}

// show shows d's pages fitted into box: as wide as the box lets the
// first page show whole, at most the box's width.
func (p *pages) show(d document, box image.Point) {
	p.clear()
	p.doc = d
	p.slots = make([]*widget.Image, d.info.Pages())
	for i := range p.slots {
		p.slots[i] = widget.NewImage(nil)
		p.slots[i].AddClass("preview-page")
		p.slots[i].SetPlaceholderColor(paper)
		p.column.Append(p.slots[i], false)
	}
	p.fit(box)
}

// fit lays the pages out for box.
func (p *pages) fit(box image.Point) {
	if len(p.slots) == 0 {
		return
	}
	first := p.doc.info.Sizes[0]
	p.box = box
	p.width = max(1, min(box.X, int(math.Round(float64(box.Y)*first.W/first.H))))
	for i, s := range p.slots {
		size := p.doc.info.Sizes[i]
		h := max(1, int(math.Round(float64(p.width)*size.H/size.W)))
		s.SetSizeRequest(p.width, h)
		s.SetMaximumSize(p.width, h)
	}
	clear(p.drawn)
	clear(p.failed)
	for _, s := range p.slots {
		s.SetImage(nil)
	}
	p.scroll.SetSizeRequest(p.width, box.Y)
	p.scroll.SetMaximumSize(box.X, box.Y)
}

// clear drops the document shown: what it drew goes, and a page it is
// drawing lands nowhere.
func (p *pages) clear() {
	p.gen++
	p.doc, p.slots, p.drawing = document{}, nil, -1
	p.drawn, p.failed = map[int]bool{}, map[int]bool{}
	p.column.Clear()
	p.scroll.SetOffset(0, 0)
}

// Pages is how many pages show, and Drawn whether page (from zero)
// shows drawn.
func (p *pages) Pages() int { return len(p.slots) }

func (p *pages) Drawn(page int) bool { return p.drawn[page] }

// near is the range of pages from first to last that lie within
// pagesAhead screens of the scroll's viewport.
func (p *pages) near() (first, last int) {
	view := p.scroll.Bounds()
	reach := view.H * pagesAhead
	top, bottom := view.Y-reach, view.Y+view.H+reach
	first, last = -1, -1
	for i, s := range p.slots {
		b := s.Bounds()
		if b.Y+b.H < top || b.Y > bottom || b.H == 0 {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
	}
	return first, last
}

// refresh lets go of the pages far from the viewport and draws the
// first near one not drawn yet.
func (p *pages) refresh() {
	if len(p.slots) == 0 || p.scroll.Bounds().H == 0 {
		return
	}
	first, last := p.near()
	for i := range p.drawn {
		if i < first || i > last {
			p.slots[i].SetImage(nil)
			delete(p.drawn, i)
		}
	}
	if p.drawing >= 0 || first < 0 {
		return
	}
	for i := first; i <= last; i++ {
		if !p.drawn[i] && !p.failed[i] {
			p.draw(i)
			return
		}
	}
}

// draw has the helper draw page at the width pages show at, in device
// pixels, off the loop.
func (p *pages) draw(page int) {
	t := p.tile(page)
	p.drawing = page
	gen, client, width := p.gen, p.doc.client, p.width
	p.spawn(func() {
		img, err := client.Render(t)
		p.invoke(func() { p.landed(gen, width, page, img, err) })
	})
}

// tile is page whole at the width pages show at, in device pixels,
// within what the helper draws in one tile.
func (p *pages) tile(page int) dochelper.Tile {
	size := p.doc.info.Sizes[page]
	scale := float64(p.width) * p.deviceScale() / size.W
	scale = min(scale, dochelper.MaxSize/size.W, dochelper.MaxSize/size.H, dochelper.MaxScale)
	return dochelper.Tile{Page: page, Scale: scale, Rect: image.Rectangle{Max: size.At(scale)}}
}

func (p *pages) deviceScale() float64 {
	num, denom := widget.DeviceScaleOf(p.scroll)
	if num == 0 {
		return 1
	}
	return float64(num) / float64(denom)
}

// landed shows page drawn as img for the document shown at width,
// unless another shows or the pages were laid out again since.
func (p *pages) landed(gen uint64, width, page int, img *image.RGBA, err error) {
	if gen != p.gen {
		return
	}
	p.drawing = -1
	if err != nil {
		log.Printf("preview: drawing page %d: %v", page+1, err)
	}
	switch {
	case width != p.width:
	case img == nil:
		p.failed[page] = true
	default:
		p.slots[page].SetImage(img)
		p.drawn[page] = true
	}
	p.refresh()
}
