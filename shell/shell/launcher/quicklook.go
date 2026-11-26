package launcher

import (
	"context"
	"image"
	"log"
	"slices"
	"sync/atomic"

	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/thumbnail"
	"github.com/stubbedev/lys/lib/exif"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	engine "github.com/stubbedev/lys/shell/service/launcher"
)

// Quick Look's boxes, in logical pixels at scale 1: the picture's, the
// text's, the icon's and the card's padding. They are twice the side
// pane's, so Quick Look shows what the pane only suggests.
const (
	quickLookImageW, quickLookImageH = 720, 540
	quickLookTextH                   = 480
	quickLookIconSize                = 128
	quickLookPad                     = 16
)

// quickLookable reports a row Quick Look shows: a file, or a picture
// the row loads, as a copied image is.
func quickLookable(p engine.Preview) bool {
	return p.Path != "" || (p.Load != nil && !p.LoadsText)
}

// lookOpener is Quick Look's Open: the file opens in its default app,
// then the session ends, so that app takes the keyboard. Nil without
// an opener, when Quick Look offers no Open.
func lookOpener(open func(path string), end func()) func(string) {
	if open == nil {
		return nil
	}
	return func(path string) {
		open(path)
		end()
	}
}

// quickLookShows is what the card shows.
type quickLookShows uint8

const (
	quickLookClosed quickLookShows = iota
	quickLookFile
	quickLookPicture
)

// quickLook is the launcher's Quick Look: the selected row's file, as
// the shared preview shows it, or its picture, in a card over the
// panel and larger than the side pane. The keys stay with the panel,
// which steps its rows underneath the card. Its widgets are built once.
type quickLook struct {
	card    *widget.Box
	file    *preview.View
	picture *widget.ImageView
	fit     image.Point
	sources atomic.Pointer[preview.Sources]
	invoke  func(func())
	spawn   func(func())
	// relayout lays the surface out again: the card comes, goes or
	// changes size.
	relayout func()
	shows    quickLookShows
	// key is the row shown, as its session names rows; cancel ends its
	// picture's load.
	key    any
	cancel context.CancelFunc
}

func newQuickLook(font render.Font, px func(int) int, invoke, spawn func(func()), open func(string), relayout func()) *quickLook {
	q := &quickLook{invoke: invoke, spawn: spawn, relayout: relayout, fit: image.Pt(px(quickLookImageW), px(quickLookImageH))}
	q.file = preview.New(preview.Config{
		Font: font, Class: "launcher-quicklook-file", Image: q.fit, Text: image.Pt(q.fit.X, px(quickLookTextH)),
		IconPx: px(quickLookIconSize), NameChars: 48, Autoplay: true, MIME: mime.System(), Invoke: invoke, Spawn: spawn,
		Open: open,
		Load: func(ctx context.Context, req preview.Request) preview.Content {
			return preview.Load(ctx, *q.sources.Load(), req)
		},
	})
	q.picture = widget.NewImageView(nil, exif.TopLeft)
	q.picture.AddClass("launcher-quicklook-picture")
	q.card = widget.NewBox(widget.Column, 0, px(quickLookPad))
	q.card.AddClass(css.Card, "launcher-quicklook")
	q.card.SetVisible(false)
	return q
}

// shown reports the card showing.
func (q *quickLook) shown() bool { return q.shows != quickLookClosed }

// show shows the row key names, whose preview is p: a file through
// sources, a picture loaded under ctx. A row Quick Look does not show
// closes the card, and show reports false.
func (q *quickLook) show(ctx context.Context, key any, p engine.Preview, sources preview.Sources) bool {
	switch {
	case p.Path != "":
		q.showFile(key, p.Path, sources)
	case p.Load != nil && !p.LoadsText:
		q.showPicture(ctx, key, p)
	default:
		q.close()
		return false
	}
	return true
}

func (q *quickLook) showFile(key any, path string, sources preview.Sources) {
	q.sources.Store(&sources)
	if q.shows != quickLookFile {
		q.clear()
		q.card.Append(q.file.Root(), true)
		q.shows = quickLookFile
	}
	q.key = key
	q.reveal()
	q.file.Show(path)
}

// reveal shows the card, laying the surface out again if it was hidden.
func (q *quickLook) reveal() {
	if q.card.Visible() {
		return
	}
	q.card.SetVisible(true)
	q.relayout()
}

// showPicture loads p's picture off the loop, unless the card already
// shows the row.
func (q *quickLook) showPicture(parent context.Context, key any, p engine.Preview) {
	if q.shows == quickLookPicture && q.key == key {
		return
	}
	q.clear()
	q.shows, q.key = quickLookPicture, key
	q.card.Append(widgets.Center(q.picture), true)
	q.reveal()
	ctx, cancel := context.WithCancel(parent)
	q.cancel = cancel
	q.spawn(func() {
		c, err := engine.LoadPreview(ctx, p)
		q.invoke(func() {
			if ctx.Err() != nil {
				return
			}
			q.landPicture(c, err)
		})
	})
}

// landPicture shows a loaded picture; one that did not load closes the
// card, saying why in the log.
func (q *quickLook) landPicture(c engine.PreviewContent, err error) {
	q.cancel()
	q.cancel = nil
	if err == nil && c.Image == nil {
		err = engine.ErrNoPreview
	}
	if err != nil {
		log.Printf("launcher: quick look: %v", err)
		q.close()
		return
	}
	q.picture.SetImage(c.Image, exif.TopLeft)
	size := thumbnail.FitSize(c.Image.Bounds(), q.fit.X, q.fit.Y)
	widgets.Pin(q.picture, size.X, size.Y)
	q.relayout()
}

// close hides the card and lets go of what it showed.
func (q *quickLook) close() {
	q.clear()
	if !q.card.Visible() {
		return
	}
	q.card.SetVisible(false)
	q.relayout()
}

// clear empties the card: the file's preview closes, the picture's load
// gives up and its pixels go.
func (q *quickLook) clear() {
	if q.cancel != nil {
		q.cancel()
		q.cancel = nil
	}
	switch q.shows {
	case quickLookFile:
		q.file.Close()
	case quickLookPicture:
		q.picture.SetImage(nil, exif.TopLeft)
	}
	q.card.Clear()
	q.shows, q.key = quickLookClosed, nil
}

// quickLookKey is what a key does while Quick Look shows.
type quickLookKey uint8

const (
	quickLookPass quickLookKey = iota
	quickLookClose
	quickLookPrevious
	quickLookNext
)

// quickLookKeyFor is what press does while Quick Look shows: Escape,
// Space and the keys that open it close it, the arrows step the rows
// underneath, and every other key is the session's as ever.
func quickLookKeyFor(press app.Accel, toggles []app.Accel) quickLookKey {
	if slices.Contains(toggles, press) {
		return quickLookClose
	}
	if press.Mods != 0 {
		return quickLookPass
	}
	switch press.Sym {
	case xkb.KeyEscape, xkb.KeySpace:
		return quickLookClose
	case xkb.KeyUp, xkb.KeyLeft:
		return quickLookPrevious
	case xkb.KeyDown, xkb.KeyRight:
		return quickLookNext
	}
	return quickLookPass
}

// quickLookStep is the rows a step key moves.
func quickLookStep(k quickLookKey) int {
	if k == quickLookPrevious {
		return -1
	}
	return 1
}

// spaceAccel is a plain Space, which opens Quick Look in the search
// panel while its field is empty.
var spaceAccel = app.Accel{Sym: xkb.KeySpace}

// accelsFor are the keys table binds to action.
func accelsFor(table []keyBinding, action keyAction) []app.Accel {
	var out []app.Accel
	for _, b := range table {
		if b.action == action {
			out = append(out, b.accel)
		}
	}
	return out
}
