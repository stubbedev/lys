package preview

import (
	"context"
	"fmt"
	"image"
	"log"
	"path/filepath"

	"github.com/stubbedev/lys/lib/exif"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/media"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/desktop/thumbnail"
)

// folderIcon is a folder's glyph.
const folderIcon = "ld-folder-symbolic"

// Config is how a View shows files and what its host does for it.
type Config struct {
	// Font paints the name, the detail line, the text and the controls.
	Font render.Font
	// Class styles the view's card.
	Class string
	// Image is the box pictures, videos and thumbnails fit into, and
	// Text the box text fills, in pixels; thumbnails are made at twice
	// Image, for HiDPI.
	Image, Text image.Point
	// IconPx is the size of a file's icon.
	IconPx int
	// NameChars is the most characters the name's line is wide.
	NameChars int
	// Placeholder is the name line while nothing is shown.
	Placeholder string
	// Autoplay plays a video as soon as it shows.
	Autoplay bool
	// MIME names the icon a file shows while it loads.
	MIME *mime.Database
	// Invoke runs fn on the UI loop; Spawn runs fn off it.
	Invoke, Spawn func(fn func())
	// Load finds out what to show of a file, off the loop: Load over the
	// host's Sources.
	Load func(context.Context, Request) Content
	// OnClose closes the preview, for Space and Escape; nil leaves them
	// to the host.
	OnClose func()
	// OnStep moves the preview to the hosting list's next (1) or previous
	// (-1) item, for the arrows; nil leaves them to the host.
	OnStep func(delta int)
	// Open opens a file in its default app, from the Open button; nil
	// shows no button.
	Open func(path string)
	// Fill shows pictures, videos and text over all the space the view
	// is given rather than fitted into the Image and Text boxes, as a
	// window-sized viewer does; Image is then the box thumbnails are
	// made for.
	Fill bool
	// OnRender runs on the loop with what the view shows a file as,
	// once its load lands and again when its video fails as it plays;
	// nil tells nobody.
	OnRender func(Renderer)
}

// View shows one file at a time, as its renderer says: its picture, its
// video, its text, its folder's count or its thumbnail over its name
// and a detail line, else its icon, or a message saying why it cannot
// be shown. It loads off the loop, at most one load per loop pass
// however often the file shown changes, and lets go of everything the
// last file held (its pixels, its video's decoding) as soon as another
// shows or it closes. Its widgets are built once.
type View struct {
	cfg     Config
	root    *widget.Box
	visual  *widget.Box
	name    *widget.Label
	info    *widget.Label
	message *widget.Label
	open    *widget.Button
	code    *widgets.CodeView
	picture *widget.ImageView
	video   *widget.Video

	path     string
	memory   *Memory
	renderer Renderer
	shown    Content
	cancel   context.CancelFunc
	queued   bool
	stream   *media.Stream
	unsub    func()
}

// New builds a view showing nothing.
func New(cfg Config) *View {
	th := widget.Current()
	small := widgets.TextPx(styling.Caption, 1)
	v := &View{cfg: cfg}
	v.root = widget.NewBox(widget.Column, th.Spacing, 2*th.Padding)
	v.root.AddClass(cfg.Class)
	v.visual = widget.NewBox(widget.Column, th.Spacing, 0)
	v.visual.AddClass("preview-visual")
	v.name = widget.NewLabel(cfg.Font, widgets.TextPx(styling.Body, 1), cfg.Placeholder, 0)
	v.name.AddClass("preview-name")
	v.name.SetAlignment(render.AlignCenter)
	v.name.SetWrap(true)
	v.name.SetMaxWidthChars(cfg.NameChars)
	v.info = widget.NewLabel(cfg.Font, small, "", 0)
	v.info.AddClass("preview-info", css.Caption, css.DimLabel)
	v.info.SetAlignment(render.AlignCenter)
	v.info.SetWrap(true)
	v.message = widget.NewLabel(cfg.Font, small, "", 0)
	v.message.AddClass("preview-message", css.Caption)
	v.message.SetAlignment(render.AlignCenter)
	v.message.SetWrap(true)
	v.message.SetMaxWidthChars(cfg.NameChars)
	v.code = widgets.NewCodeView(cfg.Font, small)
	v.picture = widget.NewImageView(nil, exif.TopLeft)
	v.picture.AddClass("preview-picture")
	v.video = widget.NewVideo(cfg.Font, small, nil)
	v.video.SetAutoplay(cfg.Autoplay)
	if cfg.Fill {
		v.root.Append(v.visual, true)
	} else {
		v.root.Append(widgets.Center(v.visual), false)
	}
	v.root.Append(v.name, false)
	v.root.Append(v.info, false)
	if cfg.Open != nil {
		label := widget.NewLabel(cfg.Font, widgets.TextPx(styling.Body, 1), i18n.T("preview-open"), 0)
		label.SetAlignment(render.AlignCenter)
		v.open = widget.NewButton(label, 0, 0)
		v.open.AddClass("preview-open", css.SuggestedAction)
		v.open.OnClick = func() {
			if v.path != "" {
				cfg.Open(v.path)
			}
		}
		v.open.SetVisible(false)
		v.root.Append(widgets.Center(v.open), false)
	}
	return v
}

// Root is the view's widget.
func (v *View) Root() *widget.Box { return v.root }

// Path is the file shown or loading, "" for none and for a picture
// held in memory.
func (v *View) Path() string { return v.path }

// Renderer is what the view shows: Empty, Loading while the file
// loads, then what it shows the file as.
func (v *View) Renderer() Renderer { return v.renderer }

// Picture is the zoomable view a picture and a thumbnail show in, for
// a host's zoom and pan keys; it shows nothing under another renderer.
func (v *View) Picture() *widget.ImageView { return v.picture }

// Stream is the video playing, for a host's playback keys; nil while
// no video plays.
func (v *View) Stream() *media.Stream { return v.stream }

// Show shows the file at path: its name and the icon its name suggests
// at once, its contents once a load the loop starts on its next pass
// lands. The file already shown or loading stays as it is.
func (v *View) Show(path string) {
	if v.memory == nil && path == v.path && v.renderer != Empty {
		return
	}
	v.load(path, nil, filepath.Base(path))
}

// ShowPicture shows the picture m holds as Show shows a picture file:
// its name, its line hidden for none, and the picture icon at once, the
// picture once a load the loop starts on its next pass lands. It has no
// file to open. Each call loads m afresh.
func (v *View) ShowPicture(m Memory) {
	v.load("", &m, m.Name)
}

// load starts showing the file at path or the picture m holds, by name.
func (v *View) load(path string, m *Memory, name string) {
	v.drop()
	v.path, v.memory, v.renderer = path, m, Loading
	v.name.SetText(name)
	v.name.SetVisible(name != "")
	v.info.SetText("")
	v.showIcon(v.cfg.MIME.FileIcon(v.guessedType()))
	if v.open != nil {
		v.open.SetVisible(m == nil)
	}
	if v.queued {
		return
	}
	v.queued = true
	v.cfg.Invoke(v.start)
}

// shownName is the name of what the view shows: the file's, or the
// picture's held in memory.
func (v *View) shownName() string {
	if v.memory != nil {
		return v.memory.Name
	}
	return filepath.Base(v.path)
}

// guessedType is the type of what the view shows before its load says:
// the file's by its name, any picture for one held in memory.
func (v *View) guessedType() string {
	if v.memory != nil {
		return anyPicture
	}
	return v.cfg.MIME.TypeByName(filepath.Base(v.path))
}

// request is what loading what the view shows asks for.
func (v *View) request() Request {
	return Request{Path: v.path, Memory: v.memory, Fit: image.Pt(2*v.cfg.Image.X, 2*v.cfg.Image.Y)}
}

// Close shows nothing: the load running gives up, and what the file
// held goes, its video's decoding with it.
func (v *View) Close() {
	v.drop()
	v.path, v.memory, v.renderer = "", nil, Empty
	v.name.SetText(v.cfg.Placeholder)
	v.name.SetVisible(true)
	v.info.SetText("")
	v.visual.Clear()
	if v.open != nil {
		v.open.SetVisible(false)
	}
}

// Key handles a key pressed without modifiers while the view shows a
// file: Space and Escape close it, the arrows move to the previous
// (Left, Up) or next (Right, Down) item, each when the host handles
// it. It reports whether it handled the key.
func (v *View) Key(sym xkb.Keysym) bool {
	if v.renderer == Empty {
		return false
	}
	switch sym {
	case xkb.KeySpace, xkb.KeyEscape:
		if v.cfg.OnClose == nil {
			return false
		}
		v.cfg.OnClose()
	case xkb.KeyLeft, xkb.KeyUp:
		return v.step(-1)
	case xkb.KeyRight, xkb.KeyDown:
		return v.step(1)
	default:
		return false
	}
	return true
}

func (v *View) step(delta int) bool {
	if v.cfg.OnStep == nil {
		return false
	}
	v.cfg.OnStep(delta)
	return true
}

// start loads the file the view last asked for, off the loop, unless
// it closed or one is loading.
func (v *View) start() {
	v.queued = false
	if v.renderer != Loading || v.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	v.cancel = cancel
	req := v.request()
	load, invoke := v.cfg.Load, v.cfg.Invoke
	v.cfg.Spawn(func() {
		c := load(ctx, req)
		invoke(func() { v.land(ctx, req, c) })
	})
}

// land shows a load that ended, unless the view moved on, when what it
// loaded goes at once.
func (v *View) land(ctx context.Context, req Request, c Content) {
	if ctx.Err() != nil || v.request() != req || v.renderer != Loading {
		discard(c)
		return
	}
	v.cancel()
	v.cancel = nil
	v.apply(c)
}

// apply shows c, the loaded content of the file shown.
func (v *View) apply(c Content) {
	name := v.shownName()
	v.renderer, v.shown = c.renderer, c
	switch c.renderer {
	case Image, Thumbnail:
		v.picture.SetImage(c.picture.img, c.picture.orient)
		b := c.picture.img.Bounds()
		w, h := c.picture.orient.Size(b.Dx(), b.Dy())
		v.fit(v.picture, thumbnail.FitSize(image.Rect(0, 0, w, h), v.cfg.Image.X, v.cfg.Image.Y))
	case Video:
		v.playVideo(c, name)
		v.rendered()
		return
	case Text:
		if v.code.Text() != c.text || v.code.Name() != name {
			v.code.SetText(c.text, name)
		}
		v.fit(v.code, v.cfg.Text)
	case Folder:
		v.showIcon(folderIcon)
	case Icon:
		v.showIcon(v.iconOf(c.info))
	case Failed:
		v.fail(c.failure, c.err)
	default:
		panic(fmt.Sprintf("preview: a load landed as %v", c.renderer))
	}
	v.info.SetText(detail(name, c))
	v.rendered()
}

// rendered tells the host what the view now shows the file as.
func (v *View) rendered() {
	if v.cfg.OnRender != nil {
		v.cfg.OnRender(v.renderer)
	}
}

// playVideo shows c's video, which the stream now owns.
func (v *View) playVideo(c Content, name string) {
	v.stream = media.NewStream(v.video, c.video, name)
	v.shown.video = nil
	v.video.SetStream(v.stream)
	w, h := v.stream.Size()
	v.fit(v.video, thumbnail.FitSize(image.Rect(0, 0, w, h), v.cfg.Image.X, v.cfg.Image.Y))
	v.info.SetText(detail(name, c))
	stream := v.stream
	v.unsub = stream.Subscribe(func() {
		if err := stream.Err(); err != nil && v.stream == stream {
			v.stopVideo()
			v.renderer = Failed
			v.fail(Unsupported, err)
			v.rendered()
		}
	})
}

// fail says why the file cannot be shown, under its icon.
func (v *View) fail(f Failure, err error) {
	log.Printf("preview: %s: %v", v.path, err)
	v.renderer = Failed
	v.message.SetText(message(f))
	v.visual.Clear()
	v.fillAs(widget.AlignCenter)
	v.visual.Append(widgets.Center(widget.NewThemeIcon(v.iconOf(v.shown.info), v.cfg.IconPx)), false)
	v.visual.Append(v.message, false)
}

// message is what a view says of a file that fails so.
func message(f Failure) string {
	switch f {
	case Unreadable:
		return i18n.T("preview-failed-unreadable")
	case Undecodable:
		return i18n.T("preview-failed-undecodable")
	case TooLarge:
		return i18n.T("preview-failed-too-large")
	case Unsupported:
		return i18n.T("preview-failed-unsupported")
	}
	panic(fmt.Sprintf("preview: failure %d has no message", f))
}

func (v *View) iconOf(info Info) string {
	if info.Dir {
		return folderIcon
	}
	if info.Type == "" {
		return v.cfg.MIME.FileIcon(v.guessedType())
	}
	return v.cfg.MIME.FileIcon(info.Type)
}

func (v *View) showIcon(name string) {
	v.showContent(widget.NewThemeIcon(name, v.cfg.IconPx))
}

func (v *View) showContent(w widget.Widget) {
	v.visual.Clear()
	v.fillAs(widget.AlignCenter)
	v.visual.Append(widgets.Center(w), false)
}

// fit shows w, a picture, video or text, size big, or over the whole
// view when it fills.
func (v *View) fit(w widget.Widget, size image.Point) {
	if !v.cfg.Fill {
		v.showContent(widgets.Pin(w, size.X, size.Y))
		return
	}
	v.visual.Clear()
	v.fillAs(widget.AlignFill)
	v.visual.Append(w, true)
}

// fillAs sets how what a filling view shows sits in it: a picture,
// video or text over all of it, an icon and a message in its middle.
func (v *View) fillAs(a widget.Align) {
	if v.cfg.Fill {
		v.visual.SetVAlign(a)
	}
}

// drop gives up the load running and lets go of what was shown.
func (v *View) drop() {
	if v.cancel != nil {
		v.cancel()
		v.cancel = nil
	}
	v.stopVideo()
	if v.shown.picture.img != nil {
		v.picture.SetImage(nil, exif.TopLeft)
	}
	discard(v.shown)
	v.shown = Content{}
}

// stopVideo closes the video showing, which stops its decoding.
func (v *View) stopVideo() {
	if v.stream == nil {
		return
	}
	v.unsub()
	v.unsub = nil
	v.video.SetStream(nil)
	v.stream.Close()
	v.stream = nil
}

// discard lets go of what c holds: an unshown video at once, pixels
// once no frame recorded until now draws them.
func discard(c Content) {
	if c.video != nil {
		if err := c.video.Close(); err != nil {
			log.Printf("preview: closing an unshown video: %v", err)
		}
	}
	if free := c.picture.free; free != nil {
		render.Retire(free)
	}
}
