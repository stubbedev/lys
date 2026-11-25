package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
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

// A palette change that cannot be written says so in the app's words,
// the error only in the closed detail: a reset keeps its override, an
// added colour its name; once a change goes through the line is gone.
func TestAPaletteChangeThatCannotBeWrittenSaysSoInTheAppsWords(t *testing.T) {
	k, w, _, _ := wmKit(t, "palette:\n  red: '#00ff00'\n")
	p := fieldControl(k, w, wmPaletteRow()).(*paletteGrid)
	tr := locale.T()
	own := p.own
	p.own = slot{get: own.get, set: func(any) error { return actionFailure("write the palette") }, unset: own.unset}
	said := func(want string) {
		t.Helper()
		if !p.problem.Visible() || p.problem.kind != stateError || p.problem.Text() != want || !strings.Contains(p.problem.detail.Text(), "dial unix") || p.problem.detail.Visible() {
			t.Fatalf("the palette says %q over %q, shown %v, want %q", p.problem.Text(), p.problem.detail.Text(), p.problem.Visible(), want)
		}
	}
	p.resets[slices.Index(p.shown, "red")].OnClick()
	said(tr.Get("wm-palette-reset-failed", i18n.Str("name", "red")))
	if got := paletteResetsShown(t, p); !got["red"] {
		t.Fatalf("a refused reset shows resets %v, want red's kept", got)
	}
	p.name.SetText("mint")
	p.add()
	said(tr.Get("wm-palette-add-failed", i18n.Str("name", "mint")))
	if p.name.Text() != "mint" || slices.Contains(p.shown, "mint") {
		t.Fatalf("a refused colour left the name %q and the colours %q", p.name.Text(), p.shown)
	}
	p.own = own
	p.add()
	if p.problem.Visible() || p.name.Text() != "" || !slices.Contains(p.shown, "mint") {
		t.Fatalf("an added colour says %q, leaves the name %q, shows %q", p.problem.Text(), p.name.Text(), p.shown)
	}
	p.name.SetText("red")
	p.add()
	if p.problem.Text() != tr.Get("wm-palette-taken", i18n.Str("name", "red")) || p.problem.reveal.button.Visible() {
		t.Errorf("a taken name says %q, offers a detail %v", p.problem.Text(), p.problem.reveal.button.Visible())
	}
}

// The palette's tiles were as wide as their names, so the grid stood in
// no columns, and the reset kept its height beside the name, leaving air
// between each name and its button. Every tile is one width, so the
// tiles of a line start at even steps, and the button sits right under
// its name.
func TestThePaletteTilesStandInColumnsWithTheirButtonUnderTheirName(t *testing.T) {
	k, w, _, _ := wmKit(t, "palette:\n  a: '#000000'\n  rosewater: '#f5e0dc'\n")
	p := fieldControl(k, w, wmPaletteRow()).(*paletteGrid)
	themed(k, p)
	tiles := treetest.WithClass(p, "wm-palette-tile")
	if len(tiles) < 3 {
		t.Fatalf("the palette shows %d tiles", len(tiles))
	}
	bounds := func(w widget.Widget) render.Rect { return w.(widget.Boundser).Bounds() }
	first, second, third := bounds(tiles[0]), bounds(tiles[1]), bounds(tiles[2])
	if first.W != second.W || second.W != third.W || second.X-first.X != third.X-second.X {
		t.Errorf("the first tiles span %+v, %+v, %+v: not one width at even steps", first, second, third)
	}
	for i, tile := range tiles {
		label := treetest.All[*widget.Label](tile)[0]
		caption := label.Bounds()
		button := p.tiles[i].(*colorEditor).Bounds()
		if gap := button.Y - caption.Y - label.Measure(widget.Column, caption.W).Nat; gap > choiceCardGap/2 {
			t.Errorf("%s's button stands %dpx under its name", p.shown[i], gap)
		}
	}
}

func TestAPaletteTileNamesItsColourAsTheColourPickersDo(t *testing.T) {
	k, w, _, _ := wmKit(t, "palette:\n  overlay0: '#6c7086'\n")
	p := fieldControl(k, w, wmPaletteRow()).(*paletteGrid)
	var captions []string
	for _, l := range treetest.All[*widget.Label](p.grid) {
		captions = append(captions, l.Text())
	}
	if !slices.Contains(captions, "Overlay 0") || !slices.Contains(captions, "Red") {
		t.Fatalf("the tiles read %q, want Overlay 0 and Red among them", captions)
	}
	if slices.Contains(captions, "overlay0") {
		t.Errorf("a tile reads its raw key: %q", captions)
	}
}
