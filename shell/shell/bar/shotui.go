package bar

import (
	"image"
	"log"
	"math"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/layers"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/transfer"
	"github.com/stubbedev/lys/ui/widget"

	timeclock "github.com/stubbedev/lys/shell/internal/clock"
	"github.com/stubbedev/lys/shell/shell/keys"
	"github.com/stubbedev/lys/shell/shell/markup"
	"github.com/stubbedev/lys/shell/shell/quicklook"
	"github.com/stubbedev/lys/shell/shell/screenshot"
	"github.com/stubbedev/lys/shell/shell/shotthumb"
)

// shotSizes are the screenshot surfaces' measures at the configured
// scale: the text, and the reading indicator's padding and its gap
// from the top of the screen.
type shotSizes struct {
	textPx  float64
	padding int
	margin  int32
}

func shotSizesOf(s config.StylingConfig) shotSizes {
	scale := float64(s.Scale)
	rem := widget.RootFontSize() * scale
	return shotSizes{
		textPx:  widgets.TextPx(styling.Body, scale),
		padding: int(math.Round(0.75 * rem)),
		margin:  int32(math.Round(3 * rem)),
	}
}

type shotUI struct {
	app     *app.Application
	outputs func() []*app.Output
	current func() *config.Config
	focused func() (string, bool)
	font    render.Font
	copy    func(transfer.Content) (func(), error)
	// toast shows a failure the user acted on; nil without an OSD.
	toast func(label, icon string)

	thumb   *shotthumb.Thumbnail
	holders map[render.ImageID]int
	saved   map[render.ImageID]screenshot.Saved
}

// newShotUI shows saved captures; copyOnLoop puts content on the
// clipboard and is called on the loop, and toast shows what failed.
func newShotUI(application *app.Application, outputs func() []*app.Output, current func() *config.Config, focused func() (string, bool), font render.Font, sheet *widget.Stylesheet, copyOnLoop func(transfer.Content) (func(), error), toast func(label, icon string), clk *timeclock.Wall) *shotUI {
	u := &shotUI{
		app: application, outputs: outputs, current: current, focused: focused, font: font, copy: copyOnLoop, toast: toast,
		holders: map[render.ImageID]int{}, saved: map[render.ImageID]screenshot.Saved{},
	}
	u.thumb = shotthumb.New(shotthumb.Deps{
		Open: func(lc app.LayerConfig) (shotthumb.Window, error) {
			w, err := application.NewLayer(lc)
			if err != nil {
				return nil, err
			}
			return w, nil
		},
		KeySym: application.KeySym,
		Invoke: application.Invoke,
		Clock:  clk,
		Edit:   u.edit,
		Delete: u.discard,
		Done:   u.release,
		Look:   quicklook.ForApp(application, font, sheet, func() float64 { return float64(current().Styling.Scale) }, layers.ScreenshotThumbnail).Look,
	})
	return u
}

func (u *shotUI) wire(h *screenshot.Host) {
	h.Present = u.present
	h.Edit = func(s screenshot.Saved) {
		u.app.Invoke(func() {
			shot := u.hold(s)
			u.edit(shot)
			u.release(shot)
		})
	}
	h.OpenFolder = func(dir string) {
		u.app.Invoke(func() { u.app.OpenPath(dir, u.failedTo("screenshot-open-folder-failed")) })
	}
	h.Open = func(path string) {
		u.app.Invoke(func() { u.app.OpenPath(path, u.failedTo("screenshot-open-failed")) })
	}
	h.OpenURL = func(uri string) {
		u.app.Invoke(func() { u.app.OpenURL(uri, u.failedTo("screenshot-open-link-failed")) })
	}
	h.Working = u.working
}

func (u *shotUI) present(s screenshot.Saved) {
	name, _ := u.focused()
	u.app.Invoke(func() {
		shot := u.hold(s)
		var out *app.Output
		for _, o := range u.outputs() {
			if o.Name == name {
				out = o
			}
		}
		if err := u.thumb.Show(shot, out, u.current().Screenshot); err != nil {
			u.failed("screenshot-thumbnail-failed", err)
		}
	})
}

func (u *shotUI) hold(s screenshot.Saved) shotthumb.Shot {
	shot := shotthumb.Shot{Path: s.Path, Image: render.NewStillImage(s.Image)}
	u.holders[shot.Image.ID] = 1
	u.saved[shot.Image.ID] = s
	return shot
}

func (u *shotUI) release(shot shotthumb.Shot) {
	id := shot.Image.ID
	if u.holders[id]--; u.holders[id] > 0 {
		return
	}
	delete(u.holders, id)
	delete(u.saved, id)
	u.app.ForgetImage(id)
}

func (u *shotUI) discard(shot shotthumb.Shot) {
	s, ok := u.saved[shot.Image.ID]
	if !ok {
		s = screenshot.Saved{Path: shot.Path}
	}
	go func() {
		if err := s.Discard(); err != nil {
			u.failed("screenshot-delete-failed", err)
		}
	}()
}

func (u *shotUI) edit(shot shotthumb.Shot) {
	u.holders[shot.Image.ID]++
	_, err := markup.Open(markup.Deps{
		App:     u.app,
		Font:    u.font,
		Px:      shotSizesOf(u.current().Styling).textPx,
		Palette: markupPalette(u.current().Styling),
		Save:    u.writer(true),
		SaveAs:  u.writer(false),
		Closed:  func() { u.release(shot) },
	}, shot.Image, shot.Path)
	if err != nil {
		u.failed("screenshot-editor-failed", err)
		u.release(shot)
	}
}

// failed toasts the summary key names, logging err with it.
func (u *shotUI) failed(key string, err error) {
	toastFailure(u.toast, "screenshot", i18n.T(key), err)
}

// failedTo is a done callback that toasts key when the work failed.
func (u *shotUI) failedTo(key string) func(error) {
	return func(err error) {
		if err != nil {
			u.failed(key, err)
		}
	}
}

// writer encodes and writes an edited image off the loop and, when
// copies, puts it on the clipboard back on the loop. A failed copy is
// toasted: the image is saved all the same.
func (u *shotUI) writer(copies bool) markup.Writer {
	return func(img *image.RGBA, path string, done func(error)) {
		go func() {
			err := screenshot.WritePNG(img, path)
			var content transfer.Content
			var encodeErr error
			if err == nil && copies {
				content, encodeErr = transfer.Image(img)
			}
			u.app.Invoke(func() {
				if err == nil && copies {
					u.copyEdited(content, encodeErr)
				}
				done(err)
			})
		}()
	}
}

func (u *shotUI) copyEdited(content transfer.Content, encodeErr error) {
	err := encodeErr
	if err == nil {
		_, err = u.copy(content)
	}
	if err != nil {
		u.failed("screenshot-copy-edited-failed", err)
	}
}

func markupPalette(s config.StylingConfig) []render.Color {
	p, err := styling.ConfigPalette(s)
	if err != nil {
		log.Printf("screenshot: markup palette: %v", err)
		p = styling.Default()
	}
	return []render.Color{p.Red, p.Yellow, p.Green, p.Blue, p.Primary, p.Fg, p.Bg}
}

func (u *shotUI) working(cancel func()) func() {
	opened := make(chan *app.LayerWindow, 1)
	u.app.Invoke(func() {
		sizes := shotSizesOf(u.current().Styling)
		label := widget.NewLabel(u.font, sizes.textPx, i18n.T("screenshot-reading-text"), 0)
		box := widget.NewBox(widget.Row, 0, sizes.padding)
		box.SetElement("item")
		box.Append(label, false)
		w, err := u.app.NewLayer(app.LayerConfig{
			Layer:     app.LayerOverlay,
			Anchor:    app.AnchorTop,
			Margin:    app.Margins{Top: sizes.margin},
			Keyboard:  app.KeyboardExclusive,
			Namespace: layers.ScreenshotReading.Name(),
			Root:      box,
			OnKey:     keys.OnEscape(u.app.KeySym, cancel),
		})
		if err != nil {
			log.Printf("screenshot: reading indicator: %v", err)
		}
		opened <- w
	})
	return func() {
		u.app.Invoke(func() {
			if w := <-opened; w != nil {
				w.Close()
			}
		})
	}
}
