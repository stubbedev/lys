package launcher

import (
	"github.com/stubbedev/lys/desktop/shell/launcherview"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/launcheripc"
)

// views is one session's widget tree: the frame every session has,
// around the panel a rofi session shows and its preview pane.
type views struct {
	*panelFrame
	*launcherview.Session
	pane *previewPane
	ql   *quickLook
}

// buildViews builds and fills the tree for session a.
func (s *Surface) buildViews(a *active) *views {
	ui := a.ui
	cfg := s.d.Config()
	px := launcherview.ScaledPx(cfg)
	look := ui.look
	look.Width = s.resolveWidth(ui)
	st := launcherview.RowStyle{
		Post: s.d.Invoke, Font: s.d.Font, Variants: s.d.Variants,
		IconPx: px(launcherview.SearchIconSize), ShowIcons: look.ShowIcons, Display: ui.display,
	}
	v := &views{pane: newPreviewPane(s.d.Font, px, 0, s.d.Invoke, s.work.Go)}
	v.Session = launcherview.NewSession(launcherview.Spec{
		Font: s.d.Font, Variants: s.d.Variants, Config: cfg, Look: look, Side: v.pane.box,
		Row: func(pos int) widget.Widget { return s.makeRow(a, pos, st) },
		OnQuery: func(text string) {
			if s.cur == a {
				s.send(cmdSetQuery{query: text})
			}
		},
	})
	if ui.filter != nil {
		v.Entry.SetText(*ui.filter)
	}
	v.SetMasked(ui.password)
	v.pane.box.height = previewMinRows*v.List.RowH() + 2*v.List.Edge()
	v.List.SetOnScroll(func(e widget.ScrollEvent) bool { return s.listScrolled(e.DY, mouseMods(e.Mods)) })

	v.panelFrame = newPanelFrame(s.d, v.Content, launcherview.WidthIn(render.Rect{}, v.Width()), px(launcherview.InputRowHeight), func() Window { return a.win })
	if sheet, ok := look.Stylesheet(s.d.SheetPriority); ok {
		v.root.AttachStylesheet(sheet)
	}
	v.ql = newQuickLook(s.d.Font, px, s.d.Invoke, s.work.Go, lookOpener(s.d.OpenFile, func() {
		if s.cur == a {
			s.end(&launcheripc.ServerFrame{Type: launcheripc.FrameCancelled, Code: 1})
		}
	}), v.overlayMoved)
	v.overlay = v.ql.card
	v.target = v.targetIn
	v.place = v.PlaceIn

	v.SetChip(deref(ui.prompt, ""))
	message := ui.errorMessage
	if message == nil {
		message = ui.mesg
	}
	v.showMessage(message)
	v.Input.SetVisible(!a.dialog)
	v.Frame.SetVisible(!a.dialog)
	v.SetRows(0)
	return v
}

// makeRow is the row factory for list position pos; a thumbnail still
// being made lands on the row if it is still showing that file.
func (s *Surface) makeRow(a *active, pos int, st launcherview.RowStyle) widget.Widget {
	it, _ := s.model.item(pos)
	index, _ := s.model.itemIndex(pos)
	row := launcherview.NewRow(viewRow(it, s.multi.ballot(index)), pos == s.sel, st)
	row.SetOnPointer(func(e widget.PointerEvent) bool {
		s.rowPressed(pos, gdkButton(e.Button), e.Clicks, mouseMods(e.Mods))
		return true
	})
	if path := thumbnailPending(it.Icon); path != "" && st.ShowIcons {
		generateThumbnail(a.ctx, s.work.Go, a.thumbs, path, func(made string) {
			s.d.Invoke(func() {
				if shown, ok := a.views.List.Row(pos); s.cur != a || !ok || shown != widget.Widget(row) {
					return
				}
				row.ShowIcon(launcherview.IconFile(made))
			})
		})
	}
	return row
}

// showMessage shows the message line, hidden without one.
func (v *views) showMessage(markup *string) { v.SetMessage(deref(markup, "")) }

// targetIn is the panel's size in an output, the preview pane beside the
// list while it shows.
func (v *views) targetIn(out render.Rect) (w, h int) {
	extra := 0
	if v.pane.box.Visible() {
		extra = v.pane.width
	}
	return v.TargetIn(out, extra)
}

// showPane shows the preview pane beside the list, previewMinRows rows
// tall so a short list still has room to preview.
func (v *views) showPane(on bool) {
	if on == v.pane.box.Visible() {
		return
	}
	v.pane.box.SetVisible(on)
}
