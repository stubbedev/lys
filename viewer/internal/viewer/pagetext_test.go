package viewer

import (
	"testing"

	"github.com/stubbedev/lys/desktop/dochelper"
)

func line(text string, y float64) dochelper.Line {
	var l dochelper.Line
	for i, r := range []rune(text) {
		x := float64(10 * i)
		l.Chars = append(l.Chars, dochelper.Char{Rune: r, Box: dochelper.Box{X0: x, Y0: y, X1: x + 10, Y1: y + 10}})
	}
	return l
}

func TestFindMatchesWithoutCaseAndAcrossLines(t *testing.T) {
	p := newPageText(dochelper.Text{Lines: []dochelper.Line{line("A needle in", 0), line("hay NEEDLE", 20)}})
	found := p.find(" needle ")
	if len(found) != 2 || p.text(found[0]) != "needle" || p.text(found[1]) != "NEEDLE" {
		t.Fatalf("found %v", found)
	}
	if across := p.find("in hay"); len(across) != 1 || p.text(across[0]) != "in\nhay" {
		t.Errorf("a match across a line break is %v", across)
	}
	for _, none := range []string{"", "  ", "pin", "needles"} {
		if got := p.find(none); len(got) != 0 {
			t.Errorf("%q found %v", none, got)
		}
	}
}

func TestTheCharacterAtAPointIsTheNearestOnItsLine(t *testing.T) {
	p := newPageText(dochelper.Text{Lines: []dochelper.Line{line("ab", 0), line("cd", 20)}})
	for _, c := range []struct {
		x, y float64
		want int
	}{{5, 5, 0}, {15, 5, 1}, {500, 5, 1}, {-50, 25, 2}, {15, 1000, 3}} {
		if got := p.charAt(c.x, c.y); got != c.want {
			t.Errorf("at %g, %g is character %d, want %d", c.x, c.y, got, c.want)
		}
	}
	if got := newPageText(dochelper.Text{}).charAt(0, 0); got != -1 {
		t.Errorf("a page with no text has character %d", got)
	}
}
