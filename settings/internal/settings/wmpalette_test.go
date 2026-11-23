package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

func paletteResetsShown(t *testing.T, p *paletteGrid) map[string]bool {
	t.Helper()
	if len(p.resets) != len(p.shown) || len(p.tiles) != len(p.shown) {
		t.Fatalf("%d resets and %d tiles for the colours %q", len(p.resets), len(p.tiles), p.shown)
	}
	shown := make(map[string]bool, len(p.shown))
	for i, name := range p.shown {
		shown[name] = p.resets[i].shown()
	}
	return shown
}

func paletteSwatch(t *testing.T, p *paletteGrid, name string) *colorEditor {
	t.Helper()
	i := slices.Index(p.shown, name)
	if i < 0 {
		t.Fatalf("the grid shows %q, not %s", p.shown, name)
	}
	return p.tiles[i].(*colorEditor)
}

func TestOverridingABuiltInPaletteColourShowsItsResetAtOnce(t *testing.T) {
	k, w, _, files := wmKit(t, "")
	p := fieldControl(k, w, wmPaletteRow()).(*paletteGrid)
	if got := paletteResetsShown(t, p); got["red"] || got["blue"] {
		t.Fatalf("an untouched palette shows resets %v", got)
	}

	paletteSwatch(t, p, "red").OnColorSet(render.RGBA(0, 255, 0, 255))
	if got := paletteResetsShown(t, p); !got["red"] || got["blue"] {
		t.Fatalf("overriding red shows resets %v, want red's alone", got)
	}
	if got := fileText(t, files.runtime); !strings.Contains(got, "red: '#00ff00'") {
		t.Fatalf("overriding red wrote %q", got)
	}

	p.resets[slices.Index(p.shown, "red")].OnClick()
	if got := paletteResetsShown(t, p); got["red"] || got["blue"] {
		t.Fatalf("resetting red shows resets %v", got)
	}
	if got := fileText(t, files.runtime); strings.Contains(got, "red") {
		t.Fatalf("resetting red kept %q", got)
	}
	if c := paletteSwatch(t, p, "red").Color(); c != render.RGB(0xf3, 0x8b, 0xa8) {
		t.Errorf("resetting red shows %v, not the built-in colour", c)
	}
}

func TestAPaletteColourAtItsBuiltInValueShowsNoReset(t *testing.T) {
	k, w, _, _ := wmKit(t, "")
	p := fieldControl(k, w, wmPaletteRow()).(*paletteGrid)
	paletteSwatch(t, p, "blue").OnColorSet(render.RGB(0x89, 0xb4, 0xfa))
	paletteSwatch(t, p, "red").OnColorSet(render.RGB(0, 0, 0xff))
	if got := paletteResetsShown(t, p); got["blue"] || !got["red"] {
		t.Fatalf("blue at its built-in value and red overridden show resets %v", got)
	}
}

func TestAnOverrideReadAtStartShowsItsReset(t *testing.T) {
	k, w, _, _ := wmKit(t, "palette:\n  red: '#00ff00'\n  mint: '#98ff98'\n")
	p := fieldControl(k, w, wmPaletteRow()).(*paletteGrid)
	if got := paletteResetsShown(t, p); !got["red"] || !got["mint"] || got["blue"] {
		t.Fatalf("a configured red and mint show resets %v, want theirs alone", got)
	}
}
