package launcherview

import (
	"strings"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/desktop/pango"
	"github.com/stubbedev/lys/desktop/shell/widgets"
)

// Icon is what a row shows beside its text: an IconName or an IconFile.
type Icon interface{ rowIcon() }

// IconName is an icon theme name.
type IconName string

// IconFile is an image file's path.
type IconFile string

func (IconName) rowIcon() {}
func (IconFile) rowIcon() {}

// Row is what one list row shows.
type Row struct {
	Text string
	// Markup reads Text as Pango markup.
	Markup bool
	// Secondary is plain text after Text, smaller and dimmed: what a
	// row says about itself besides its name (a file's folder).
	Secondary string
	// Icon is nil for none.
	Icon                         Icon
	Urgent, Active, Unselectable bool
	// Ballot marks a row of a session that takes several, nil otherwise.
	Ballot *string
}

// Ellipsize is where a long row truncates.
type Ellipsize int

// Ellipsize values.
const (
	EllipsizeEnd Ellipsize = iota
	EllipsizeStart
	EllipsizeMiddle
)

// RowDisplay is which columns of a row show, split on what, and where a
// long row truncates.
type RowDisplay struct {
	// Columns are 1-based; nil shows the whole text.
	Columns   []uint32
	Separator string
	Ellipsize Ellipsize
}

// shown keeps the configured columns, joined by a space.
func (d RowDisplay) shown(text string) string {
	if d.Columns == nil {
		return text
	}
	parts := strings.Split(text, d.Separator)
	var kept []string
	for _, c := range d.Columns {
		if i := int(c) - 1; c > 0 && i < len(parts) {
			kept = append(kept, parts[i])
		}
	}
	return strings.Join(kept, " ")
}

func (d RowDisplay) mode() widget.EllipsizeMode {
	switch d.Ellipsize {
	case EllipsizeStart:
		return widget.EllipsizeStart
	case EllipsizeMiddle:
		return widget.EllipsizeMiddle
	case EllipsizeEnd:
	}
	return widget.EllipsizeEnd
}

// RowStyle is what every row in a session is built with.
type RowStyle struct {
	Font      render.Font
	Variants  widget.VariantFunc
	IconPx    int
	ShowIcons bool
	Display   RowDisplay
	// Post runs a function on the UI loop, for an icon decoded off it.
	Post func(func())
}

// RowView is one list row: GTK's listview `row` element (selected rows
// carry :selected) around the .launcher-search-row box. It is the hit
// leaf, so its pointer hook sees every press on it.
type RowView struct {
	*widget.Box
	Content  *widget.Box
	iconSlot *widget.Box
	style    RowStyle
}

// HitTest makes the row the leaf the pointer lands on.
func (r *RowView) HitTest(p widget.Point) widget.Widget { return r.HitLeaf(r, p) }

// NewRow builds row as the list shows it, selected or not.
func NewRow(row Row, selected bool, st RowStyle) *RowView {
	content := NewRowBox()
	content.SetState(widget.StateSelected, selected)
	for _, flag := range []struct {
		on    bool
		class string
	}{{row.Urgent, "urgent"}, {row.Active, "active"}, {row.Unselectable, "nonselectable"}} {
		if flag.on {
			content.AddClass(flag.class)
		}
	}
	if row.Ballot != nil {
		b := widget.NewLabel(st.Font, FontPx(), *row.Ballot, 0)
		b.AddClass("launcher-row-ballot")
		content.Append(b, false)
	}
	r := &RowView{Box: widget.NewBox(widget.Row, 0, 0), Content: content, iconSlot: widget.NewBox(widget.Row, 0, 0), style: st}
	if st.ShowIcons {
		if row.Icon != nil {
			r.iconSlot.Append(iconWidget(row.Icon, st.IconPx, st.Post), false)
		}
		content.Append(r.iconSlot, false)
	}
	if row.Secondary == "" {
		content.Append(rowLabel(row, st), true)
	} else {
		content.AppendAligned(rowLabel(row, st), false, widget.AlignCenter)
		content.AppendAligned(secondaryLabel(row.Secondary, st), true, widget.AlignCenter)
	}
	r.SetElement("row")
	r.SetState(widget.StateSelected, selected)
	r.Append(content, true)
	return r
}

// ShowIcon replaces the row's icon, as when a thumbnail lands.
func (r *RowView) ShowIcon(icon Icon) {
	r.iconSlot.Clear()
	r.iconSlot.Append(iconWidget(icon, r.style.IconPx, r.style.Post), false)
}

func rowLabel(row Row, st RowStyle) widget.Widget {
	shown := st.Display.shown(row.Text)
	if !row.Markup {
		l := widget.NewLabel(st.Font, FontPx(), shown, 0)
		l.SetEllipsize(st.Display.mode())
		l.AddClass("launcher-row-label")
		return l
	}
	markup, ok := pango.Translate(shown)
	if !ok {
		markup = pango.Escape(shown)
	}
	rl := widget.NewRichLabel(st.Font, FontPx(), markup, 0)
	rl.SetVariants(st.Variants)
	rl.SetEllipsize(st.Display.mode())
	rl.AddClass("launcher-row-label")
	return rl
}

func secondaryLabel(text string, st RowStyle) *widget.Label {
	l := widget.NewLabel(st.Font, SmallPx(), text, 0)
	l.SetEllipsize(st.Display.mode())
	l.AddClass("launcher-row-secondary", css.DimLabel)
	return l
}

// iconWidget is the icon a row shows: a theme name, or a file decoded off
// the loop.
func iconWidget(icon Icon, px int, post func(func())) *widget.Icon {
	var w *widget.Icon
	switch ic := icon.(type) {
	case IconName:
		w = widget.NewThemeIconFallback(string(ic), px)
	case IconFile:
		w = widgets.NewAsyncFileIcon(string(ic), "", px, post, nil)
	}
	w.AddClass("launcher-row-icon")
	return w
}
