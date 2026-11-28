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
	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/memory-go"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	lockPreviewScreenW = 832
	lockPreviewScreenH = 520
)

var lockPreviewMoment = time.Date(2026, time.March, 14, 9, 41, 0, 0, time.UTC)

const lockPreviewBatteryPercent = 80

var lockPreviewModes = [...]struct {
	mode  lockview.Mode
	label string
}{
	{lockview.Lock, "settings-lock-preview-lock"},
	{lockview.Greeter, "settings-lock-preview-greeter"},
}

// lockPreview is lys-login's screen drawn from the config the window
// edits, at a fixed moment, for sample accounts: the lock screen or the
// greeter, as the switch above it picks.
func lockPreview(k *kit) widget.Widget {
	bg := k.lockBackdrop()
	mode := lockview.Lock
	p := newLivePreview(k, "settings-lock-preview", func(k *kit, cfg *config.Config, font render.Font) widget.Widget {
		return drawLockOnScreen(k, cfg, font, bg, mode)
	})
	bg.redraw = p.configChanged
	labels := make([]string, len(lockPreviewModes))
	for i, m := range lockPreviewModes {
		labels[i] = locale.T().Get(m.label)
	}
	modes := newChoicePick(k, labels, 0)
	modes.AddClass("settings-lock-preview-mode")
	modes.OnSelect = func(i int) {
		mode = lockPreviewModes[i].mode
		p.configChanged()
	}
	column := widget.NewBox(widget.Column, widget.Current().Spacing, 0)
	column.AppendAligned(modes, false, widget.AlignCenter)
	column.Append(p, false)
	return &lockPreviewPanel{Box: column, preview: p}
}

// lockPreviewPanel is the lock preview under the switch between its
// screens.
type lockPreviewPanel struct {
	*widget.Box
	preview *livePreview
}

func (l *lockPreviewPanel) configChanged() { l.preview.configChanged() }

func drawLockOnScreen(k *kit, cfg *config.Config, font render.Font, bg *lockBackdrop, mode lockview.Mode) widget.Widget {
	shown := bg.show(lockBackdropKey{path: cfg.Login.Background.ImagePath(cfg.Wallpaper.Wallpaper), sigma: cfg.Login.Blur.Sigma()})
	spec := lockview.Spec{Mode: mode, Config: cfg.Login, Font: font, Backdrop: shown.backdrop, Opaque: !cfg.Styling.Translucent()}
	fingerprint := lockPreviewSpec(&spec, cfg, font)
	surface := lockview.Build(spec)
	if mode == lockview.Greeter {
		surface.Prompt.Username.SetVisible(!lockview.ListsUsers(cfg.Login.Greeter, len(lockPreviewAccounts())))
	}
	surface.Prompt.SetFingerprint(fingerprint)
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
	note := previewState(k)
	if shown.say(note, cfg.Login.Background) {
		column.Append(note, false)
	}
	return column
}

// lockPreviewSpec fills spec's mode's rows as cfg shows them, with
// sample state, and reports whether the fingerprint prompt stands in for
// the password.
func lockPreviewSpec(spec *lockview.Spec, cfg *config.Config, font render.Font) (fingerprint bool) {
	switch spec.Mode {
	case lockview.Greeter:
		g := cfg.Login.Greeter
		users := lockview.UserRow()
		accounts := lockPreviewAccounts()
		users.SetVisible(lockview.ListsUsers(g, len(accounts)))
		for i, name := range accounts {
			u := lockview.NewUserButton(font, name)
			u.SetSelected(i == 0)
			users.Append(u.Button, false)
		}
		spec.Identity = users
		spec.Greeter.Power, _ = lockview.GreeterFooter(font, g, func(logind.PowerAction) {})
		fingerprint = g.Fingerprint
	case lockview.Lock:
		l := cfg.Login.Lock
		spec.Identity = lockview.NewAccountTile(font, lockPreviewAccounts()[0]).Root
		if l.ShowMedia {
			media := lockview.NewMediaRow(font, func() {}, func() {})
			media.Set(lockPreviewTrack(), true)
			spec.Lock.Status = media.Root
		}
		if l.ShowBattery {
			battery := lockview.NewBatteryChip(font, cfg.Battery.ChargingIcon)
			battery.Set(lockview.BatteryLevel{Percent: lockPreviewBatteryPercent}, true)
			spec.Lock.Corner = battery.Root
		}
		if l.ShowSwitchUser {
			spec.Lock.Footer = lockview.SwitchUserButton(font, func() {})
		}
		fingerprint = l.Fingerprint
	}
	if fingerprint {
		spec.Fingerprint = lockview.NewFingerprint(func() {})
	}
	return fingerprint
}

func lockPreviewAccounts() []string {
	t := locale.T()
	return []string{t.Get("settings-lock-preview-account"), t.Get("settings-lock-preview-second-account")}
}

func lockPreviewTrack() lockview.Track {
	t := locale.T()
	return lockview.Track{Title: t.Get("settings-lock-preview-track"), Artist: t.Get("settings-lock-preview-artist"), Playing: true, CanNext: true}
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

// say says in note what the preview's background is short of under bg,
// and reports whether there is anything to say.
func (s lockBackdropShowing) say(note *inlineState, bg config.Background) bool {
	t := locale.T()
	switch s.state {
	case lockBackdropLoading:
		note.Loading(t.Get("settings-lock-preview-loading"))
	case lockBackdropFailed:
		note.Unavailable("ld-image-symbolic", t.Get("settings-lock-preview-unreadable", i18n.Str("path", s.path)), s.err)
	case lockBackdropNone:
		if bg.Mode == config.BackgroundColor {
			return false
		}
		note.Empty("ld-image-symbolic", t.Get("settings-lock-preview-no-image"))
	case lockBackdropShown:
		return false
	}
	return true
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
		bg, err := lockview.LoadBackdrop(&b.mem, imagedecode.OpenOffHeapOriented, key.path, key.sigma, []image.Point{{X: lockPreviewScreenW, Y: lockPreviewScreenH}})
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
