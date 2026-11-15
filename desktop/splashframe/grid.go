// Package splashframe is the boot splash's frame as both sides of the
// hand-over to the greeter compute it: the grid every splash state sits
// on, the logo and where it goes, and the record of the splash's last
// frame that the greeter reproduces as its first.
package splashframe

import (
	"image"

	"github.com/stubbedev/lys/desktop/tokens"
)

// The fixed sizes of the grid at scale 1, in pixels.
const (
	// LogoPx is the side of the lys logo.
	LogoPx = 112
	// IndicatorPx is the side of the throbber and of the lock over a
	// prompt.
	IndicatorPx = 40
	// EntryPx is the width of the prompt's field and of the progress bar.
	EntryPx = 320
)

// Space is the spacing token r in pixels at scale.
func Space(r tokens.Rem, scale int) int {
	return int(r.Px(tokens.RemPx, float64(scale)))
}

// OpticalOrigin is the top left corner of a box of size centred across a
// frame and at the golden ratio of its height, as plymouth places a logo.
func OpticalOrigin(frame, size image.Point) image.Point {
	return image.Pt((frame.X-size.X)/2, frame.Y*382/1000-size.Y/2)
}

// LysLogo is where the lys logo goes on a frame at scale: at the optical
// centre when the status group fits below it there, otherwise centred
// with the group as one block, so a short frame holds every row.
func LysLogo(frame image.Point, scale int) image.Rectangle {
	n := image.Pt(LogoPx*scale, LogoPx*scale)
	at := OpticalOrigin(frame, n)
	block := n.Y + groupGap(scale) + stack(frame, scale, 0).Group.Dy()
	if at.Y+block > frame.Y-Space(tokens.SpaceSM, scale) {
		at.Y = max((frame.Y-block)/2, 0)
	}
	return image.Rectangle{Min: at, Max: at.Add(n)}
}

// Grid is where everything on a splash frame goes, in device pixels: the
// logo, and below it the status group of fixed rows that every state
// fills, so moving between states never moves anything.
type Grid struct {
	Scale int
	// Logo is the logo's box; Group holds every row below it.
	Logo, Group image.Rectangle
	// Busy is the indicator's row and Throb the square in its middle that
	// the throbber and the lock fill.
	Busy, Throb image.Rectangle
	// Title, Secondary and Notice are the text rows.
	Title, Secondary, Notice image.Rectangle
	// Field is the prompt's row and Entry the field in its middle.
	Field, Entry image.Rectangle
	// Hints is the row under the field.
	Hints image.Rectangle
}

type column struct {
	minX, maxX, y int
}

func (c *column) row(h int) image.Rectangle {
	r := image.Rect(c.minX, c.y, c.maxX, c.y+h)
	c.y += h
	return r
}

func (c *column) gap(h int) {
	c.y += h
}

func centred(row image.Rectangle, w int) image.Rectangle {
	mid := (row.Min.X + row.Max.X) / 2
	w = min(w, row.Dx())
	return image.Rect(mid-w/2, row.Min.Y, mid-w/2+w, row.Max.Y)
}

// groupGap is the space between the logo and the status group.
func groupGap(scale int) int {
	return Space(tokens.SpaceXL, scale)
}

// stack is the status group's rows on a frame at scale, the group's top
// at top.
func stack(frame image.Point, scale, top int) Grid {
	s := func(v int) int { return v * scale }
	sp := func(t tokens.Rem) int { return Space(t, scale) }
	g := Grid{Scale: scale}
	margin := sp(tokens.SpaceXL)
	c := column{minX: margin, maxX: frame.X - margin, y: top}
	g.Busy = c.row(s(IndicatorPx))
	c.gap(sp(tokens.SpaceLG))
	g.Title = c.row(sp(tokens.SpaceXL))
	g.Secondary = c.row(sp(tokens.SpaceLG))
	c.gap(sp(tokens.SpaceMD))
	g.Field = c.row(sp(tokens.SpaceXL) + sp(tokens.SpaceSM))
	g.Entry = centred(g.Field, s(EntryPx))
	c.gap(sp(tokens.SpaceSM))
	g.Hints = c.row(sp(tokens.SpaceLG))
	g.Notice = c.row(sp(tokens.SpaceLG))
	g.Group = image.Rect(c.minX, top, c.maxX, c.y)
	n := s(IndicatorPx)
	at := g.Busy.Min.Add(g.Busy.Max).Div(2).Sub(image.Pt(n/2, n/2))
	g.Throb = image.Rectangle{Min: at, Max: at.Add(image.Pt(n, n))}
	return g
}

// Layout is the grid of a frame at scale with the logo in the box logo:
// the status group starts below it. LysLogo leaves room for the group; a
// logo the firmware placed may not, and the group then moves up, never
// over the logo.
func Layout(frame image.Point, scale int, logo image.Rectangle) Grid {
	g := stack(frame, scale, logo.Max.Y+groupGap(scale))
	g.Logo = logo
	overflow := g.Group.Max.Y - (frame.Y - Space(tokens.SpaceSM, scale))
	room := g.Group.Min.Y - (logo.Max.Y + Space(tokens.SpaceSM, scale))
	if up := image.Pt(0, min(overflow, room)); up.Y > 0 {
		for _, at := range g.groupRows() {
			*at = at.Sub(up)
		}
	}
	return g
}

func (g *Grid) groupRows() []*image.Rectangle {
	return []*image.Rectangle{&g.Group, &g.Busy, &g.Throb, &g.Title, &g.Secondary, &g.Notice, &g.Field, &g.Entry, &g.Hints}
}

// Translated is the grid moved by d, for drawing part of a frame into a
// buffer of its own.
func (g Grid) Translated(d image.Point) Grid {
	g.Logo = g.Logo.Add(d)
	for _, at := range g.groupRows() {
		*at = at.Add(d)
	}
	return g
}
