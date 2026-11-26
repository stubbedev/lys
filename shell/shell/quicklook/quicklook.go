// Package quicklook is Quick Look on a surface of its own: the shared
// preview of one file in a card centered on an output, over everything
// and holding the keyboard while it shows. Space, Escape or a press
// outside the card closes it. The screenshot thumbnail and the
// notifications show it, their own surfaces too small to preview on.
package quicklook

import (
	"context"
	"image"
	"log"
	"math"

	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/shell/layers"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/desktop/thumbnail"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/shell/shell/keys"
)

// The card's boxes, in logical pixels at scale 1: the picture's, the
// text's and the icon's.
const (
	imageW, imageH = 800, 600
	textH          = 480
	iconPx         = 128
	nameChars      = 56
)

// Window is the mapped layer surface (app.LayerWindow).
type Window interface{ Close() }

// Deps are Quick Look's collaborators.
type Deps struct {
	// Open maps the layer surface.
	Open func(app.LayerConfig) (Window, error)
	// KeySym reads a key code through the seat's keymap.
	KeySym keys.Translator
	// Invoke runs fn on the loop goroutine, Spawn off it.
	Invoke, Spawn func(fn func())
	// Font paints the card; Sheet styles it.
	Font  render.Font
	Sheet *widget.Stylesheet
	// Scale is the styling scale the card's sizes follow.
	Scale func() float64
	// MIME names files' types; Thumbnails makes what a file that is
	// neither picture, video nor text shows, nil showing its icon.
	MIME       *mime.Database
	Thumbnails *thumbnail.Thumbnailer
	// OpenFile opens the file shown in its default app, from the Open
	// button; nil shows no button.
	OpenFile func(path string)
}

// ForApp is Quick Look on a's surfaces, sized by scale, opening files
// in their default apps through a; it shows pictures, videos, text and
// folders, and any other file as its icon.
func ForApp(a *app.Application, font render.Font, sheet *widget.Stylesheet, scale func() float64) *QuickLook {
	return New(Deps{
		Open:     func(cfg app.LayerConfig) (Window, error) { return a.NewLayer(cfg) },
		KeySym:   a.KeySym,
		Invoke:   a.Invoke,
		Spawn:    func(fn func()) { go fn() },
		Font:     font,
		Sheet:    sheet,
		Scale:    scale,
		MIME:     mime.System(),
		OpenFile: Opener(a),
	})
}

// Opener opens a file in its default app through a, raised by an
// activation token for the press that asked; a failure is logged.
func Opener(a *app.Application) func(path string) {
	return func(path string) {
		a.OpenPath(path, func(err error) {
			if err != nil {
				log.Printf("quick look: open %s: %v", path, err)
			}
		})
	}
}

// QuickLook shows one file at a time; its methods run on the loop
// goroutine.
type QuickLook struct {
	d        Deps
	pictures *preview.Pictures
	cur      *look
}

// look is one file shown: its surface and its preview.
type look struct {
	win    Window
	view   *preview.View
	closed func()
}

// New builds Quick Look; nothing maps until Show.
func New(d Deps) *QuickLook {
	return &QuickLook{d: d, pictures: preview.NewPictures(preview.PictureBudget)}
}

// Show replaces whatever Quick Look shows with the file at path, on out
// (nil leaves the output to the compositor); closed runs once it no
// longer shows it, however it closed.
func (q *QuickLook) Show(path string, out *app.Output, closed func()) error {
	q.Close()
	l := &look{closed: closed}
	px := scaled(q.d.Scale())
	src := preview.Sources{MIME: q.d.MIME, Thumbnails: q.d.Thumbnails, Pictures: q.pictures}
	cfg := preview.Config{
		Font: q.d.Font, Class: "quicklook-card", NameChars: nameChars, Autoplay: true,
		Image: image.Pt(px(imageW), px(imageH)), Text: image.Pt(px(imageW), px(textH)), IconPx: px(iconPx),
		MIME: q.d.MIME, Invoke: q.d.Invoke, Spawn: q.d.Spawn,
		Load:    func(ctx context.Context, req preview.Request) preview.Content { return preview.Load(ctx, src, req) },
		OnClose: func() { q.closeLook(l) },
	}
	if open := q.d.OpenFile; open != nil {
		cfg.Open = func(path string) {
			q.closeLook(l)
			open(path)
		}
	}
	l.view = preview.New(cfg)
	l.view.Root().AddClass(css.Card)
	r := &root{card: l.view.Root()}
	if q.d.Sheet != nil {
		r.AttachStylesheet(q.d.Sheet)
	}
	win, err := q.d.Open(app.LayerConfig{
		Output:        out,
		Layer:         app.LayerOverlay,
		Anchor:        app.AnchorTop | app.AnchorBottom | app.AnchorLeft | app.AnchorRight,
		ExclusiveZone: -1,
		Keyboard:      app.KeyboardExclusive,
		Namespace:     layers.QuickLook.Name(),
		Root:          r,
		OnKey: keys.OnKey(q.d.KeySym, func(_ *widget.Router, sym xkb.Keysym, mods app.Mods) {
			if sym == xkb.KeyEscape || mods == 0 {
				l.view.Key(sym)
			}
		}),
		OnPress: func(_ uint32, over widget.Widget) {
			if over == widget.Widget(r) {
				q.closeLook(l)
			}
		},
		OnClosed: func() { q.closeLook(l) },
	})
	if err != nil {
		l.view.Close()
		closed()
		return err
	}
	l.win = win
	q.cur = l
	l.view.Show(path)
	return nil
}

// Look shows the file at path on the compositor's choice of output:
// Show for a host that names no output.
func (q *QuickLook) Look(path string, closed func()) error { return q.Show(path, nil, closed) }

// Shown is the file on screen, false when there is none.
func (q *QuickLook) Shown() (string, bool) {
	if q.cur == nil {
		return "", false
	}
	return q.cur.view.Path(), true
}

// Close takes Quick Look down.
func (q *QuickLook) Close() {
	if q.cur != nil {
		q.closeLook(q.cur)
	}
}

// closeLook takes l down, unless it already went.
func (q *QuickLook) closeLook(l *look) {
	if q.cur != l {
		return
	}
	q.cur = nil
	l.view.Close()
	l.win.Close()
	l.closed()
}

func scaled(scale float64) func(int) int {
	return func(n int) int { return int(math.Round(float64(n) * scale)) }
}

// Centered is w at its natural size, at most out's, centered in out.
func Centered(out render.Rect, w widget.Widget) render.Rect {
	width := min(w.Measure(widget.Row, widget.Unknown).Nat, out.W)
	height := min(w.Measure(widget.Column, width).Nat, out.H)
	return render.Rect{X: out.X + (out.W-width)/2, Y: out.Y + (out.H-height)/2, W: width, H: height}
}

// root covers the output, transparent, the card centered on it: a press
// on the root itself is a press outside the card.
type root struct {
	widget.Base
	card *widget.Box
	kids [1]widget.Widget
}

// Measure wants nothing: the surface is the output.
func (r *root) Measure(widget.Axis, int) widget.Sizes { return widget.Sizes{} }

func (r *root) Arrange(rect render.Rect) {
	r.ArrangeSelf(rect)
	r.card.Arrange(Centered(rect, r.card))
	widget.SetParents(r, r.Children()...)
}

// ArrangeRoot mirrors Arrange for the root path.
func (r *root) ArrangeRoot(rect render.Rect) { r.Arrange(rect) }

func (r *root) Paint(cv *render.Canvas) { widget.PaintChild(cv, r.card) }

func (r *root) Children() []widget.Widget { r.kids[0] = r.card; return r.kids[:] }

// BackdropRegion is the card's rounded box: what the compositor blurs.
func (r *root) BackdropRegion() []render.Rect {
	return render.RoundedRectRegion(r.card.Bounds(), widget.CornerRadii(r.card))
}

func (r *root) HitTest(p widget.Point) widget.Widget {
	if hit := r.card.HitTest(p); hit != nil {
		return hit
	}
	return r.HitLeaf(r, p)
}
