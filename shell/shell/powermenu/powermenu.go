// Package powermenu is the native power menu: a
// full-screen dimmed overlay with a centered pane of frosted glass
// holding lock, log out, suspend, reboot and shut-down buttons, each
// running its [modules.power] command. The power module's `:menu`
// binding opens it, and the dashboard's session buttons go through it
// (Request). It opens with a button focused; the arrows, like Tab,
// move the window's focus along the buttons, Return presses the
// focused one and Escape dismisses it. It enters and leaves through
// the power surface's transition, and a button's command runs once
// the menu is gone.
package powermenu

import (
	"log"
	"math"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/layers"
	"github.com/stubbedev/lys/desktop/shell/reveal"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
)

const (
	// iconPx is an action glyph's size at scale 1.
	iconPx = 24
	// buttonPx is an action tile's side at scale 1.
	buttonPx = 96
	// chipPx is the side of the disc behind an action's glyph at
	// scale 1.
	chipPx = 48
)

// escape dismisses the menu.
var escape, _ = app.ParseAccel("Escape")

// Action is one of the menu's power actions.
type Action int

// The actions, in the menu's order.
const (
	Lock Action = iota
	LogOut
	Suspend
	Reboot
	ShutDown
)

// actions are every Action, in the menu's order.
var actions = [...]Action{Lock, LogOut, Suspend, Reboot, ShutDown}

// spec is an action's icon, label, command and visibility, and
// whether it ends the session.
type spec struct {
	icon, labelID string
	danger        bool
	command       func(config.PowerConfig) string
	visible       func(config.PowerConfig) bool
}

var specs = [...]spec{
	Lock:     {"ld-lock-symbolic", "power-menu-lock", false, func(c config.PowerConfig) string { return c.LockCommand }, func(c config.PowerConfig) bool { return c.ShowLock }},
	LogOut:   {"ld-log-out-symbolic", "power-menu-log-out", true, func(c config.PowerConfig) string { return c.LogoutCommand }, func(c config.PowerConfig) bool { return c.ShowLogout }},
	Suspend:  {"ld-moon-symbolic", "power-menu-suspend", false, func(c config.PowerConfig) string { return c.SuspendCommand }, func(c config.PowerConfig) bool { return c.ShowSuspend }},
	Reboot:   {"ld-rotate-ccw-symbolic", "power-menu-reboot", true, func(c config.PowerConfig) string { return c.RebootCommand }, func(c config.PowerConfig) bool { return c.ShowReboot }},
	ShutDown: {"ld-power-symbolic", "power-menu-shut-down", true, func(c config.PowerConfig) string { return c.ShutdownCommand }, func(c config.PowerConfig) bool { return c.ShowShutdown }},
}

// Icon is the action's glyph.
func (a Action) Icon() string { return specs[a].icon }

// Label is the action's name in the user's language.
func (a Action) Label() string { return i18n.T(specs[a].labelID) }

// Danger reports whether the action ends the session.
func (a Action) Danger() bool { return specs[a].danger }

// Command is the action's [modules.power] command.
func (a Action) Command(c config.PowerConfig) string { return specs[a].command(c) }

// shownActions are the actions c shows, in order, with also among
// them wherever c hides it.
func shownActions(c config.PowerConfig, also ...Action) []Action {
	out := make([]Action, 0, len(actions))
	for _, a := range actions {
		if specs[a].visible(c) || slices.Contains(also, a) {
			out = append(out, a)
		}
	}
	return out
}

// Window is the overlay surface the menu maps (app.LayerWindow).
type Window interface {
	Close()
	SetFocus(widget.Widget) error
}

// Deps are the menu's collaborators.
type Deps struct {
	// Config is the live config; commands are read on click.
	Config func() *config.Config
	// Open maps the overlay.
	Open func(cfg app.LayerConfig) (Window, error)
	// Run runs a command without waiting for it (spawn.Quiet).
	Run func(cmd string)
	// Font shapes the labels; Sheet (the power-menu-* rules) styles
	// the overlay.
	Font  render.Font
	Sheet *widget.Stylesheet
}

// Menu is the power menu. Its methods run on the UI loop.
type Menu struct {
	d   Deps
	win Window
	rev *widget.Revealer
	// closing: the exit is playing; a Show reopens a fresh surface.
	closing bool
}

// New builds the menu; nothing maps until Show.
func New(d Deps) *Menu { return &Menu{d: d} }

// Show opens the menu with the current config's buttons and the first
// focused, or does nothing while it is already up.
func (m *Menu) Show() {
	cfg := m.d.Config()
	m.open(cfg, shownActions(cfg.Power), 0)
}

// Request runs a the way the menu would: an action that ends the
// session opens the menu on it, shown though the config hides it, so
// only a second press runs it; any other runs at once. While the menu
// is up a request that needs it does nothing.
func (m *Menu) Request(a Action) {
	cfg := m.d.Config()
	if !a.Danger() {
		m.run(a.Command(cfg.Power))
		return
	}
	shown := shownActions(cfg.Power, a)
	m.open(cfg, shown, slices.Index(shown, a))
}

func (m *Menu) open(cfg *config.Config, shown []Action, focus int) {
	if m.win != nil && !m.closing {
		return
	}
	scale := float64(cfg.Styling.Scale)
	th := widget.Current()
	row := &buttonRow{Box: widget.NewBox(widget.Row, th.Spacing, 3*th.Padding), buttons: make([]*widget.Button, 0, len(shown))}
	row.AddClass(css.Glass, "power-menu-surface")
	row.SetHomogeneous(true)
	for _, a := range shown {
		b := m.button(a, scale)
		row.buttons = append(row.buttons, b)
		row.Append(b, false)
	}
	m.rev = widget.NewRevealer(widgets.Center(row))
	reveal.Show(m.rev, cfg.Animations, config.AnimPower)
	root := widget.NewBox(widget.Column, 0, 0)
	root.AddClass("power-menu-window")
	if m.d.Sheet != nil {
		root.AttachStylesheet(m.d.Sheet)
	}
	root.Append(m.rev, true)
	win, err := m.d.Open(app.LayerConfig{
		Layer:          app.LayerOverlay,
		Anchor:         app.AnchorTop | app.AnchorBottom | app.AnchorLeft | app.AnchorRight,
		ExclusiveZone:  -1,
		Keyboard:       app.KeyboardExclusive,
		Namespace:      layers.PowerMenu.Name(),
		Root:           root,
		BackgroundBlur: cfg.Styling.Translucent(),
		KeyCapture: func(a app.Accel) bool {
			if a.Sym != escape.Sym { // whatever modifiers are held
				return false
			}
			m.Cancel()
			return true
		},
	})
	if err != nil {
		log.Printf("power menu: cannot map the overlay: %v", err)
		return
	}
	m.win, m.closing = win, false
	if focus < 0 || focus >= len(row.buttons) {
		return
	}
	if err := win.SetFocus(row.buttons[focus]); err != nil {
		log.Printf("power menu: focus button %d: %v", focus, err)
	}
}

type buttonRow struct {
	*widget.Box
	buttons []*widget.Button
}

func (r *buttonRow) NavigateFocus(target widget.Widget, a widget.KeyAction) widget.Widget {
	switch a {
	case widget.KeyLeft, widget.KeyUp:
		return widget.MoveFocusAmong(r.buttons, target, widget.FocusMovePrev)
	case widget.KeyRight, widget.KeyDown:
		return widget.MoveFocusAmong(r.buttons, target, widget.FocusMoveNext)
	}
	return nil
}

// scaled is px at the styling scale.
func scaled(px int, scale float64) int { return int(math.Round(float64(px) * scale)) }

// button is one action: a borderless square tile, pinned to its side
// at the styling scale, with the glyph on a disc over the label, which
// ellipsizes; its command is re-read on click.
func (m *Menu) button(a Action, scale float64) *widget.Button {
	icon := widget.NewThemeIcon(a.Icon(), scaled(iconPx, scale))
	icon.AddClass("power-menu-icon")
	label := widget.NewLabel(m.d.Font, widgets.TextPx(styling.Body, scale), a.Label(), 0)
	label.AddClass("power-menu-label", css.DimLabel)
	label.SetAlignment(render.AlignCenter)
	label.SetEllipsize(widget.EllipsizeEnd)
	content := widget.NewBox(widget.Column, widget.Current().Spacing, 0)
	content.SetVExpand(true)
	chip := widget.NewBox(widget.Column, 0, 0)
	chip.AddClass("power-menu-chip")
	widgets.Pin(chip, scaled(chipPx, scale), scaled(chipPx, scale))
	chip.Append(widgets.Center(icon), true)
	content.AppendAligned(chip, false, widget.AlignCenter)
	content.Append(label, false)
	b := widget.NewButton(widgets.Center(content), 0, 0)
	b.AddClass("power-menu-button", css.Flat)
	if a.Danger() {
		b.AddClass("danger")
	}
	b.SetTooltip(a.Label())
	widgets.Pin(b, scaled(buttonPx, scale), scaled(buttonPx, scale))
	b.OnClick = func() { m.hide(a.Command(m.d.Config().Power)) }
	return b
}

// Cancel plays the exit and closes the menu.
func (m *Menu) Cancel() { m.hide("") }

func (m *Menu) hide(then string) {
	if m.win == nil || m.closing {
		return
	}
	m.closing = true
	win := m.win
	reveal.Hide(m.rev, m.d.Config().Animations, config.AnimPower, func() {
		win.Close()
		if m.win == win {
			m.win, m.rev, m.closing = nil, nil, false
		}
		m.run(then)
	})
}

// run runs cmd; an empty one runs nothing.
func (m *Menu) run(cmd string) {
	if cmd != "" {
		m.d.Run(cmd)
	}
}

// Open reports whether the menu is up (its exit not begun).
func (m *Menu) Open() bool { return m.win != nil && !m.closing }
