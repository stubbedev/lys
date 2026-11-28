package bar

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/lib/logtest"
	"github.com/stubbedev/lys/ui/widget"
)

func TestScreenshotBindingRunsInProcess(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	var shots []string
	var reads []config.ScreenReading
	ctx.Screenshot = func(mode config.ScreenshotMode, output string) { shots = append(shots, string(mode)+"|"+output) }
	ctx.ReadScreen = func(kind config.ScreenReading) { reads = append(reads, kind) }
	spawned := spawns(t)
	runClickAction(ctx, ctx.Config.Screenshot.Clicks().RightClick)
	for _, line := range []string{"lys-shell screenshot output HDMI-A-1", "lys-shell screenshot", "lys-shell screenshot screen", "lys-shell screenshot text", "lys-shell screenshot qr"} {
		runClickAction(ctx, config.ParseClickAction(line))
	}
	if !slices.Equal(shots, []string{"output|", "output|HDMI-A-1", "region|", "screen|"}) || !slices.Equal(reads, config.ScreenReadings) {
		t.Fatalf("captured %v, read %v", shots, reads)
	}
	runClickAction(ctx, config.ParseClickAction("lys-shell audio output-mute"))
	runClickAction(ctx, config.ParseClickAction("lys-shell screenshots region"))
	if len(shots) != 4 || len(reads) != 2 {
		t.Fatalf("a foreign binding reached the screenshot host: %v %v", shots, reads)
	}
	if got := spawned(); !slices.Equal(got, []string{"lys-shell screenshots region"}) {
		t.Fatalf("spawned %q, want only the foreign command", got)
	}
}

func TestAScreenshotBindingWithoutTheHostIsLogged(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	logs := logtest.Capture(t)
	spawned := spawns(t)
	runClickAction(ctx, config.ParseClickAction("lys-shell screenshot window"))
	runClickAction(ctx, config.ParseClickAction("lys-shell screenshot qr"))
	for _, want := range []string{"click: lys-shell screenshot window: no screenshot host", "click: lys-shell screenshot qr: no screenshot host"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("log %q lacks %q", logs.String(), want)
		}
	}
	if got := spawned(); len(got) != 0 {
		t.Fatalf("spawned %q", got)
	}
}

func TestScreenshotModule(t *testing.T) {
	cfg := config.Defaults()
	m, err := newScreenshot(newTestContext(t, cfg))
	if err != nil {
		t.Fatal(err)
	}
	if btn, ok := m.Root().(*barButton); !ok || shownIcon(btn) == nil || btn.Label().Visible() {
		t.Fatalf("default root %T, want an icon-only bar button", m.Root())
	}
	cfg.Screenshot.Label, cfg.Screenshot.LabelShow, cfg.Screenshot.LabelMaxLength = "Capture", true, 4
	m, err = newScreenshot(newTestContext(t, cfg))
	if err != nil {
		t.Fatal(err)
	}
	btn, ok := m.Root().(*barButton)
	if !ok || shownIcon(btn) == nil {
		t.Fatalf("labelled root %T, want the icon + label bar button", m.Root())
	}
	btn.configure(barview.ModuleTitle("screenshot", cfg), barview.ModuleButton("screenshot", cfg), config.ClickConfig{}, nil)
	if got := btn.Label().Text(); got != "Capture" || btn.Label().MaxWidthChars() != 4 || btn.Label().Ellipsize() != widget.EllipsizeEnd {
		t.Fatalf("label %q capped at %d, want Capture capped at 4 chars", got, btn.Label().MaxWidthChars())
	}
	// No label-max-length: no cap, no ellipsis.
	cfg.Screenshot.LabelMaxLength = 0
	btn.configure(barview.ModuleTitle("screenshot", cfg), barview.ModuleButton("screenshot", cfg), config.ClickConfig{}, nil)
	if btn.Label().MaxWidthChars() != 0 || btn.Label().Ellipsize() != widget.EllipsizeNone {
		t.Fatalf("uncapped label: %d chars, ellipsize %v", btn.Label().MaxWidthChars(), btn.Label().Ellipsize())
	}
	cfg.Screenshot.LabelMaxLength = 4
	cfg.Screenshot.IconShow, cfg.Screenshot.LabelShow = false, false
	if _, err := newScreenshot(newTestContext(t, cfg)); err == nil {
		t.Fatal("a module with nothing to show was built")
	}
	cfg.Screenshot.LabelShow = true
	m, err = newScreenshot(newTestContext(t, cfg))
	if err != nil {
		t.Fatal(err)
	}
	if btn, ok := m.Root().(*barButton); !ok || shownIcon(btn) != nil || btn.Label().Text() != "Capture" {
		t.Fatalf("label-only root %T, want a bar button with only a label", m.Root())
	}
}
