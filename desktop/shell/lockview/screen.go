package lockview

import (
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
)

// Mode is the situation the login screen is shown in.
type Mode int

// The modes: the greeter before login, and the lock over a session.
const (
	Greeter Mode = iota
	Lock
)

// GreeterRows are the rows only the greeter shows: the session picker
// under the entries and the power buttons at the bottom of the screen.
// Either may be nil.
type GreeterRows struct {
	Session, Power widget.Widget
}

// LockRows are the rows only the lock screen shows: the media row under
// the clock, the battery chip in the corner and the switch-user footer.
// Any may be nil.
type LockRows struct {
	Status, Corner, Footer widget.Widget
}

// Spec is one login screen surface.
type Spec struct {
	Mode   Mode
	Config config.LoginConfig
	Font   render.Font
	Sheet  *widget.Stylesheet
	// Backdrop is the background image, nil for the color fill.
	Backdrop *Backdrop
	// Identity is who the screen asks for, above the entries: the lock
	// screen's account, the greeter's user list. nil for none.
	Identity widget.Widget
	// Greeter holds the greeter's own rows; Build reads it in greeter
	// mode only.
	Greeter     GreeterRows
	Lock        LockRows
	Fingerprint *Fingerprint
	// Focus moves keyboard focus.
	Focus func(widget.Widget)
	// OnSubmit receives the secret entry's text on Enter.
	OnSubmit   func(string)
	NextLayout func()
}

// Surface is a built login screen and the handles its mode drives.
type Surface struct {
	Root   widget.Widget
	Prompt *Prompt
	// Scrim is the blank-screen blackout over everything.
	Scrim *widget.Box
}

// Cover is the screen's background alone, the layer the greeter keeps
// under a session it hands over to.
func Cover(s Spec) widget.Widget {
	return Window(s.Sheet, Background(s.Backdrop, HexFill(s.Config.Background.Color)))
}

// Build assembles a login screen: the background, the clock, the account
// asked for and the credential prompt with the caps lock warning, and the
// blank scrim, the same in every mode; the greeter adds its username
// entry and its rows.
func Build(s Spec) *Surface {
	o := Options{
		Font: s.Font, ShowClock: s.Config.ShowClock, Identity: s.Identity,
		Focus: s.Focus, NextLayout: s.NextLayout, Fingerprint: s.Fingerprint,
	}
	switch s.Mode {
	case Greeter:
		greeter := i18n.Greeter()
		o.Username, o.Password, o.CapsLock = greeter.Get("greeter-username"), greeter.Get("greeter-password"), greeter.Get("greeter-caps-lock")
		o.Extra, o.Footer = s.Greeter.Session, s.Greeter.Power
	case Lock:
		o.Password, o.CapsLock = i18n.T("lock-password"), i18n.T("lock-caps-lock")
		o.Status, o.Corner, o.Footer = s.Lock.Status, s.Lock.Corner, s.Lock.Footer
	}
	surface := &Surface{Prompt: newPrompt(o, s.OnSubmit), Scrim: Scrim()}
	surface.Root = Window(s.Sheet,
		Background(s.Backdrop, HexFill(s.Config.Background.Color)), surface.Prompt.Reveal, surface.Scrim)
	return surface
}
