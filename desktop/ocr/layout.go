package ocr

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	wordSpace     = 0.45
	propRunSpaces = 3
)

type token struct {
	text   string
	spaces int
}

func layout(ds []decoded) string {
	minPen := math.Inf(1)
	for _, d := range ds {
		if d.gs.mono {
			minPen = math.Min(minPen, d.glyphs[0].penX)
		}
	}
	var b strings.Builder
	for i, d := range ds {
		if i > 0 {
			b.WriteByte('\n')
		}
		if d.gs.mono {
			b.WriteString(strings.Repeat(" ", int(math.Round((d.glyphs[0].penX-minPen)/d.gs.space))))
		}
		for _, t := range disambiguate(tokens(d)) {
			b.WriteString(strings.Repeat(" ", t.spaces))
			b.WriteString(t.text)
		}
	}
	return b.String()
}

func tokens(d decoded) []token {
	out := make([]token, len(d.glyphs))
	for j, g := range d.glyphs {
		out[j].text = g.g.text
		if j == 0 {
			continue
		}
		prev := d.glyphs[j-1]
		gap := g.penX - (prev.penX + prev.g.adv)
		if gap <= wordSpace*d.gs.space {
			continue
		}
		n := max(1, int(math.Round(gap/d.gs.space)))
		if !d.gs.mono && n < propRunSpaces {
			n = 1
		}
		out[j].spaces = n
	}
	return out
}

func disambiguate(ts []token) []token {
	for j := range ts {
		before, after := neighbour(ts, j, -1), neighbour(ts, j, 1)
		switch ts[j].text {
		case "I", "|":
			if unicode.IsLower(before) {
				ts[j].text = "l"
			}
		case "O", "o":
			if (unicode.IsDigit(before) || unicode.IsDigit(after)) && !unicode.IsLetter(before) && !unicode.IsLetter(after) {
				ts[j].text = "0"
			}
		}
	}
	return ts
}

func neighbour(ts []token, j, dir int) rune {
	if dir < 0 {
		if j == 0 || ts[j].spaces > 0 {
			return utf8.RuneError
		}
		r, _ := utf8.DecodeLastRuneInString(ts[j-1].text)
		return r
	}
	if j+1 >= len(ts) || ts[j+1].spaces > 0 {
		return utf8.RuneError
	}
	r, _ := utf8.DecodeRuneInString(ts[j+1].text)
	return r
}
