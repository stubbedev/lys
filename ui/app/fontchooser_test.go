package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/stubbedev/lys/ui/internal/sysfont"
	"github.com/stubbedev/lys/ui/widget"
)

// TestFontFilter pins the chooser's search and monospace narrowing
// (#74): case-insensitive substring over the family name, the
// fixed-width classification consulted only when the toggle is on, and
// the input order preserved.
func TestFontFilter(t *testing.T) {
	families := []string{"dejavu sans", "dejavu sans mono", "liberation mono", "noto sans"}
	mono := func(fam string) bool { return strings.HasSuffix(fam, "mono") }

	if got := fontFilter(families, "", false, mono); !slices.Equal(got, families) {
		t.Errorf("unfiltered = %v, want the input", got)
	}
	if got := fontFilter(families, "DEJAVU", false, mono); !slices.Equal(got, []string{"dejavu sans", "dejavu sans mono"}) {
		t.Errorf("case-insensitive search = %v", got)
	}
	if got := fontFilter(families, "", true, mono); !slices.Equal(got, []string{"dejavu sans mono", "liberation mono"}) {
		t.Errorf("monospace filter = %v", got)
	}
	if got := fontFilter(families, "noto", true, mono); len(got) != 0 {
		t.Errorf("search + mono = %v, want none", got)
	}
}

// TestSysfontFamilies pins the enumeration (#74): sorted, deduped, and
// non-empty on a system with fonts installed; every listed name
// resolves back through Best, which is the round trip the chooser's
// OnFont promises.
func TestSysfontFamilies(t *testing.T) {
	families, err := sysfont.Families()
	if err != nil {
		t.Skipf("no system font store in this environment: %v", err)
	}
	if len(families) == 0 {
		t.Fatal("Families() is empty on a system with fonts")
	}
	if !slices.IsSorted(families) {
		t.Error("Families() is not sorted")
	}
	if len(slices.Compact(slices.Clone(families))) != len(families) {
		t.Error("Families() has duplicates")
	}
	// The round trip: a listed family resolves through Best, and so
	// does its display-cased name - what the chooser hands back.
	tf, err := sysfont.Best(families[0], 13)
	if err != nil {
		t.Fatalf("Best(%q) failed: %v", families[0], err)
	}
	disp := sysfont.FamilyDisplay(families[0])
	if disp == "" {
		t.Fatal("FamilyDisplay returned an empty name")
	}
	if _, err := sysfont.Best(disp, 13); err != nil {
		t.Errorf("Best(display %q) failed: %v", disp, err)
	}
	_ = tf
}

// TestFontRowModelLaziness pins the virtualization contract: rows are
// built only on request, so a huge family list never parses more
// faces than the rows asked for.
func TestFontRowModelLaziness(t *testing.T) {
	fams := make([]string, 2000)
	for i := range fams {
		fams[i] = "family-not-installed-" + string(rune('a'+i%26)) + string(rune('0'+i%10))
	}
	m := &fontRowModel{face: testFace(t), fams: fams}
	if m.Len() != 2000 {
		t.Fatalf("Len = %d, want 2000", m.Len())
	}
	// Rows for absent families still build (the label falls back to the
	// chooser face) - the laziness is the point: nothing was parsed
	// until Row ran.
	if m.Row(0) == nil {
		t.Error("Row(0) returned nil")
	}
}

// FontFamilies lists display names that resolve back, sorted
// case-insensitively, without duplicates.
func TestFontFamilies(t *testing.T) {
	names, err := FontFamilies()
	if err != nil {
		t.Skipf("no system font store: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("no families on a system with fonts")
	}
	if !slices.IsSortedFunc(names, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) }) {
		t.Error("not sorted case-insensitively")
	}
	if len(slices.Compact(slices.Clone(names))) != len(names) {
		t.Error("duplicates")
	}
	if _, err := Font(names[0], 13); err != nil {
		t.Errorf("Font(%q): %v", names[0], err)
	}
	again, _ := FontFamilies()
	if &again[0] != &names[0] {
		t.Error("a second call resolved the list again")
	}
}

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

// The font chooser's family names start where the search field's text
// does, and the dialog keeps its width whatever size its preview line
// is shown at (#362).
func TestTheFontChoosersRowsLineUpWithItsSearchAndItsWidthHoldsStill(t *testing.T) {
	face, err := render.LoadFont(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	a := &Application{tooltipFace: face}
	lay := func(size float64) (natural int, search *widget.Entry, list *widget.List) {
		frame, err := a.dialogRoot(fontChooserDialog(face, []string{"alpha", "beta"}, FontChooserConfig{Size: size}), func(string) {})
		if err != nil {
			t.Fatal(err)
		}
		root := frame.root
		natural = root.Measure(widget.Row, widget.Unknown).Nat
		sz := widget.SizeWithin(root, widget.Size{W: 380, H: 600})
		h := max(sz.H, 420)
		root.Arrange(render.Rect{W: 380, H: h})
		root.Paint(render.New(make([]byte, render.Stride(380)*h), render.Stride(380), 380, h))
		var walk func(w widget.Widget)
		walk = func(w widget.Widget) {
			switch x := w.(type) {
			case *widget.Entry:
				if search == nil {
					search = x
				}
			case *widget.List:
				list = x
			}
			if c, ok := w.(interface{ Children() []widget.Widget }); ok {
				for _, k := range c.Children() {
					walk(k)
				}
			}
		}
		walk(root)
		return natural, search, list
	}
	small, search, list := lay(10)
	if search == nil || list == nil || len(list.Children()) == 0 {
		t.Fatalf("search found %v, list found %v", search != nil, list != nil)
	}
	slot := list.Children()[0].(widget.Boundser).Bounds()
	row := (&fontRowModel{face: face, fams: []string{"alpha"}}).Row(0)
	row.Arrange(slot)
	name := row.(interface{ Children() []widget.Widget }).Children()[0].(*widget.Label)
	if textX := search.Bounds().X + 12; name.Bounds().X != textX {
		t.Errorf("a family name starts at x=%d, want the search text's x=%d", name.Bounds().X, textX)
	}
	if large, _, _ := lay(40); large != small {
		t.Errorf("the dialog wants %dpx with a 40px preview, %dpx with a 10px one", large, small)
	}
}
