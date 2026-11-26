package widget

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

// clickAt presses and releases the left button at the center of w.
func clickAt(r *Router, w interface{ Bounds() render.Rect }) {
	b := w.Bounds()
	p := Point{X: b.X + b.W/2, Y: b.Y + b.H/2}
	r.Move(p)
	r.Press(BTNLeft, p, 0)
	r.Release(BTNLeft, p)
}

// TestClickWithin pins GTK's GestureClick on a container: a click on a
// plain descendant reaches the nearest registered container, a button
// inside keeps its own click, a disabled container hears nothing, and
// a press that moves to another widget before release is no click.
func TestClickWithin(t *testing.T) {
	face := entryFace(t)
	white := render.RGB(255, 255, 255)
	title := NewLabel(face, 12, "title", white)
	inner := NewLabel(face, 12, "inner", white)
	closes, cards, nested := 0, 0, 0
	closeBtn := NewButton(NewLabel(face, 12, "x", white), 2, 2)
	closeBtn.OnClick = func() { closes++ }
	group := NewBox(Column, 0, 0)
	group.Append(inner, false)
	group.SetOnClickWithin(func() { nested++ })
	card := NewBox(Column, 4, 4)
	card.Append(title, false)
	card.Append(closeBtn, false)
	card.Append(group, false)
	card.SetOnClickWithin(func() { cards++ })
	SizeWithin(card, Size{W: 200, H: 200})
	card.Arrange(render.Rect{W: 200, H: 200})
	r := &Router{Root: card}

	clickAt(r, title)
	if cards != 1 {
		t.Errorf("a click on the title: card heard %d, want 1", cards)
	}
	clickAt(r, closeBtn)
	if closes != 1 || cards != 1 {
		t.Errorf("close button: closes %d cards %d, want the button alone", closes, cards)
	}
	clickAt(r, inner)
	if nested != 1 || cards != 1 {
		t.Errorf("nested: inner %d card %d, want the nearest container alone", nested, cards)
	}

	// Pressed on the title, released on the inner label: no click.
	tb, ib := title.Bounds(), inner.Bounds()
	r.Press(BTNLeft, Point{X: tb.X + 1, Y: tb.Y + 1}, 0)
	r.Release(BTNLeft, Point{X: ib.X + 1, Y: ib.Y + 1})
	if cards != 1 || nested != 1 {
		t.Error("a press dragged to another widget clicked")
	}

	card.SetEnabled(false)
	clickAt(r, title)
	if cards != 1 {
		t.Error("a disabled card heard a click")
	}
	card.SetEnabled(true)
	card.SetOnClickWithin(nil)
	clickAt(r, title)
	if cards != 1 {
		t.Error("an unregistered hook still fired")
	}
}

// TestARichLabelPassesItsPlainClicksToTheHookAbove pins a notification
// body: a click on its plain text reaches the card around it, a click
// on its link opens the link alone, and a selectable label keeps every
// click, since a click drops its selection.
func TestARichLabelPassesItsPlainClicksToTheHookAbove(t *testing.T) {
	face := testFace(t)
	body := NewRichLabel(face, richPx, `see <a href="https://example.com">here</a>!`, render.RGB(255, 255, 255))
	var links []string
	body.OnLinkClick = func(href string) { links = append(links, href) }
	cards := 0
	card := NewBox(Column, 0, 0)
	card.Append(body, false)
	card.SetOnClickWithin(func() { cards++ })
	size := SizeWithin(card, Size{W: 500, H: 100})
	card.Arrange(render.Rect{W: size.W, H: size.H})
	r := &Router{Root: card}
	_, baseline, ok := body.lineGeom()
	if !ok {
		t.Fatal("the body does not fit its own natural box")
	}
	b := body.Bounds()
	plain := Point{X: b.X + 2, Y: baseline - 2}
	link := Point{X: b.X + int(body.shaped[0].sh.Advance()+body.shaped[1].sh.Advance()/2+0.5), Y: baseline - 2}
	click := func(p Point) {
		r.Move(p)
		r.Press(BTNLeft, p, 0)
		r.Release(BTNLeft, p)
	}

	click(plain)
	if cards != 1 || len(links) != 0 {
		t.Errorf("a click on the body's plain text: card heard %d, links %q; want the card alone", cards, links)
	}
	click(link)
	if cards != 1 || len(links) != 1 || links[0] != "https://example.com" {
		t.Errorf("a click on the body's link: card heard %d, links %q; want the link alone", cards, links)
	}
	body.OnLinkClick = nil
	click(link)
	if cards != 2 {
		t.Errorf("a click on a link nothing opens: card heard %d, want 2", cards)
	}
	body.SetSelectable(true)
	click(plain)
	if cards != 2 {
		t.Errorf("a click on a selectable body reached the card: heard %d, want 2", cards)
	}
}
