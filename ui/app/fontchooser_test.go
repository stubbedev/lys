package app

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/stubbedev/lys/ui/widget"
)

// The font chooser's search, labelled monospace filter and list share
// the dialog's content edges: they start at the title's x, the list
// ends where the buttons do, and the filter stands centred on the
// search entry's line (#275).
func TestTheFontChooserLinesUpWithItsTitleAndButtons(t *testing.T) {
	face, err := render.LoadFont(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	a := &Application{tooltipFace: face}
	frame, err := a.dialogRoot(fontChooserDialog(face, []string{"alpha", "beta", "gamma"}, FontChooserConfig{FamilyOnly: true}), func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	root := frame.root
	sz := widget.SizeWithin(root, widget.Size{W: 380, H: 420})
	root.Arrange(render.Rect{W: 380, H: max(sz.H, 420)})
	var (
		title, search, list, mono render.Rect
		monoLabel                 string
		buttonsEnd                int
	)
	var walk func(w widget.Widget)
	walk = func(w widget.Widget) {
		switch x := w.(type) {
		case *widget.Label:
			if x.Text() == "Pick a font" {
				title = x.Bounds()
			}
		case *widget.Entry:
			search = x.Bounds()
		case *widget.List:
			list = x.Bounds()
		case *widget.CheckButton:
			mono = x.Bounds()
			if l, ok := x.Child().(*widget.Label); ok {
				monoLabel = l.Text()
			}
		case *widget.Button:
			buttonsEnd = max(buttonsEnd, x.Bounds().X+x.Bounds().W)
		}
		if c, ok := w.(interface{ Children() []widget.Widget }); ok {
			for _, k := range c.Children() {
				walk(k)
			}
		}
	}
	walk(root)
	if title.W == 0 || search.W == 0 || list.W == 0 || mono.W == 0 || buttonsEnd == 0 {
		t.Fatalf("title %v search %v list %v filter %v buttons end %d: a part is missing", title, search, list, mono, buttonsEnd)
	}
	if search.X != title.X || list.X != title.X {
		t.Errorf("search starts at x=%d and the list at x=%d, want the title's x=%d", search.X, list.X, title.X)
	}
	if end := list.X + list.W; end != buttonsEnd {
		t.Errorf("the list ends at x=%d, want the buttons' end x=%d", end, buttonsEnd)
	}
	if mono.X+mono.W != buttonsEnd {
		t.Errorf("the filter ends at x=%d, want the row to end with the list at x=%d", mono.X+mono.W, buttonsEnd)
	}
	if d := (mono.Y + mono.H/2) - (search.Y + search.H/2); d < -1 || d > 1 {
		t.Errorf("the filter's centre is %dpx off the search entry's", d)
	}
	if monoLabel == "" {
		t.Error("the monospace filter has no label")
	}
}
