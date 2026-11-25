package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"golang.org/x/image/font/gofont/gobold"
)

// The stylesheet named the sans family on `window` alone, so a dialog's
// or a popover's tree, which no window element heads, resolved its
// weights with no family: Change Password's semibold title fell back to
// the regular face, lighter than the rows under it. A title weighs the
// same in a dialog and a popover as in the window.
func TestADialogsAndAPopoversTextResolvesTheWindowsFamily(t *testing.T) {
	k := testKit(t, "")
	bold, err := render.LoadFont(gobold.TTF)
	if err != nil {
		t.Fatal(err)
	}
	sans := k.store.svc.Config().General.FontSans
	widget.SetFaceResolver(func(family string, weight int) (render.Font, bool) {
		return bold, family == sans && weight > 400
	})
	t.Cleanup(func() { widget.SetFaceResolver(nil) })
	sheet := apptheme.New(k.store.svc.Config())
	width := func(element string) int {
		root := widget.NewBox(widget.Column, 0, 0)
		root.SetElement(element)
		title := k.label("Change Password", "heading")
		root.Append(title, false)
		sheet.Attach(root)
		root.Arrange(render.Rect{W: 600, H: 100})
		return title.Measure(widget.Row, -1).Nat
	}
	window := width("window")
	if box := width("box"); box == window {
		t.Fatalf("premise: a heading outside the window measures as one inside it, %dpx", box)
	}
	for _, element := range []string{"dialog", "popover"} {
		if got := width(element); got != window {
			t.Errorf("a heading in a %s measures %dpx, in the window %dpx", element, got, window)
		}
	}
}
