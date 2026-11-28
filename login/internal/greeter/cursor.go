package greeter

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/xdg"
	"github.com/stubbedev/lys/ui/gtksettings"

	"github.com/stubbedev/lys/login/internal/cursorrecord"
)

// The greeter runs pre-login, so there is no session to ask what
// cursor the user sees (cursor.rs). The record the user's last session
// handed the display manager comes first (cursorrecord); then the
// user's own dotfiles, best effort, as only a home the greeter account
// may enter yields them: the configuration of the compositor matching
// the remembered session (niri's cursor block, sway's seat
// xcursor_theme), the XCURSOR_* variables of the user's
// ~/.config/environment.d, which every systemd user session (Hyprland's
// among them) starts with, the other compositors' configuration, then
// GTK settings.ini and ~/.icons/default/index.theme.
//
// Like GTK 4.22, lys-ui asks the compositor to draw the cursor
// (cursor-shape-v1) when it can; on a host without the protocol
// it draws xcursor themes client-side from XCURSOR_THEME and
// XCURSOR_SIZE, so the resolved cursor is applied by exporting those
// before connecting. The bundled-PNG fallback the Rust greeter works
// around does not exist here.

// Cursor is what one source found; either side may be missing.
type Cursor = cursorrecord.Cursor

// DetectCursor is the cursor u's session shows: the record under
// records, then the dotfiles in u's home; session is the remembered
// session id, whose compositor is consulted first. A dotfile the
// greeter can read but not parse is logged and skipped.
func DetectCursor(records string, u User, session string) Cursor {
	home := u.Home
	read := func(rel string) string {
		data, _ := os.ReadFile(filepath.Join(home, rel)) //nolint:gosec // best-effort dotfile read
		return string(data)
	}
	found := recordedCursor(records, u.UID)
	if found.Complete() {
		return found
	}
	niri := func() Cursor { return parseNiri(read(".config/niri/config.kdl")) }
	sway := func() Cursor { return parseSway(read(".config/sway/config")) }
	environment := func() Cursor {
		c, err := environmentDCursor(filepath.Join(home, environmentDDir))
		if err != nil {
			log.Printf("greeter: %v", err)
		}
		return c
	}
	sources := []func() Cursor{environment, niri, sway}
	switch session = strings.ToLower(session); {
	case strings.Contains(session, "niri"):
		sources = []func() Cursor{niri, environment, sway}
	case strings.Contains(session, "sway"):
		sources = []func() Cursor{sway, environment, niri}
	}
	sources = append(sources,
		func() Cursor { return parseGTKSettings(read(".config/gtk-4.0/settings.ini")) },
		func() Cursor { return parseGTKSettings(read(".config/gtk-3.0/settings.ini")) },
		func() Cursor { return parseIndexTheme(read(".icons/default/index.theme")) },
	)
	for _, source := range sources {
		if found.Complete() {
			break
		}
		found = found.Or(source())
	}
	return found
}

// cursorUser is whose cursor the greeter shows: the remembered user's,
// else the only user's; false for neither.
func cursorUser(users []User, lastUser string) (User, bool) {
	for _, u := range users {
		if u.Name == lastUser && lastUser != "" {
			return u, true
		}
	}
	if len(users) == 1 {
		return users[0], true
	}
	return User{}, false
}

// detectLastCursor is the cursor cursorUser's last session showed,
// none without such a user or with match-cursor off.
func detectLastCursor(init Init) Cursor {
	u, ok := cursorUser(init.Users, init.LastUser)
	if !ok || !init.Config.Login.Greeter.MatchCursor {
		return Cursor{}
	}
	return DetectCursor(cursorrecord.Dir, u, init.LastSession)
}

// KioskCursor is the cursor the display manager has the greeter's
// kiosk compositor draw: ResolveCursor over the record the last
// session of lastUser, else of the only user, left under records. It
// reads no dotfiles: the display manager runs as root, and opens
// nothing a user controls.
func KioskCursor(g config.LoginGreeterConfig, records string, users []User, lastUser string) Cursor {
	var recorded Cursor
	if u, ok := cursorUser(users, lastUser); ok && g.MatchCursor {
		recorded = recordedCursor(records, u.UID)
	}
	return ResolveCursor(g, recorded)
}

// ResolveCursor is the cursor the greeter shows. With match-cursor on:
// what the last session showed (detected), the XCURSOR_* environment,
// then cursor-theme and cursor-size. With it off: cursor-theme and
// cursor-size, an empty theme leaving the environment's. The theme may
// stay "".
func ResolveCursor(g config.LoginGreeterConfig, detected Cursor) Cursor {
	configured := Cursor{Theme: g.CursorTheme, Size: g.CursorSize}
	env, _, err := cursorrecord.FromEnv(xdg.OS)
	if err != nil {
		log.Printf("greeter: the environment's cursor: %v", err)
	}
	resolved := configured.Or(env)
	if g.MatchCursor {
		resolved = detected.Or(env).Or(configured)
	}
	resolved.Size = max(resolved.Size, 1)
	return resolved
}

// ApplyCursor exports the resolved cursor for lys-ui's theme loader.
func ApplyCursor(c Cursor) {
	if c.Theme != "" {
		_ = os.Setenv("XCURSOR_THEME", c.Theme)
	}
	_ = os.Setenv("XCURSOR_SIZE", strconv.FormatUint(uint64(c.Size), 10))
}

// recordedCursor is the cursor uid's last session recorded under
// records; none when it left no record or the record cannot be read,
// which is logged.
func recordedCursor(records string, uid uint32) Cursor {
	c, err := cursorrecord.Read(records, uid)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Printf("greeter: %v", err)
		}
		return Cursor{}
	}
	return c
}

const environmentDDir = ".config/environment.d"

func environmentDCursor(dir string) (Cursor, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
		return Cursor{}, nil
	}
	if err != nil {
		return Cursor{}, fmt.Errorf("read the cursor from %s: %w", dir, err)
	}
	var c Cursor
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path) //nolint:gosec // best-effort dotfile read
		if err != nil {
			return Cursor{}, fmt.Errorf("read the cursor from %s: %w", path, err)
		}
		file, err := parseEnvironmentD(string(data))
		if err != nil {
			return Cursor{}, fmt.Errorf("read the cursor from %s: %w", path, err)
		}
		c = file.Or(c)
	}
	return c, nil
}

func parseEnvironmentD(text string) (Cursor, error) {
	var c Cursor
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" || strings.ContainsAny(key, " \t") {
			return Cursor{}, fmt.Errorf("line %d: %q is no KEY=VALUE assignment", i+1, line)
		}
		if key != "XCURSOR_THEME" && key != "XCURSOR_SIZE" {
			continue
		}
		value = unquote(strings.TrimSpace(value))
		if strings.Contains(value, "$") {
			return Cursor{}, fmt.Errorf("line %d: %s=%s references a variable", i+1, key, value)
		}
		if key == "XCURSOR_THEME" {
			c.Theme = value
			continue
		}
		size, err := strconv.ParseUint(value, 10, 32)
		if err != nil || size == 0 {
			return Cursor{}, fmt.Errorf("line %d: XCURSOR_SIZE=%s is no positive size", i+1, value)
		}
		c.Size = uint32(size)
	}
	return c, nil
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// parseNiri scans niri's KDL for xcursor-theme and xcursor-size.
func parseNiri(text string) Cursor {
	var c Cursor
	for line := range strings.SplitSeq(text, "\n") {
		line, _, _ = strings.Cut(line, "//")
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "xcursor-theme"); ok {
			if v := strings.Trim(strings.TrimSpace(rest), `"`); c.Theme == "" && v != "" {
				c.Theme = v
			}
		} else if rest, ok := strings.CutPrefix(line, "xcursor-size"); ok && c.Size == 0 {
			c.Size = cursorrecord.ParseSize(rest)
		}
	}
	return c
}

// parseSway reads `seat <name> xcursor_theme <theme> [size]`.
func parseSway(text string) Cursor {
	var c Cursor
	for line := range strings.SplitSeq(text, "\n") {
		line, _, _ = strings.Cut(line, "#")
		tok := strings.Fields(line)
		if len(tok) < 4 || tok[0] != "seat" || tok[2] != "xcursor_theme" {
			continue
		}
		if c.Theme == "" {
			c.Theme = strings.Trim(tok[3], `"`)
		}
		if c.Size == 0 && len(tok) > 4 {
			c.Size = cursorrecord.ParseSize(tok[4])
		}
	}
	return c
}

// parseGTKSettings reads gtk-cursor-theme-name/-size.
func parseGTKSettings(text string) Cursor {
	s := gtksettings.Parse(text)
	return Cursor{Theme: s.CursorThemeName, Size: s.CursorThemeSize}
}

// parseIndexTheme reads the Inherits= of ~/.icons/default/index.theme
// (theme only).
func parseIndexTheme(text string) Cursor {
	for line := range strings.SplitSeq(text, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "Inherits" {
			continue
		}
		first, _, _ := strings.Cut(value, ",")
		return Cursor{Theme: strings.TrimSpace(first)}
	}
	return Cursor{}
}
