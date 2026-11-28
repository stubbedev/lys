package launcherview

import (
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/desktop/shell/treetest"
)

func TestRowDisplayColumnsAndEllipsize(t *testing.T) {
	d := RowDisplay{Columns: []uint32{2, 3, 9}, Separator: "|"}
	if got := d.shown("a|b|c"); got != "b c" {
		t.Errorf("columns = %q", got)
	}
	if (RowDisplay{}).shown("a|b") != "a|b" {
		t.Error("no columns keeps the text")
	}
	if (RowDisplay{Ellipsize: EllipsizeStart}).mode() != widget.EllipsizeStart || (RowDisplay{}).mode() != widget.EllipsizeEnd {
		t.Error("ellipsize modes")
	}
}

func rowStyle(t *testing.T) RowStyle {
	return RowStyle{Font: treetest.Font(t), IconPx: 24, Post: func(fn func()) { fn() }}
}

// A session's rows are the search panel's row box, carrying the selected
// state.
func TestASessionsRowsAreTheSharedRowBox(t *testing.T) {
	row := NewRow(Row{Text: "alpha"}, true, rowStyle(t))
	if !row.Content.HasClass("launcher-search-row") || !row.Content.HasState(widget.StateSelected) || !row.HasState(widget.StateSelected) {
		t.Error("a selected row is not a selected .launcher-search-row")
	}
	if idle := NewRow(Row{Text: "beta"}, false, rowStyle(t)); idle.Content.HasState(widget.StateSelected) {
		t.Error("an unselected row carries the selected state")
	}
}

func TestARowCarriesItsStatesAndBallot(t *testing.T) {
	ballot := "☑ "
	row := NewRow(Row{Text: "alpha", Urgent: true, Unselectable: true, Ballot: &ballot}, false, rowStyle(t))
	if !row.Content.HasClass("urgent") || !row.Content.HasClass("nonselectable") || row.Content.HasClass("active") {
		t.Errorf("an urgent unselectable row has classes %v", row.Content.Classes())
	}
	if labels := treetest.Labels(row); !slices.Equal(labels, []string{ballot, "alpha"}) {
		t.Errorf("a ballot row reads %v", labels)
	}
	if labels := treetest.Labels(NewRow(Row{Text: "alpha"}, false, rowStyle(t))); !slices.Equal(labels, []string{"alpha"}) {
		t.Errorf("a row of a single-pick session reads %v", labels)
	}
}

func TestAMarkupRowThatDoesNotParseShowsTheLiteral(t *testing.T) {
	row := NewRow(Row{Text: "<b>broken", Markup: true}, false, rowStyle(t))
	labels := treetest.All[*widget.RichLabel](row)
	if len(labels) != 1 || labels[0].Text() != "<b>broken" {
		t.Errorf("broken markup reads %v", treetest.Labels(row))
	}
	row = NewRow(Row{Text: "<b>bold</b>", Markup: true}, false, rowStyle(t))
	if labels := treetest.All[*widget.RichLabel](row); len(labels) != 1 || labels[0].Text() != "bold" {
		t.Errorf("markup reads %v", treetest.Labels(row))
	}
}

func TestARowShowsItsIconOnlyWhenIconsShow(t *testing.T) {
	treetest.IconTheme(t, "launcherview-test-app")
	st := rowStyle(t)
	if icons := treetest.All[*widget.Icon](NewRow(Row{Text: "a", Icon: IconName("launcherview-test-app")}, false, st)); len(icons) != 0 {
		t.Error("a row drew an icon with icons off")
	}
	st.ShowIcons = true
	row := NewRow(Row{Text: "a", Icon: IconName("launcherview-test-app")}, false, st)
	if icons := treetest.All[*widget.Icon](row); len(icons) != 1 || !icons[0].HasClass("launcher-row-icon") {
		t.Fatalf("a row with icons on drew %d icons", len(icons))
	}
	row.ShowIcon(IconName("launcherview-test-other"))
	if icons := treetest.All[*widget.Icon](row); len(icons) != 1 {
		t.Errorf("a replaced icon left %d icons", len(icons))
	}
}

// A desktop entry names a specific icon a theme may lack: the row
// resolves it through GTK's dash fallbacks (preferences-system-network
// to preferences-system), as a GAppInfo icon does, instead of drawing
// nothing.
func TestRowIconFallsBackAlongTheName(t *testing.T) {
	treetest.IconTheme(t, "preferences-system")
	post := func(fn func()) { fn() }
	if ink := treetest.Ink(iconWidget(IconName("preferences-system-network"), 24, post)); ink == 0 {
		t.Error("the row drew no icon for a name whose dash-shortened form exists")
	}
	if ink := treetest.Ink(iconWidget(IconName("unrelated-app"), 24, post)); ink != 0 {
		t.Error("a name with no present form drew an icon")
	}
}

func TestARowsFileIconPointingAtAFIFODrawsNothingAndDoesNotBlock(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "icon.png")
	if err := unix.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	posted := make(chan func(), 4)
	icon := iconWidget(IconFile(fifo), 24, func(fn func()) { posted <- fn })
	const quietWindow = 200 * time.Millisecond
	for quiet := false; !quiet; {
		select {
		case fn := <-posted:
			fn()
		case <-time.After(quietWindow):
			quiet = true
		}
	}
	if ink := treetest.Ink(icon); ink != 0 {
		t.Errorf("the row drew %d pixels for a file that is not one", ink)
	}
}

// laidOut is row arranged at its natural height in a row w wide.
func laidOut(row *RowView, w int) render.Rect {
	h := row.Measure(widget.Column, w).Nat
	row.Arrange(render.Rect{W: w, H: h})
	return render.Rect{W: w, H: h}
}

func centreY(r render.Rect) int { return r.Y + r.H/2 }

// A row's secondary text follows its name on the same line, smaller and
// dimmed, read as plain text, and leaves the row as tall as one without.
func TestARowsSecondaryTextIsDimSmallerAndOnTheNamesLine(t *testing.T) {
	st := rowStyle(t)
	plain := NewRow(Row{Text: "budget.ods"}, false, st)
	row := NewRow(Row{Text: "budget.ods", Secondary: "~/<i>notes</i>"}, false, st)
	if got, want := laidOut(row, 600).H, laidOut(plain, 600).H; got != want {
		t.Errorf("a row with secondary text is %dpx tall, want %dpx as without", got, want)
	}
	if labels := treetest.Labels(row); !slices.Equal(labels, []string{"budget.ods", "~/<i>notes</i>"}) {
		t.Fatalf("the row reads %v, want its name then the secondary text verbatim", labels)
	}
	secondary := treetest.LabelReading(t, row, "~/<i>notes</i>")
	name := treetest.LabelReading(t, row, "budget.ods")
	if !secondary.HasClass("launcher-row-secondary") || !secondary.HasClass(css.DimLabel) || name.HasClass(css.DimLabel) {
		t.Errorf("classes: secondary %v, name %v; want only the secondary dimmed", secondary.Classes(), name.Classes())
	}
	if got, want := treetest.RenderedTextPx(secondary, st.Font), SmallPx(); math.Abs(got-want) > 0.02*want {
		t.Errorf("the secondary text renders at %.2fpx, want the small %.2fpx", got, want)
	}
	if got, body := treetest.RenderedTextPx(name, st.Font), FontPx(); math.Abs(got-body) > 0.02*body {
		t.Errorf("the name renders at %.2fpx, want the body %.2fpx", got, body)
	}
	nb, sb := name.Bounds(), secondary.Bounds()
	if sb.X < nb.X+nb.W || math.Abs(float64(centreY(sb)-centreY(nb))) > 1 {
		t.Errorf("the secondary text at %v does not follow the name at %v on its line", sb, nb)
	}
	if pb := treetest.LabelReading(t, plain, "budget.ods").Bounds(); math.Abs(float64(centreY(pb)-centreY(nb))) > 1 {
		t.Errorf("the name sits at %v, %v without secondary text", nb, pb)
	}
}

func TestALongSecondaryTextEllipsizesAndRowsWithoutOneShowNone(t *testing.T) {
	st := rowStyle(t)
	long := "~/" + strings.Repeat("deeply/nested/", 40)
	row := NewRow(Row{Text: "report.pdf", Secondary: long}, false, st)
	frame := laidOut(row, 400)
	secondary := treetest.LabelReading(t, row, long)
	if b := secondary.Bounds(); b.X+b.W > frame.W {
		t.Errorf("a long secondary text runs to %d past the row's %d", b.X+b.W, frame.W)
	}
	if name := treetest.LabelReading(t, row, "report.pdf"); name.Bounds().W < name.Measure(widget.Row, widget.Unknown).Nat {
		t.Errorf("a long secondary text squeezed the name to %dpx", name.Bounds().W)
	}
	for _, r := range []Row{{Text: "plain"}, {Text: "<b>bold</b>", Markup: true}} {
		if got := treetest.WithClass(NewRow(r, false, st), "launcher-row-secondary"); len(got) != 0 {
			t.Errorf("a row without secondary text %+v holds %d secondary labels", r, len(got))
		}
	}
}
