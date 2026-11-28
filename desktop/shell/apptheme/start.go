package apptheme

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/logging"
)

// Session is a desktop app on the compositor: its application, its
// theme following the config it started with, the sans-serif face at
// the app's size, and the outputs the compositor announces.
type Session struct {
	App     *app.Application
	Theme   *Theme
	Face    *render.Typeface
	Outputs func() []*app.Output
	undo    []func()
}

// Start connects the app named name, as lys-files, to the compositor
// and themes it from cfg: the toolkit logs through desktop/logging
// before the connection, the app's own icons and lys-shell's join the
// search path, and the stylesheet follows the user's styles, all
// before any window opens. Close undoes it.
func Start(name string, cfg *config.Config, fontPx float64) (*Session, error) {
	app.SetLogger(logging.Library())
	sess, err := app.Connect()
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	s := &Session{App: app.NewApplication(sess), Outputs: sess.Outputs, undo: []func(){sess.Close}}
	ShipIcons(name)
	s.undo = append(s.undo, Setup(s.App))
	s.Theme = New(cfg)
	s.Theme.WatchUserStyles(s.App)
	s.Face, err = app.Font(cfg.General.FontSans, fontPx)
	if err != nil {
		s.Close()
		return nil, fmt.Errorf("font %q: %w", cfg.General.FontSans, err)
	}
	return s, nil
}

// Close stops the icons watch and closes the connection.
func (s *Session) Close() {
	for _, undo := range slices.Backward(s.undo) {
		undo()
	}
	s.undo = nil
}

// ShipIcons puts the icons the app named name installs on the search
// path: they live in its own share directory beside its program, so it
// needs no lys-shell and owns no file of its.
func ShipIcons(name string) { shipIcons(name, os.Executable, lysUIIcons) }

func shipIcons(name string, executable func() (string, error), search iconSearch) {
	exe, err := executable()
	if err != nil {
		log.Printf("%s: icons: find this program: %v", name, err)
		return
	}
	search.set(append(search.paths(), iconsBeside(exe, name)))
}

func iconsBeside(exe, name string) string {
	return filepath.Join(filepath.Dir(exe), "..", "share", name, "icons")
}
