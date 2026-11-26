package kiosk

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/logtest"
	"github.com/stubbedev/lys/lib/testhome"
	"github.com/stubbedev/lys/lib/xdg"
	"github.com/stubbedev/lys/lib/xkb"
	"go.yaml.in/yaml/v3"

	"github.com/stubbedev/lys/login/internal/greeter"
	"github.com/stubbedev/lys/login/internal/keyboard"
	"github.com/stubbedev/lys/login/internal/keyboard/keyboardtest"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

func TestKioskConfigQuotesTheGreeter(t *testing.T) {
	got := Config{Greet: []string{"/nix/store/x/bin/lys-login", "greet", "--sessions", "/run/my sessions", `--env`, `A="b"`}, Cursor: greeter.Cursor{Size: 24}}.YAML()
	want := "binds: []\n" + look + lid + "env:\n  XCURSOR_SIZE: \"24\"\nautostart:\n  - \"/nix/store/x/bin/lys-login greet --sessions '/run/my sessions' --env 'A=\\\"b\\\"'\"\n"
	if got != want {
		t.Errorf("kiosk config =\n%s\nwant\n%s", got, want)
	}
}

func TestKioskConfigDrawsNoBorder(t *testing.T) {
	var cfg struct {
		Borders struct {
			Width *int `yaml:"width"`
		} `yaml:"borders"`
	}
	if err := yaml.Unmarshal([]byte(Config{Greet: []string{"lys-login", "greet"}, Cursor: greeter.Cursor{Size: 24}}.YAML()), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Borders.Width == nil || *cfg.Borders.Width != 0 {
		t.Errorf("borders.width = %v, want 0 so lys-wm draws no border around the greeter", cfg.Borders.Width)
	}
}

// A kiosk config leaves out nothing lys-wm's default look would draw around
// the greeter or animate on the way to it.
func TestKioskConfigTurnsOffTheDefaultLook(t *testing.T) {
	var cfg struct {
		Gaps struct {
			Inner *int `yaml:"inner"`
			Outer *int `yaml:"outer"`
		} `yaml:"gaps"`
		Decoration struct {
			Rounding *int `yaml:"rounding"`
			Shadow   struct {
				Enabled *bool `yaml:"enabled"`
			} `yaml:"shadow"`
		} `yaml:"decoration"`
		Animations struct {
			Enabled *bool `yaml:"enabled"`
		} `yaml:"animations"`
	}
	if err := yaml.Unmarshal([]byte(Config{Greet: []string{"lys-login", "greet"}, Cursor: greeter.Cursor{Size: 24}}.YAML()), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Gaps.Inner == nil || *cfg.Gaps.Inner != 0 || cfg.Gaps.Outer == nil || *cfg.Gaps.Outer != 0 {
		t.Errorf("gaps = %+v, want 0 so the greeter reaches the edges", cfg.Gaps)
	}
	if cfg.Decoration.Rounding == nil || *cfg.Decoration.Rounding != 0 {
		t.Errorf("decoration.rounding = %v, want 0 so no corner shows lys-wm's background", cfg.Decoration.Rounding)
	}
	if cfg.Decoration.Shadow.Enabled == nil || *cfg.Decoration.Shadow.Enabled {
		t.Errorf("decoration.shadow.enabled = %v, want false", cfg.Decoration.Shadow.Enabled)
	}
	if cfg.Animations.Enabled == nil || *cfg.Animations.Enabled {
		t.Errorf("animations.enabled = %v, want false so the greeter does not fade in over lys-wm's background", cfg.Animations.Enabled)
	}
}

func TestKioskConfigLeavesTheLidToLogind(t *testing.T) {
	var cfg struct {
		LidAction *string `yaml:"lid_action"`
	}
	if err := yaml.Unmarshal([]byte(Config{Greet: []string{"lys-login", "greet"}, Cursor: greeter.Cursor{Size: 24}}.YAML()), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.LidAction == nil || *cfg.LidAction != "logind" {
		t.Errorf("lid_action = %v, want logind so lys-wm takes no lid inhibitor the greeter user may not hold", cfg.LidAction)
	}
}

// lys-wm draws the greeter's cursor, from the theme and size in the
// kiosk config's environment, which the greeter inherits.
func TestKioskConfigCarriesTheCursor(t *testing.T) {
	var cfg struct {
		Env map[string]string `yaml:"env"`
	}
	if err := yaml.Unmarshal([]byte(Config{Greet: []string{"lys-login", "greet"}, Cursor: greeter.Cursor{Theme: `Bibata "Ice"`, Size: 48}}.YAML()), &cfg); err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"XCURSOR_THEME": `Bibata "Ice"`, "XCURSOR_SIZE": "48"}; !reflect.DeepEqual(cfg.Env, want) {
		t.Errorf("env = %v, want %v", cfg.Env, want)
	}
	cfg.Env = nil
	if err := yaml.Unmarshal([]byte(Config{Greet: []string{"lys-login", "greet"}, Cursor: greeter.Cursor{Size: 24}}.YAML()), &cfg); err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"XCURSOR_SIZE": "24"}; !reflect.DeepEqual(cfg.Env, want) {
		t.Errorf("no theme: env = %v, want %v, lys-wm's own theme", cfg.Env, want)
	}
}

type kioskKeyboard struct {
	Input struct {
		Keyboard map[string]string `yaml:"keyboard"`
	} `yaml:"input"`
	Env map[string]string `yaml:"env"`
}

func greeterKiosk(t *testing.T, s Sources) kioskKeyboard {
	t.Helper()
	var cfg kioskKeyboard
	if err := yaml.Unmarshal([]byte(Config{Greet: []string{"lys-login", "greet"}, Cursor: greeter.Cursor{Size: 24}, Keyboard: s.KeyboardNames()}.YAML()), &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func keyboardSources(t *testing.T, xorg string) Sources {
	t.Helper()
	dir := t.TempDir()
	s := Sources{Keyboard: keyboard.Sources{Xorg: filepath.Join(dir, "00-keyboard.conf"), Vconsole: filepath.Join(dir, "vconsole.conf")}}
	if xorg != "" {
		if err := os.WriteFile(s.Keyboard.Xorg, []byte(xorg), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestTheKioskTypesWithLocaledsLayout(t *testing.T) {
	keyboardtest.XKBData(t)
	keyboardtest.ServeLocale1(t, xkb.Names{Model: "pc105", Layout: "de,us", Variant: "nodeadkeys,", Options: "grp:alt_shift_toggle"})
	cfg := greeterKiosk(t, keyboardSources(t, `Option "XkbLayout" "fr"`))
	want := map[string]string{"model": "pc105", "layout": "de,us", "variant": "nodeadkeys,", "options": "grp:alt_shift_toggle"}
	if !reflect.DeepEqual(cfg.Input.Keyboard, want) {
		t.Errorf("input.keyboard = %v, want %v", cfg.Input.Keyboard, want)
	}
	for name := range cfg.Env {
		if strings.HasPrefix(name, "XKB_") {
			t.Errorf("the kiosk sets %s: lys-wm takes the layout from input.keyboard", name)
		}
	}
}

func TestWithoutLocaledTheKioskTypesWithTheXorgFilesLayout(t *testing.T) {
	keyboardtest.XKBData(t)
	keyboardtest.NoSystemBus(t)
	cfg := greeterKiosk(t, keyboardSources(t, "Section \"InputClass\"\n  Option \"XkbLayout\" \"fr\"\nEndSection\n"))
	if want := map[string]string{"layout": "fr"}; !reflect.DeepEqual(cfg.Input.Keyboard, want) {
		t.Errorf("input.keyboard = %v, want %v", cfg.Input.Keyboard, want)
	}
}

func TestALayoutLysWMCannotCompileLeavesTheKioskOnItsDefault(t *testing.T) {
	keyboardtest.XKBData(t)
	keyboardtest.ServeLocale1(t, xkb.Names{Layout: "no-such-layout"})
	logged := logtest.Capture(t)
	cfg := greeterKiosk(t, keyboardSources(t, ""))
	if cfg.Input.Keyboard != nil {
		t.Errorf("input.keyboard = %v, want none, so lys-wm still starts", cfg.Input.Keyboard)
	}
	if !strings.Contains(logged.String(), "no-such-layout") {
		t.Errorf("the refused layout was not logged: %q", logged.String())
	}
}

func TestNoSystemLayoutLeavesInputOut(t *testing.T) {
	if got := (Config{Greet: []string{"lys-login", "greet"}, Cursor: greeter.Cursor{Size: 24}}).YAML(); strings.Contains(got, "input:") {
		t.Errorf("a kiosk without a system layout names one:\n%s", got)
	}
}

func TestTheRememberedStickyAndSlowKeysReachTheKiosk(t *testing.T) {
	state := filepath.Join(t.TempDir(), "last-session")
	if err := greeter.SaveLast(greeter.AccessibilityPath(state), "sticky-keys\nslow-keys\nlarge-text\n"); err != nil {
		t.Fatal(err)
	}
	s := Sources{StatePath: state}
	var cfg kioskKeyboard
	if err := yaml.Unmarshal([]byte(Config{Greet: []string{"lys-login", "greet"}, Cursor: greeter.Cursor{Size: 24}, Keyboard: xkb.Names{Layout: "de"}, A11y: s.Accessibility()}.YAML()), &cfg); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"layout": "de", "sticky_keys": "true", "slow_keys": "300"}
	if !reflect.DeepEqual(cfg.Input.Keyboard, want) {
		t.Errorf("input.keyboard = %v, want %v", cfg.Input.Keyboard, want)
	}
	var bare kioskKeyboard
	if err := yaml.Unmarshal([]byte(Config{Greet: []string{"lys-login", "greet"}, Cursor: greeter.Cursor{Size: 24}, A11y: greeter.Accessibility{greeter.StickyKeys: true}}.YAML()), &bare); err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"sticky_keys": "true"}; !reflect.DeepEqual(bare.Input.Keyboard, want) {
		t.Errorf("without a layout input.keyboard = %v, want %v", bare.Input.Keyboard, want)
	}
}

func TestWriteReplacesTheKioskConfigReadableByAll(t *testing.T) {
	dir := t.TempDir()
	c := Config{Greet: []string{"lys-login", "greet"}, Cursor: greeter.Cursor{Size: 24}}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := Write(dir, c)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, FileName); path != want {
		t.Errorf("path = %s, want %s", path, want)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != c.YAML() {
		t.Errorf("wrote\n%s\nwant\n%s", got, c.YAML())
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("mode = %v, want 0644 so the compositor's user reads it", perm)
	}
}

func TestWriteIntoAMissingDirectoryIsAnError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "absent")
	if _, err := Write(dir, Config{}); err == nil || !strings.Contains(err.Error(), dir) {
		t.Errorf("Write into %s = %v, want an error naming it", dir, err)
	}
}

func TestExecWithoutARuntimeDirIsALoudError(t *testing.T) {
	for _, dir := range []string{"", "run/user/1000"} {
		if err := Exec(runtimeEnv(dir), "lys-wm", Config{}); !errors.Is(err, ErrNoRuntimeDir) {
			t.Errorf("Exec with XDG_RUNTIME_DIR=%q = %v, want %v", dir, err, ErrNoRuntimeDir)
		}
	}
}

func TestExecOfAMissingCompositorWritesNothing(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "no-such-lys-wm")
	if err := Exec(runtimeEnv(dir), missing, Config{}); err == nil || !strings.Contains(err.Error(), missing) {
		t.Errorf("Exec of %s = %v, want an error naming it", missing, err)
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a kiosk config was written for a compositor that does not exist: %v", err)
	}
}

func runtimeEnv(dir string) xdg.Env {
	return func(name string) string {
		if name == "XDG_RUNTIME_DIR" {
			return dir
		}
		return ""
	}
}
