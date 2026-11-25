package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// listsEndWithTheirLabelledAdd checks that every list on every page
// ends one way: its end starts at the list's start edge as its last row,
// save a state saying how adding went, and leads with an add button
// reading what it adds. A bare + only ever follows the parts it adds
// to, on their line, never standing alone where a list's add would.
func listsEndWithTheirLabelledAdd() pageCheck {
	return pageCheck{visit: func(t *testing.T, w *window) {
		walkOwn(w.page, func(x widget.Widget) {
			box, ok := x.(*widget.Box)
			if !ok || !shown(box) {
				return
			}
			var kids []widget.Widget
			for _, c := range box.Children() {
				if shown(c) {
					kids = append(kids, c)
				}
			}
			for i, c := range kids {
				if b, ok := c.(*widget.Button); ok {
					checkAddButton(t, w.pageID, b, box, len(kids))
				}
				end, ok := c.(*widget.Box)
				if !ok || !end.HasClass("list-end") {
					continue
				}
				if got, want := end.Bounds().X, box.Bounds().X; got != want {
					t.Errorf("%s: a list's end starts at x=%d, not at its list's start %d", w.pageID, got, want)
				}
				for _, after := range kids[i+1:] {
					if _, state := after.(*inlineState); !state {
						t.Errorf("%s: a list's end is followed by %T", w.pageID, after)
					}
				}
				if first := end.Children()[0]; !isListAdd(first) {
					t.Errorf("%s: a list's end leads with %T, not its add button", w.pageID, first)
				}
			}
		})
	}}
}

// checkAddButton checks an add button b on page, one of shown children
// of box: a list's reads what it adds, a bare + shares its row.
func checkAddButton(t *testing.T, page string, b *widget.Button, box *widget.Box, shown int) {
	t.Helper()
	switch r, _ := roleOf(b); r {
	case roleListAdd:
		if len(treetest.Labels(b)) == 0 {
			t.Errorf("%s: a list's add button reads nothing", page)
		}
	case roleAdd:
		if box.Axis() != widget.Row || shown < 2 {
			t.Errorf("%s: a bare + %q stands alone on its line", page, b.TooltipText())
		}
	}
}

func isListAdd(w widget.Widget) bool {
	b, ok := w.(*widget.Button)
	if !ok {
		return false
	}
	r, ok := roleOf(b)
	return ok && r == roleListAdd
}

// A list editor says it is empty while it holds no row and not once it
// holds one, and its add button reads what it adds.
func TestAListEditorSaysWhenItIsEmpty(t *testing.T) {
	k := testKit(t, "")
	c := fieldControl(k, k.store, field("launcher.styles", launcherStylesRow)).(*listEditor)
	if !c.end.empty.Visible() || c.end.empty.Text() != locale.T().Get("settings-list-empty") {
		t.Fatalf("an empty list reads %q, shown %v", c.end.empty.Text(), c.end.empty.Visible())
	}
	if !isListAdd(c.end.add) || !slices.Contains(treetest.Labels(c.end.add), locale.T().Get("settings-map-add")) {
		t.Errorf("the add button reads %q", treetest.Labels(c.end.add))
	}
	c.end.add.OnClick()
	if c.end.empty.Visible() {
		t.Error("a list holding a row still says it is empty")
	}
}

// A card list says it is empty while it holds no card, and not once a
// card is added; removing the last says so again.
func TestACardListSaysWhenItIsEmpty(t *testing.T) {
	k := testKit(t, "")
	c := fieldControl(k, k.store, field(worldClockPath, worldClockRow)).(*worldClockEditor).cards
	c.removeItem(0)
	if len(c.cards) != 0 || !c.end.empty.Visible() {
		t.Fatalf("with %d cards the empty line shows %v", len(c.cards), c.end.empty.Visible())
	}
	c.end.add.OnClick()
	if len(c.cards) != 1 || c.end.empty.Visible() {
		t.Errorf("with %d cards the empty line shows %v", len(c.cards), c.end.empty.Visible())
	}
}
