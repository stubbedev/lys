package filebrowser

import (
	"errors"
	"fmt"

	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/shell/keys"
)

// Window is the window a Standalone maps the browser on.
type Window interface {
	Close()
	SetFocus(widget.Widget) error
}

// WindowSpec is one window to map: its title, its tree, and the hooks
// its keys and pointer buttons go to.
type WindowSpec struct {
	Title   string
	Root    widget.Widget
	OnKey   func(r *widget.Router, code uint32, mods app.Mods)
	OnPress func(button uint32, target widget.Widget)
}

// WindowDeps are what a Standalone maps its window with: Open maps it,
// KeySym reads its keys through the seat's keymap, Sheet styles it.
type WindowDeps struct {
	Open   func(WindowSpec) (Window, error)
	KeySym keys.Translator
	Sheet  *widget.Stylesheet
}

// Standalone shows the browser in a window of its own: a browse with
// no request behind it and nothing to answer. It runs on the loop.
type Standalone struct {
	b   *Browser
	w   WindowDeps
	win Window
}

// errNoWindow is a focus asked for while no window is mapped.
var errNoWindow = errors.New("no file browser window is mapped")

// NewStandalone builds the host; nothing maps until Show.
func NewStandalone(d Deps, w WindowDeps) *Standalone {
	s := &Standalone{w: w}
	s.b = New(d, s.setFocus)
	return s
}

// Browser is the browser the window shows.
func (s *Standalone) Browser() *Browser { return s.b }

func (s *Standalone) setFocus(w widget.Widget) error {
	if s.win == nil {
		return errNoWindow
	}
	return s.win.SetFocus(w)
}

// Show maps a window browsing as o says, its file view focused, in
// place of the one showing. A window that cannot map, or cannot take
// the focus, is an error, and nothing shows.
func (s *Standalone) Show(o Options) error {
	s.Hide()
	s.b.Begin(o)
	root := widget.NewBox(widget.Column, 0, 0)
	root.SetElement("window")
	if s.w.Sheet != nil {
		root.AttachStylesheet(s.w.Sheet)
	}
	root.Append(s.b.Header(), false)
	root.Append(s.b.Main(), true)
	win, err := s.w.Open(WindowSpec{
		Title: o.Title, Root: root,
		OnKey:   keys.OnKey(s.w.KeySym, s.b.Key),
		OnPress: func(button uint32, _ widget.Widget) { s.b.Press(button) },
	})
	if err != nil {
		s.b.End()
		s.b.Release()
		return fmt.Errorf("mapping the file browser on %q: %w", s.b.Dir(), err)
	}
	s.win = win
	if err := win.SetFocus(s.b.FileView()); err != nil {
		s.Hide()
		return fmt.Errorf("focusing the file browser on %q: %w", o.Folder, err)
	}
	return nil
}

// Hide ends the browse and closes its window, freeing what it drew;
// with none showing it does nothing.
func (s *Standalone) Hide() {
	if s.win == nil {
		return
	}
	s.b.End()
	s.win.Close()
	s.win = nil
	s.b.Release()
}

// Close ends the host once the loop has ended (Browser.Close).
func (s *Standalone) Close() { s.b.Close() }
