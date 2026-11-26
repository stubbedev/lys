package filebrowser

import (
	"context"
	"image"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/widget"
)

// The previews' boxes in logical pixels: Quick Look's picture and text,
// and the pane's, whose width the pane gives.
const (
	quicklookImageW, quicklookImageH = 480, 360
	quicklookTextW, quicklookTextH   = 480, 320
	paneImageH, paneTextH            = 180, 260
)

// newQuicklook is Quick Look's card: a preview of the selection over
// the file views, closed by Space and Escape, stepped through the view
// by the arrows (quicklookArrow), playing a video at once, opening the
// file in its default app from its Open button.
func (c *Browser) newQuicklook() *preview.View {
	return preview.New(c.previewConfig(preview.Config{
		Class: "file-chooser-quicklook", NameChars: 32, Autoplay: true,
		Image: image.Pt(quicklookImageW, quicklookImageH), Text: image.Pt(quicklookTextW, quicklookTextH),
		OnClose: c.closeQuicklook, Open: c.d.OpenFile,
	}))
}

// newPreviewPane is the preview pane beside the file views, w wide,
// saying so while nothing is selected.
func (c *Browser) newPreviewPane(w int) *preview.View {
	return preview.New(c.previewConfig(preview.Config{
		Class: "file-chooser-preview", NameChars: 22, Placeholder: i18n.T("file-chooser-no-selection"),
		Image: image.Pt(w, paneImageH), Text: image.Pt(w, paneTextH),
	}))
}

// previewConfig is cfg with what every preview of the chooser shares.
func (c *Browser) previewConfig(cfg preview.Config) preview.Config {
	cfg.Font, cfg.MIME, cfg.IconPx = c.d.Font, c.d.MIME, previewIconPx
	cfg.Invoke, cfg.Spawn = c.d.Invoke, c.offLoop.Go
	cfg.Load = func(ctx context.Context, req preview.Request) preview.Content { return c.previewLoader(ctx, req) }
	return cfg
}

// loadPreview finds out what to show of a file through the chooser's
// thumbnailer and pictures; safe off the loop.
func (c *Browser) loadPreview(ctx context.Context, req preview.Request) preview.Content {
	return preview.Load(ctx, preview.Sources{MIME: c.d.MIME, Thumbnails: c.thumbnailer, Pictures: c.pictures}, req)
}

// openPreviews are the previews showing.
func (c *Browser) openPreviews() []*preview.View {
	var open []*preview.View
	if c.ui.quicklook.Visible() {
		open = append(open, c.ui.ql)
	}
	if c.ui.preview.Visible() {
		open = append(open, c.ui.pv)
	}
	return open
}

// closePreviews closes both previews, which lets go of what they show.
func (c *Browser) closePreviews() {
	if c.ui == nil {
		return
	}
	c.ui.ql.Close()
	c.ui.pv.Close()
}

// quicklookArrow steps Quick Look through the active view on an arrow
// key, as the view's own arrows move: a row of the grid for Up and Down
// and one item for Left and Right, one item of the list for every
// arrow. The view takes the arrows it navigates itself while it holds
// r's focus, so only the rest move it here. It reports whether sym is
// an arrow.
func (c *Browser) quicklookArrow(r *widget.Router, sym xkb.Keysym) bool {
	var grid, list widget.KeyAction
	switch sym {
	case xkb.KeyUp:
		grid, list = widget.KeyUp, widget.KeyUp
	case xkb.KeyDown:
		grid, list = widget.KeyDown, widget.KeyDown
	case xkb.KeyLeft:
		grid, list = widget.KeyLeft, widget.KeyUp
	case xkb.KeyRight:
		grid, list = widget.KeyRight, widget.KeyDown
	default:
		return false
	}
	if r.Focused() == c.focusView() && (c.s.view.grid || grid == list) {
		return true
	}
	if !c.s.view.grid {
		grid = list
	}
	c.activeView().KeyAction(grid, 0)
	return true
}
