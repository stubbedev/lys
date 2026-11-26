// Package kiosk is lys-wm's config for the greeter, under the display
// manager and under greetd alike: lys-wm hosts the greeter as its one
// program, with no binds and none of its default look.
package kiosk

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shlex"
	"github.com/stubbedev/lys/lib/atomicfile"
	"github.com/stubbedev/lys/lib/xdg"
	"github.com/stubbedev/lys/lib/xkb"

	"github.com/stubbedev/lys/login/internal/greeter"
	"github.com/stubbedev/lys/login/internal/keyboard"
)

const FileName = "kiosk.yaml"

// look is the part of lys-wm's default look a kiosk turns off: no
// border, gaps, rounding or shadow, so the greeter fills the screen to
// its edges, and no animations, so its first frame on screen is the
// whole greeter, and the hand-over from the splash shows nothing between
// the two, not lys-wm's empty background through a fade.
const look = "borders:\n  width: 0\ngaps:\n  inner: 0\n  outer: 0\ndecoration:\n  rounding: 0\n  shadow:\n    enabled: false\nanimations:\n  enabled: false\n"

type Config struct {
	Greet    []string
	Cursor   greeter.Cursor
	Keyboard xkb.Names
	A11y     greeter.Accessibility
}

// YAML is lys-wm's config for the greeter: no binds, so the greeter's
// keys reach only it, look, the system keyboard layout, the greeter as
// the one program, and the cursor in the environment, from which lys-wm
// draws the cursor the greeter asks for and which the greeter inherits.
func (c Config) YAML() string {
	quoted := make([]string, len(c.Greet))
	for i, arg := range c.Greet {
		quoted[i] = shlex.MustQuote(arg)
	}
	var b strings.Builder
	b.WriteString("binds: []\n" + look)
	writeKeyboard(&b, c.Keyboard, c.A11y)
	b.WriteString("env:\n  XCURSOR_SIZE: " + yamlString(strconv.FormatUint(uint64(c.Cursor.Size), 10)) + "\n")
	if c.Cursor.Theme != "" {
		b.WriteString("  XCURSOR_THEME: " + yamlString(c.Cursor.Theme) + "\n")
	}
	b.WriteString("autostart:\n  - " + yamlString(strings.Join(quoted, " ")) + "\n")
	return b.String()
}

func writeKeyboard(b *strings.Builder, names xkb.Names, a11y greeter.Accessibility) {
	keys := a11y.WMConfig()
	if names == (xkb.Names{}) && keys == "" {
		return
	}
	b.WriteString("input:\n  keyboard:\n" + keys)
	for _, f := range [...]struct{ key, value string }{{"model", names.Model}, {"layout", names.Layout}, {"variant", names.Variant}, {"options", names.Options}} {
		if f.value != "" {
			b.WriteString("    " + f.key + ": " + yamlString(f.value) + "\n")
		}
	}
}

func yamlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

type Sources struct {
	ConfigPath string
	StatePath  string
	CursorDir  string
	Keyboard   keyboard.Sources
}

func (s Sources) Settings() config.LoginGreeterConfig {
	cfg, err := config.LoadGreeter(s.ConfigPath, config.StderrDiagnostics)
	if err != nil {
		log.Printf("kiosk: the greeter's config: %v", err)
	}
	return cfg.Login.Greeter
}

func (s Sources) Config(settings config.LoginGreeterConfig, users []greeter.User, greet []string) Config {
	return Config{Greet: greet, Cursor: s.Cursor(settings, users), Keyboard: s.KeyboardNames(), A11y: s.Accessibility()}
}

// Cursor is the cursor the next greeter shows: the greeter's config,
// else the record the remembered user's last session left.
func (s Sources) Cursor(settings config.LoginGreeterConfig, users []greeter.User) greeter.Cursor {
	return greeter.KioskCursor(settings, s.CursorDir, users, greeter.LoadLast(greeter.LastUserPath(s.StatePath)))
}

func (s Sources) KeyboardNames() xkb.Names {
	r, err := s.Keyboard.Read(func(err error) {
		log.Printf("kiosk: %v; the greeter's keyboard layout comes from %s, else %s", err, s.Keyboard.Xorg, s.Keyboard.Vconsole)
	})
	if err != nil {
		log.Printf("kiosk: the greeter types with lys-wm's default keymap: %v", err)
		return xkb.Names{}
	}
	if err := keyboard.Check(r.Names); err != nil {
		log.Printf("kiosk: the greeter types with lys-wm's default keymap: the layout from %s: %v", r.From, err)
		return xkb.Names{}
	}
	return r.Names
}

func (s Sources) Accessibility() greeter.Accessibility {
	a, err := greeter.LoadAccessibility(s.StatePath)
	if err != nil {
		log.Printf("kiosk: the greeter starts without sticky or slow keys: %v", err)
	}
	return a
}

func Write(dir string, c Config) (string, error) {
	path := filepath.Join(dir, FileName)
	if err := atomicfile.WriteFile(path, []byte(c.YAML()), 0o644); err != nil {
		return "", fmt.Errorf("write the kiosk config %s: %w", path, err)
	}
	return path, nil
}

var ErrNoRuntimeDir = errors.New("XDG_RUNTIME_DIR names no absolute directory, so there is nowhere to write the kiosk config; run it in a login session, as greetd starts its greeter")

func Exec(env xdg.Env, compositor string, c Config) error {
	runtimeDir, ok := env.RuntimeHome()
	if !ok {
		return ErrNoRuntimeDir
	}
	bin, err := exec.LookPath(compositor)
	if err != nil {
		return fmt.Errorf("find the compositor %q: %w", compositor, err)
	}
	path, err := Write(runtimeDir, c)
	if err != nil {
		return err
	}
	if err := syscall.Exec(bin, []string{bin, "--config", path}, os.Environ()); err != nil { //nolint:gosec // the compositor the greeter's command line names
		return fmt.Errorf("start the compositor %s: %w", bin, err)
	}
	return nil
}
