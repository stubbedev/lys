package viewer

import (
	"context"
	"image"
	"log"
	"path/filepath"
	"time"

	"github.com/stubbedev/lys/desktop/dochelper"
	"github.com/stubbedev/lys/desktop/fileops"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/transfer"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/viewer/internal/locale"
)

const (
	thumbnailBox  = 1024
	iconPx        = 96
	nameChars     = 80
	toolbarIconPx = 16
	seekStep      = 5 * time.Second
	volumeStep    = 0.1
)

const (
	previousIcon = "ld-chevron-left-symbolic"
	nextIcon     = "ld-chevron-right-symbolic"
	loopIcon     = "ld-repeat-symbolic"
	openWithIcon = "ld-external-link-symbolic"
	copyIcon     = "ld-copy-symbolic"
	trashIcon    = "ld-trash-2-symbolic"
)

type Host interface {
	Invoke(fn func())
	Spawn(fn func())
	SetTitle(title string)
	Toast(text string, undo func())
	Copy(content transfer.Content) error
	SetFullscreen(on bool)
	OpenWith(anchor widget.Boundser, path string)
	OpenURI(uri string)
	Focus(w widget.Widget)
}

type Config struct {
	Font      render.Font
	MIME      *mime.Database
	Load      func(context.Context, preview.Request) preview.Content
	Documents func(path string) *dochelper.Client
	Bin       fileops.Bin
	Host      Host
}

type trashed struct {
	path string
	at   int
	undo *fileops.Undo
}

type Viewer struct {
	host        Host
	bin         fileops.Bin
	opens       opens
	view        *preview.View
	doc         *document
	root        *widget.Box
	bar         *widget.Box
	previous    *widget.Button
	next        *widget.Button
	loop        *widget.ToggleButton
	openWith    *widget.Button
	copy        *widget.Button
	trash       *widget.Button
	files       Collection
	trashed     []trashed
	trashing    map[string]bool
	fullscreen  bool
	keys        map[app.Accel]func() bool
	whileTyping map[app.Accel]bool
}

func New(cfg Config) *Viewer {
	v := &Viewer{host: cfg.Host, bin: cfg.Bin, opens: newOpens(cfg.MIME), trashing: map[string]bool{}}
	v.view = preview.New(preview.Config{
		Font: cfg.Font, Class: "viewer", Image: image.Pt(thumbnailBox, thumbnailBox),
		IconPx: iconPx, NameChars: nameChars, Placeholder: locale.T().Get("viewer-empty"),
		Autoplay: true, MIME: cfg.MIME, Invoke: cfg.Host.Invoke, Spawn: cfg.Host.Spawn, Load: cfg.Load,
		Fill: true, OnRender: v.rendered,
	})
	th := widget.Current()
	v.bar = widget.NewBox(widget.Row, th.Spacing, th.Padding)
	v.bar.AddClass(css.Toolbar)
	v.previous = v.button(previousIcon, "viewer-previous", func() { v.step(-1) })
	v.next = v.button(nextIcon, "viewer-next", func() { v.step(1) })
	v.loop = widget.NewToggleButton(widget.NewThemeIcon(loopIcon, toolbarIconPx), 0, 0)
	label(v.loop, "viewer-loop")
	v.loop.OnToggled = v.looped
	v.openWith = v.button(openWithIcon, "viewer-open-with", v.openWithApps)
	v.copy = v.button(copyIcon, "viewer-copy", v.copyPicture)
	v.trash = v.button(trashIcon, "viewer-trash", v.trashCurrent)
	v.bar.Append(v.previous, false)
	v.bar.Append(v.next, false)
	v.bar.Append(widget.NewSpacer(0, 0), true)
	v.bar.Append(v.loop, false)
	v.bar.Append(v.openWith, false)
	v.bar.Append(v.copy, false)
	v.bar.Append(v.trash, false)
	v.doc = newDocument(cfg.Font, cfg.Host, cfg.Documents)
	v.root = widget.NewBox(widget.Column, 0, 0)
	v.root.Append(v.view.Root(), true)
	v.root.Append(v.doc.root, true)
	v.root.Append(v.bar, false)
	v.keys = v.bindings()
	return v
}

type labelled interface {
	AddClass(names ...string)
	SetTooltip(s string)
	SetAccessibleName(name string)
}

func label(w labelled, id string) {
	text := locale.T().Get(id)
	w.AddClass(css.Flat)
	w.SetTooltip(text)
	w.SetAccessibleName(text)
}

func (v *Viewer) button(icon, id string, click func()) *widget.Button {
	b := widget.NewButton(widget.NewThemeIcon(icon, toolbarIconPx), 0, 0)
	label(b, id)
	b.OnClick = click
	return b
}

func (v *Viewer) Root() *widget.Box { return v.root }

func (v *Viewer) View() *preview.View { return v.view }

func (v *Viewer) Files() Collection { return v.files }

func (v *Viewer) Open(c Collection) {
	v.files = c
	v.trashed = nil
	v.show()
}

func (v *Viewer) Close() {
	v.view.Close()
	v.doc.Close()
}

func (v *Viewer) show() {
	path, ok := v.files.Current()
	if path != v.view.Path() || !ok {
		v.copy.SetEnabled(false)
		v.loop.SetVisible(false)
	}
	reading := ok && v.opens.document(path)
	v.view.Root().SetVisible(!reading)
	switch {
	case reading:
		v.view.Close()
		v.doc.Open(path)
		v.host.SetTitle(filepath.Base(path))
	case ok:
		v.doc.Close()
		v.view.Show(path)
		v.host.SetTitle(filepath.Base(path))
	default:
		v.doc.Close()
		v.view.Close()
		v.host.SetTitle(locale.T().Get("viewer-empty"))
	}
	v.previous.SetEnabled(ok && !v.files.First())
	v.next.SetEnabled(ok && !v.files.Last())
	v.openWith.SetEnabled(ok)
	v.trash.SetEnabled(ok)
}

func (v *Viewer) rendered(r preview.Renderer) {
	v.copy.SetEnabled(r == preview.Image)
	stream := v.view.Stream()
	v.loop.SetVisible(stream != nil)
	if stream != nil {
		stream.SetLoop(v.loop.Active())
	}
}

func (v *Viewer) step(delta int) bool {
	next, ok := v.files.Step(delta)
	if ok {
		v.files = next
		v.show()
	}
	return true
}

func (v *Viewer) jump(at int) bool {
	if next, ok := v.files.Jump(at); ok {
		v.files = next
		v.show()
	}
	return true
}

func (v *Viewer) looped(on bool) {
	if s := v.view.Stream(); s != nil {
		s.SetLoop(on)
	}
}

func (v *Viewer) openWithApps() {
	if path, ok := v.files.Current(); ok {
		v.host.OpenWith(v.openWith, path)
	}
}

func (v *Viewer) toggleFullscreen() bool {
	v.setFullscreen(!v.fullscreen)
	return true
}

func (v *Viewer) setFullscreen(on bool) {
	v.fullscreen = on
	v.bar.SetVisible(!on)
	v.host.SetFullscreen(on)
}

func (v *Viewer) trashCurrent() {
	path, ok := v.files.Current()
	if !ok || v.trashing[path] {
		return
	}
	v.trashing[path] = true
	bin := v.bin
	v.host.Spawn(func() {
		undo, err := fileops.Start(fileops.Trash{Bin: bin, Paths: []string{path}}, quiet{}).Wait()
		v.host.Invoke(func() { v.trashDone(path, undo, err) })
	})
}

func (v *Viewer) trashDone(path string, undo *fileops.Undo, err error) {
	delete(v.trashing, path)
	name := i18n.Str("name", filepath.Base(path))
	if err != nil {
		log.Printf("viewer: %v", err)
		v.host.Toast(locale.T().Get("viewer-trash-failed", name), nil)
		return
	}
	files, at, ok := v.files.Without(path)
	if !ok {
		return
	}
	v.files = files
	v.trashed = append(v.trashed, trashed{path: path, at: at, undo: undo})
	v.show()
	v.host.Toast(locale.T().Get("viewer-trashed", name), func() { v.restore(path) })
}

func (v *Viewer) undoTrash() bool {
	if len(v.trashed) == 0 {
		return false
	}
	v.restore(v.trashed[len(v.trashed)-1].path)
	return true
}

func (v *Viewer) restore(path string) {
	last := len(v.trashed) - 1
	if last < 0 || v.trashed[last].path != path {
		return
	}
	t := v.trashed[last]
	v.trashed = v.trashed[:last]
	v.host.Spawn(func() {
		_, err := fileops.Start(t.undo, quiet{}).Wait()
		v.host.Invoke(func() { v.restored(t, err) })
	})
}

func (v *Viewer) restored(t trashed, err error) {
	if err != nil {
		log.Printf("viewer: %v", err)
		v.host.Toast(locale.T().Get("viewer-restore-failed", i18n.Str("name", filepath.Base(t.path))), nil)
		return
	}
	v.files = v.files.With(t.path, t.at)
	v.show()
}

func (v *Viewer) copyPicture() {
	path, ok := v.files.Current()
	if !ok || v.view.Renderer() != preview.Image {
		return
	}
	v.host.Spawn(func() {
		content, err := pictureContent(path)
		v.host.Invoke(func() {
			if err == nil {
				err = v.host.Copy(content)
			}
			if err != nil {
				log.Printf("viewer: copy %s: %v", path, err)
				v.host.Toast(locale.T().Get("viewer-copy-failed"), nil)
				return
			}
			v.host.Toast(locale.T().Get("viewer-copied"), nil)
		})
	})
}

type quiet struct{}

func (quiet) Progress(fileops.Progress) {}

func (quiet) Resolve(fileops.Conflict) fileops.Resolution {
	return fileops.Resolution{Action: fileops.Skip}
}
