package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// A popover hosts its content outside the window's tree, where the
// app's stylesheet does not reach unless put there: unstyled, its setting
// rows take lys-ui's bare look, unlike the page's. Styled as a picker's
// content, a row measures as it does on the page.
func TestAPickersRowsReadLikeThePagesRows(t *testing.T) {
	k := testKit(t, "")
	theme := apptheme.New(k.store.svc.Config())
	measure := func(style func(widget.Widget)) widget.Sizes {
		content := gapBox(widget.Column)
		row := k.row("Current password", "").withControl(k.entry())
		content.Append(row, false)
		style(content)
		content.Arrange(render.Rect{W: passwordDialogWidth, H: 400})
		return row.Measure(widget.Row, -1)
	}
	page := measure(func(w widget.Widget) { theme.Attach(w.(*widget.Box)) })
	picker := measure(windowPickers{theme: theme}.style)
	if picker != page {
		t.Errorf("a picker's row measures %+v, the page's %+v", picker, page)
	}
	bare := measure(func(widget.Widget) {})
	if bare == page {
		t.Fatalf("premise: an unstyled row measures as a styled one, %+v", bare)
	}
}
