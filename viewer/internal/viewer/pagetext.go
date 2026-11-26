package viewer

import (
	"math"
	"strings"
	"unicode"

	"github.com/stubbedev/lys/desktop/dochelper"
)

type pageText struct {
	chars  []dochelper.Char
	lineOf []int
	folded []rune
	at     []int
}

func newPageText(t dochelper.Text) pageText {
	var p pageText
	for l, line := range t.Lines {
		if l > 0 {
			p.folded = append(p.folded, ' ')
			p.at = append(p.at, -1)
		}
		for _, c := range line.Chars {
			p.folded = append(p.folded, unicode.ToLower(c.Rune))
			p.at = append(p.at, len(p.chars))
			p.chars = append(p.chars, c)
			p.lineOf = append(p.lineOf, l)
		}
	}
	return p
}

type span struct{ from, to int }

func (p pageText) find(query string) []span {
	q := []rune(strings.ToLower(strings.TrimSpace(query)))
	if len(q) == 0 {
		return nil
	}
	var found []span
	for i := 0; i+len(q) <= len(p.folded); i++ {
		if !runesAt(p.folded, i, q) {
			continue
		}
		first, last := p.at[i], p.at[i+len(q)-1]
		if first >= 0 && last >= 0 {
			found = append(found, span{from: first, to: last + 1})
			i += len(q) - 1
		}
	}
	return found
}

func runesAt(s []rune, i int, q []rune) bool {
	for j, r := range q {
		if s[i+j] != r {
			return false
		}
	}
	return true
}

func (p pageText) charAt(x, y float64) int {
	best, bestDist := -1, math.Inf(1)
	for i, c := range p.chars {
		b := c.Box
		dx := max(b.X0-x, 0, x-b.X1)
		dy := max(b.Y0-y, 0, y-b.Y1)
		if d := dy*1e6 + dx; d < bestDist {
			best, bestDist = i, d
		}
	}
	return best
}

func (p pageText) text(s span) string {
	var b strings.Builder
	for i := s.from; i < s.to; i++ {
		if i > s.from && p.lineOf[i] != p.lineOf[i-1] {
			b.WriteByte('\n')
		}
		b.WriteRune(p.chars[i].Rune)
	}
	return b.String()
}

func (p pageText) boxes(s span) []dochelper.Box {
	boxes := make([]dochelper.Box, 0, s.to-s.from)
	for i := s.from; i < s.to; i++ {
		boxes = append(boxes, p.chars[i].Box)
	}
	return boxes
}

func linkAt(links []dochelper.Link, x, y float64) (dochelper.Link, bool) {
	for _, l := range links {
		if l.Box.Contains(x, y) {
			return l, true
		}
	}
	return dochelper.Link{}, false
}
