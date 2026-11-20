package launcherview

import (
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
)

// Spec is what a session's panel is built from.
type Spec struct {
	Font     render.Font
	Variants widget.VariantFunc
	Config   *config.Config
	Look     Look
	// Row builds the row at a list position as it comes into view.
	Row func(pos int) widget.Widget
	// Side sits beside the list, nil for nothing.
	Side widget.Widget
	// OnQuery hears every edit of the field.
	OnQuery func(string)
}

// Session is a rofi session's panel content: the field, the message, the
// list with whatever sits beside it, the mode tabs and the footer.
type Session struct {
	*Chrome
	Content *widget.Box
	List    *List
	Frame   *widget.Box
	Tabs    *widget.Box
	look    Look
	font    render.Font
}

// NewSession builds the panel's content for spec, showing no rows.
func NewSession(spec Spec) *Session {
	px := ScaledPx(spec.Config)
	th := widget.Current()
	s := &Session{Chrome: NewChrome(spec.Font, spec.Variants, spec.Config, spec.OnQuery), look: spec.Look, font: spec.Font}
	s.SearchIcon.Show(SearchIconName)
	s.List = NewList(px(SearchRowHeight), RowMargin(px), spec.Look.Lines, spec.Look.Fixed, spec.Row)
	s.Frame = widget.NewBox(widget.Column, 0, 0)
	s.Frame.AddClass("launcher-list")
	s.Frame.Append(s.List, true)
	body := widget.NewBox(widget.Row, 0, 0)
	body.Append(s.Frame, true)
	if spec.Side != nil {
		body.Append(spec.Side, false)
	}

	s.Tabs = widget.NewBox(widget.Row, 0, 0)
	s.Tabs.SetElement("togglegroup")
	s.Tabs.AddClass("launcher-tabs")
	s.Tabs.SetMargin(render.Insets{Left: th.Padding, Right: th.Padding, Bottom: th.Padding})
	s.Tabs.SetHomogeneous(true)
	s.Tabs.SetVisible(spec.Look.Sidebar)

	s.Content = widget.NewBox(widget.Column, 0, 0)
	s.Content.Append(s.Input, false)
	s.Content.Append(s.Message, false)
	s.Content.Append(s.Sep, false)
	s.Content.Append(body, true)
	s.Content.Append(s.Tabs, false)
	s.Content.Append(s.Footer, false)
	return s
}

// Width is the panel's width before an output holds it in.
func (s *Session) Width() int { return s.look.Width }

// SetRows shows n rows, and the separator and the footer's hints while
// there are rows to act on, as the search panel does.
func (s *Session) SetRows(n int) {
	s.List.Reset(n)
	s.Sep.SetVisible(n > 0)
	s.Footer.SetVisible(n > 0)
	if n > 0 {
		s.ShowHints(FooterHint{"Enter", i18n.T("launcher-footer-select")}, CloseHint())
	}
}

// SetTabs makes one tab per mode, named as names says, the active one
// marked; pick hears which is clicked.
func (s *Session) SetTabs(modes []string, names map[string]string, active int, pick func(int)) {
	s.Tabs.Clear()
	for i, mode := range modes {
		label := mode
		if shown, ok := names[mode]; ok {
			label = shown
		}
		b := widget.NewButton(widget.NewLabel(s.font, FontPx(), label, 0), 0, 0)
		b.AddClass("launcher-tab")
		if i == active {
			b.AddClass("active")
			b.SetState(widget.StateChecked, true)
		}
		b.OnClick = func() { pick(i) }
		s.Tabs.Append(b, true)
	}
}

// TargetIn is the panel's size in an output, extra pixels wider than the
// look for what sits beside the list: as tall as its content, up to the
// room below where the panel sits.
func (s *Session) TargetIn(out render.Rect, extra int) (w, h int) {
	w = WidthIn(out, s.look.Width+extra)
	h = s.Content.Measure(widget.Column, w).Nat
	if out.H > 0 {
		h = min(h, RoomBelow(out))
	}
	return w, max(h, 1)
}

// PlaceIn is where the look puts a panel w by h in an output.
func (s *Session) PlaceIn(out render.Rect, w, h int) render.Rect {
	return Place(out, w, h, s.look.Location, s.look.OffsetX, s.look.OffsetY)
}
