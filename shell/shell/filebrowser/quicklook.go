package filebrowser

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/desktop/shell/reveal"
	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/shell/shell/quicklook"
)

// The previews' boxes in logical pixels: Quick Look's least, before it
// knows the views' area, and the pane's, whose width the pane gives.
// Quick Look's box is the views' area but for quicklookShare of it
// left around the card and quicklookChrome over the content for the
// title strip; text is at most quicklookTextW wide, a page's width.
const (
	quicklookMinW, quicklookMinH = 240, 160
	quicklookChrome              = 56
	quicklookTextW               = 720
	paneImageH, paneTextH        = 180, 260
)

// quicklookShare is the part of the views' area Quick Look's content
// may take along each axis.
const quicklookShare = 0.85

// newQuicklook is Quick Look's card, in the frame that lays it over the
// file views: a preview of the selection, a document's pages scrolling,
// closed by Space, Escape and a click beside it, stepped through the view by the arrows
// (quicklookArrow), playing a video at once, opening the file in its
// default app from its Open button.
func (c *Browser) newQuicklook() (*preview.View, *quicklookFrame) {
	least := image.Pt(quicklookMinW, quicklookMinH)
	v := preview.New(c.previewConfig(preview.Config{
		Class: "file-chooser-quicklook", NameChars: 48, Autoplay: true, Bare: true, Pages: true,
		Image: least, Text: least, OnClose: c.closeQuicklook, Open: c.d.OpenFile,
	}))
	v.Root().AddClass(css.Card)
	rev := widget.NewRevealer(v.Root())
	rev.SetFadeFrom(reveal.FadeFrom)
	f := &quicklookFrame{rev: rev, outside: c.closeQuicklook, fit: func(area render.Rect) {
		box := quicklookBox(area)
		v.Fit(box, image.Pt(min(box.X, quicklookTextW), box.Y))
	}}
	f.SetVisible(false)
	return v, f
}

// quicklookOpen reports Quick Look open: showing, or entering, and not
// leaving.
func (c *Browser) quicklookOpen() bool { return c.ui.quicklook.rev.Revealed() }

// quicklookKey handles a plain key while Quick Look shows: its own
// keys, then the arrows stepping through the view. It reports whether
// it handled the key.
func (c *Browser) quicklookKey(r *widget.Router, sym xkb.Keysym) bool {
	return c.quicklookOpen() && (c.ui.ql.Key(sym) || c.quicklookArrow(r, sym))
}

func (c *Browser) toggleQuicklook() {
	if c.quicklookOpen() {
		c.closeQuicklook()
		return
	}
	if _, ok := c.selected(); ok {
		c.ui.quicklook.SetVisible(true)
		reveal.Show(c.ui.quicklook.rev, c.quicklookMotion(), config.AnimLauncher)
		c.refreshPreview()
	}
}

// closeQuicklook plays Quick Look's exit, then hides it and lets go of
// what it showed, unless it opened again meanwhile.
func (c *Browser) closeQuicklook() {
	f := c.ui.quicklook
	if !f.Visible() {
		return
	}
	reveal.Hide(f.rev, c.quicklookMotion(), config.AnimLauncher, c.dropQuicklook)
}

// dropQuicklook hides Quick Look at once and lets go of what it showed.
func (c *Browser) dropQuicklook() {
	f := c.ui.quicklook
	f.rev.SetOnTransitionDone(nil)
	f.rev.SetDuration(0)
	f.rev.SetRevealed(false)
	f.SetVisible(false)
	c.ui.ql.Close()
}

// quicklookMotion is how Quick Look moves: as the launcher does, and at
// once while the toolkit's motion is reduced or off.
func (c *Browser) quicklookMotion() config.AnimationsConfig {
	a := c.d.Animations()
	if anim.Instant() {
		a.Enabled = false
	}
	return a
}

// refreshPreview shows the selected entry in the previews showing, or
// says nothing is selected.
func (c *Browser) refreshPreview() {
	if c.ui == nil || len(c.openPreviews()) == 0 {
		return
	}
	e, ok := c.selected()
	if !ok {
		c.closeQuicklook()
		c.ui.pv.Close()
		return
	}
	for _, v := range c.openPreviews() {
		if vfs.IsRemote(e.Path) && !e.IsDir && e.Size <= remotePreviewMax && strings.HasPrefix(c.d.MIME.TypeByName(e.Name()), "image/") {
			v.ShowPicture(preview.Memory{Name: e.Name(), Read: c.remoteReader(e.Path)})
			continue
		}
		v.Show(e.Path)
	}
}

// remotePreviewMax is the largest picture on a network location the
// previews fetch to show.
const remotePreviewMax = 32 << 20

// remoteReader reads the file at loc whole through its filesystem, for
// a preview off the loop.
func (c *Browser) remoteReader(loc string) func() ([]byte, error) {
	mounts := c.mounts
	return func() ([]byte, error) {
		fsys, p, err := mounts.Resolve(loc)
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
		defer cancel()
		f, err := fsys.Open(ctx, p)
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(f, remotePreviewMax+1))
		if err = errors.Join(err, f.Close()); err != nil {
			return nil, fmt.Errorf("read %s: %w", loc, err)
		}
		if len(data) > remotePreviewMax {
			return nil, fmt.Errorf("read %s: past %d bytes", loc, remotePreviewMax)
		}
		return data, nil
	}
}

// quicklookBox is the box Quick Look's content fits into over views
// laid out in area.
func quicklookBox(area render.Rect) image.Point {
	return image.Pt(
		max(quicklookMinW, int(float64(area.W)*quicklookShare)),
		max(quicklookMinH, int(float64(area.H)*quicklookShare)-quicklookChrome),
	)
}

// quicklookFrame covers the file views and centers Quick Look's card,
// in the revealer playing its enter and exit, on them at the card's own
// size, its content fitted to the views' area (fit). A click beside the
// open card runs outside; while the card leaves, presses pass through
// to the views.
type quicklookFrame struct {
	widget.Base
	rev     *widget.Revealer
	fit     func(area render.Rect)
	outside func()
	kids    [1]widget.Widget
}

// Measure wants nothing: the frame is the views' area.
func (f *quicklookFrame) Measure(widget.Axis, int) widget.Sizes { return widget.Sizes{} }

func (f *quicklookFrame) Arrange(r render.Rect) {
	f.ArrangeSelf(r)
	f.fit(r)
	f.rev.Arrange(quicklook.Centered(r, f.rev))
	widget.SetParents(f, f.Children()...)
}

// ArrangeRoot mirrors Arrange for the root path.
func (f *quicklookFrame) ArrangeRoot(r render.Rect) { f.Arrange(r) }

func (f *quicklookFrame) Paint(cv *render.Canvas) { widget.PaintChild(cv, f.rev) }

func (f *quicklookFrame) Children() []widget.Widget { f.kids[0] = f.rev; return f.kids[:] }

// HitTest is the card's, else the frame itself while the card is open:
// a click beside it.
func (f *quicklookFrame) HitTest(p widget.Point) widget.Widget {
	if hit := f.rev.HitTest(p); hit != nil {
		return hit
	}
	if f.rev.Revealed() {
		return f.HitLeaf(f, p)
	}
	return nil
}

// ClickAt is a click beside the card.
func (f *quicklookFrame) ClickAt(widget.Point) { f.outside() }

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
	return preview.Load(ctx, preview.Sources{MIME: c.d.MIME, Thumbnails: c.thumbnailer, Pictures: c.pictures, Documents: c.d.Documents}, req)
}

// openPreviews are the previews showing.
func (c *Browser) openPreviews() []*preview.View {
	var open []*preview.View
	if c.quicklookOpen() {
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
	c.dropQuicklook()
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
