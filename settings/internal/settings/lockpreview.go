package settings

import (
	"image"
	"log"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/desktop/shell/lockview"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
	"github.com/stubbedev/memory-go"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	lockPreviewScreenW = 832
	lockPreviewScreenH = 520
)

var lockPreviewMoment = time.Date(2026, time.March, 14, 9, 41, 0, 0, time.UTC)

// lockPreview is lys-login's lock screen drawn from the config the
// window edits, at a fixed moment, for a sample account.
func lockPreview(k *kit) widget.Widget {
	bg := k.lockBackdrop()
	p := newLivePreview(k, "settings-lock-preview", func(k *kit, cfg *config.Config, font render.Font) widget.Widget {
		return drawLockOnScreen(k, cfg, font, bg)
	})
	bg.redraw = p.configChanged
	return p
}

func drawLockOnScreen(k *kit, cfg *config.Config, font render.Font, bg *lockBackdrop) widget.Widget {
	shown := bg.show(lockBackdropKey{path: cfg.Login.Background.ImagePath(cfg.Wallpaper.Wallpaper), sigma: cfg.Login.Blur.Sigma()})
	t := locale.T()
	account := lockview.NewAccountTile(font, t.Get("settings-lock-preview-account"))
	surface := lockview.Build(lockview.Spec{
		Mode: lockview.Lock, Config: cfg.Login, Font: font, Backdrop: shown.backdrop,
		Identity: account.Root, Password: i18n.T("lock-password"), CapsLock: i18n.T("lock-caps-lock"),
	})
	surface.Prompt.SetTime(lockview.TimeTexts(cfg.Login, lockPreviewMoment))
	surface.Prompt.Reveal.SetDuration(0)
	surface.Prompt.Reveal.SetRevealed(true)
	surface.Prompt.Reveal.Finish()
	screen := widget.NewBox(widget.Column, 0, 0)
	screen.AddClass("settings-lock-preview-screen")
	screen.Append(surface.Root, true)
	widgets.Pin(screen, 0, lockPreviewScreenH)
	column := widget.NewBox(widget.Column, widget.Current().Spacing, 0)
	column.Append(overflowing(screen), false)
	if note := shown.note(cfg.Login.Background); note != "" {
		column.Append(wrapped(k.label(note, "settings-preview-state", css.Caption, css.DimLabel)), false)
	}
	return column
}

type lockBackdropKey struct {
	path  string
	sigma float64
}

type lockBackdropState int

const (
	lockBackdropNone lockBackdropState = iota
	lockBackdropLoading
	lockBackdropShown
	lockBackdropFailed
)

// lockBackdropShowing is what the preview draws its background from:
// the backdrop while it is loaded, else the colour fill and why.
type lockBackdropShowing struct {
	state    lockBackdropState
	path     string
	backdrop *lockview.Backdrop
	err      error
}

func (s lockBackdropShowing) note(bg config.Background) string {
	t := locale.T()
	switch s.state {
	case lockBackdropLoading:
		return t.Get("settings-lock-preview-loading")
	case lockBackdropFailed:
		return t.Get("settings-lock-preview-unreadable", i18n.Str("path", s.path), i18n.Str("reason", s.err.Error()))
	case lockBackdropNone:
		if bg.Mode != config.BackgroundColor {
			return t.Get("settings-lock-preview-no-image")
		}
	case lockBackdropShown:
	}
	return ""
}

// lockBackdrop is the one lock screen background the window's previews
// hold: the image the draft names, blurred as lys-login blurs it at the
// preview's size, loaded off the loop and let go of once the draft names
// another.
type lockBackdrop struct {
	k       *kit
	mem     memory.Shared
	showing lockBackdropShowing
	key     lockBackdropKey
	drawing bool
	redraw  func()
}

func (k *kit) lockBackdrop() *lockBackdrop {
	if k.lockBg == nil {
		k.lockBg = &lockBackdrop{k: k}
	}
	return k.lockBg
}

type loadedLockBackdrop struct {
	key      lockBackdropKey
	backdrop *lockview.Backdrop
	err      error
}

// show is what the preview draws for key, starting its load when the
// draft names an image not held yet.
func (b *lockBackdrop) show(key lockBackdropKey) lockBackdropShowing {
	if key == b.key && b.showing.state != lockBackdropNone {
		return b.showing
	}
	b.release()
	b.key = key
	if key.path == "" {
		b.showing = lockBackdropShowing{}
		return b.showing
	}
	b.showing = lockBackdropShowing{state: lockBackdropLoading, path: key.path}
	b.drawing = true
	async(b.k, func() loadedLockBackdrop {
		bg, err := lockview.LoadBackdrop(&b.mem, imagedecode.OpenOffHeap, key.path, key.sigma, []image.Point{{X: lockPreviewScreenW, Y: lockPreviewScreenH}})
		return loadedLockBackdrop{key: key, backdrop: bg, err: err}
	}, b.loaded)
	b.drawing = false
	return b.showing
}

func (b *lockBackdrop) loaded(l loadedLockBackdrop) {
	if l.key != b.key {
		if l.backdrop != nil {
			l.backdrop.Retire(b.k.forgetImage, "settings: lock preview")
		}
		return
	}
	if l.err != nil {
		log.Printf("settings: lock preview: %v", l.err)
		b.showing = lockBackdropShowing{state: lockBackdropFailed, path: l.key.path, err: l.err}
	} else {
		b.showing = lockBackdropShowing{state: lockBackdropShown, path: l.key.path, backdrop: l.backdrop}
	}
	if !b.drawing {
		b.redraw()
	}
}

func (b *lockBackdrop) release() {
	if b.showing.backdrop != nil {
		b.showing.backdrop.Retire(b.k.forgetImage, "settings: lock preview")
	}
	b.showing = lockBackdropShowing{}
}
