package launcher

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/launcherview"

	"github.com/stubbedev/lys/shell/internal/launcheripc"
)

func TestASessionsFontAndStyleOverrideTheConfiguredLook(t *testing.T) {
	cfg := config.Defaults()
	cfg.Launcher.History.Enable = false
	cfg.Launcher.Font = "Inter 12"
	cfg.Launcher.Styles = map[string]string{"compact": ".launcher-row { padding: 0; }"}
	rows := make(chan []string)
	close(rows)
	look := func(opts launcheripc.SessionOptions) launcherview.Look {
		opts.Dmenu = true
		return buildSession(opts, cfg, rows, setupDeps{}).ui.look
	}
	if got := look(launcheripc.SessionOptions{}); got.Font != "Inter 12" || got.Style != "" {
		t.Errorf("no options = font %q style %q, want the configured font alone", got.Font, got.Style)
	}
	if got := look(launcheripc.SessionOptions{Font: new("Monospace 9"), Style: new("compact")}); got.Font != "Monospace 9" || got.Style != cfg.Launcher.Styles["compact"] {
		t.Errorf("-font and -style = font %q style %q", got.Font, got.Style)
	}
	if got := look(launcheripc.SessionOptions{Style: new("nope")}); got.Style != "" {
		t.Errorf("an unknown preset contributed %q", got.Style)
	}
}

func TestBuildSessionResolvesTheUI(t *testing.T) {
	cfg := config.Defaults()
	cfg.Launcher.History.Enable = false
	cfg.Styling.Scale = 2
	lines := uint32(4)
	loc := uint8(1) // rofi's north-west
	opts := launcheripc.SessionOptions{Dmenu: true, Lines: &lines, Location: &loc, KeepRight: true, Prompt: new("pick")}
	rows := make(chan []string)
	close(rows)
	setup := buildSession(opts, cfg, rows, setupDeps{})
	if len(setup.modes) != 1 || setup.modes[0].Name() != "dmenu" {
		t.Fatalf("modes = %v", setup.modes)
	}
	ui := setup.ui
	if ui.look.Lines != 4 || ui.display.Ellipsize != launcherview.EllipsizeStart || deref(ui.prompt, "") != "pick" {
		t.Errorf("ui = %+v", ui)
	}
	if want := int(cfg.Launcher.Width.ResolvePx(config.LauncherWidthBaseRem*16, 2)); ui.look.Width != want {
		t.Errorf("width %d, want %d at styling scale 2", ui.look.Width, want)
	}
	if ui.look.Location != config.LauncherNorthWest {
		t.Errorf("location = %s", ui.look.Location)
	}
	// An out-of-range rofi location keeps the configured one.
	bad := uint8(200)
	opts.Location = &bad
	if got := buildSession(opts, cfg, rows, setupDeps{}).ui.look.Location; got != cfg.Launcher.Location {
		t.Errorf("bad location = %s", got)
	}
}
