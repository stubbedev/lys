package launcher

import (
	"context"
	"image"
	"slices"
	"sync/atomic"

	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/shell/preview"
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

// quickLook is the launcher's Quick Look: the selected row's file, or
// its picture held in memory, as the shared preview shows them, in a
// card over the panel and larger than the side pane. The keys stay
// with the panel, which steps its rows underneath the card. Its widgets
// are built once.
type quickLook struct {
	card    *widget.Box
	file    *preview.View
	sources atomic.Pointer[preview.Sources]
	// moved repaints where the card was and where it goes as it comes
	// or goes.
	moved func()
	shows quickLookShows
	// key is the row shown, as its session names rows.
	key any
}

func newQuickLook(font render.Font, px func(int) int, invoke, spawn func(func()), open func(string), moved func()) *quickLook {
	q := &quickLook{moved: moved}
	fit := image.Pt(px(quickLookImageW), px(quickLookImageH))
	q.file = preview.New(preview.Config{
		Font: font, Class: "launcher-quicklook-file", Image: fit, Text: image.Pt(fit.X, px(quickLookTextH)),
		IconPx: px(quickLookIconSize), NameChars: 48, Autoplay: true, MIME: mime.System(), Invoke: invoke, Spawn: spawn,
		Open: open,
		Load: func(ctx context.Context, req preview.Request) preview.Content {
			return preview.Load(ctx, *q.sources.Load(), req)
		},
	})
	q.card = widget.NewBox(widget.Column, 0, px(quickLookPad))
	q.card.AddClass(css.Card, "launcher-quicklook")
	q.card.SetVisible(false)
	return q
}

// shown reports the card showing.
func (q *quickLook) shown() bool { return q.shows != quickLookClosed }

// show shows the row key names, whose preview is p, through sources: a
// file, or a picture it loads, each as the shared preview shows it. A
// row Quick Look does not show closes the card, and show reports
// false.
func (q *quickLook) show(_ context.Context, key any, p engine.Preview, sources preview.Sources) bool {
	switch {
	case p.Path != "":
		q.open(key, quickLookFile, sources)
		q.file.Show(p.Path)
	case p.Load != nil && !p.LoadsText:
		if q.shows == quickLookPicture && q.key == key {
			return true
		}
		q.open(key, quickLookPicture, sources)
		q.file.ShowPicture(preview.Memory{Read: p.Load})
	default:
		q.close()
		return false
	}
	return true
}

// open makes the card show the row key names, as shows, through
// sources.
func (q *quickLook) open(key any, shows quickLookShows, sources preview.Sources) {
	q.sources.Store(&sources)
	if q.shows == quickLookClosed {
		q.card.Append(q.file.Root(), true)
	}
	q.shows, q.key = shows, key
	q.reveal()
}

// reveal shows the card if it was hidden.
func (q *quickLook) reveal() {
	if q.card.Visible() {
		return
	}
	q.card.SetVisible(true)
	q.moved()
}

// close hides the card and lets go of what it showed: the preview
// closes, its load giving up and its pixels going.
func (q *quickLook) close() {
	if q.shows != quickLookClosed {
		q.file.Close()
		q.card.Clear()
		q.shows, q.key = quickLookClosed, nil
	}
	if !q.card.Visible() {
		return
	}
	q.moved()
	q.card.SetVisible(false)
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
