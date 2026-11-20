package settings

import (
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/desktop/service/wallpaper/extract"
	"github.com/stubbedev/lys/desktop/styling/material"
	"github.com/stubbedev/lys/ui/widget"
)

func TestTheThemingWallpaperIsTheOneTheShellThemesFrom(t *testing.T) {
	w := config.WallpaperConfig{Wallpaper: "/all.png", Monitors: []config.MonitorWallpaperConfig{{Name: "DP-1", Wallpaper: "/dp.png"}, {Name: "HDMI-A-1"}}}
	for _, tc := range []struct {
		theming string
		outputs []string
		want    string
	}{
		{"DP-1", nil, "/dp.png"},
		{"HDMI-A-1", nil, "/all.png"},
		{"", []string{"HDMI-A-1", "DP-1"}, "/dp.png"},
		{"", []string{"HDMI-A-1"}, "/all.png"},
		{"", nil, "/all.png"},
	} {
		if got, cycling := themingWallpaper(w, tc.theming, tc.outputs); got != tc.want || cycling {
			t.Errorf("theming %q over %v: %q, cycling %v; want %q", tc.theming, tc.outputs, got, cycling, tc.want)
		}
	}
	w.CyclingDirectory = "/walls"
	if got, cycling := themingWallpaper(w, "DP-1", nil); got != "" || !cycling {
		t.Errorf("while cycling the theming wallpaper is %q, cycling %v; want none, cycling", got, cycling)
	}
}

func twoHueWallpaper(t *testing.T) string {
	t.Helper()
	img := solidImage(500, 100, color.RGBA{R: 0x20, G: 0x60, B: 0xd0, A: 0xff})
	for y := range 100 {
		for x := 300; x < 500; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 0xd0, G: 0x40, B: 0x20, A: 0xff})
		}
	}
	path := filepath.Join(t.TempDir(), "two.png")
	savePNG(t, path, img)
	return path
}

func sourceSwatchesOf(t *testing.T, configYAML string) (*kit, *sourceSwatches) {
	t.Helper()
	k := testKit(t, configYAML)
	return k, fieldControl(k, k.store, pageRow(t, "theme", "styling.matugen-source-color")).(*sourceSwatches)
}

func cardLabels(c *choiceCards) []string {
	var out []string
	for _, card := range c.cards {
		out = append(out, card.label)
	}
	return out
}

func TestTheSourceColourIsPickedFromTheWallpapersSwatches(t *testing.T) {
	path := twoHueWallpaper(t)
	img, err := imagedecode.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	want := extract.SourceColors(material.SampleImage(img, extract.SampleSide))
	if len(want) != 2 {
		t.Fatalf("the fixture offers %d candidates, want 2", len(want))
	}
	k, c := sourceSwatchesOf(t, "styling:\n  theme-provider: matugen\nwallpaper:\n  wallpaper: "+path+"\n")
	if got := cardLabels(c.shown); len(got) != 2 || got[0] != want[0].Hex() || got[1] != want[1].Hex() {
		t.Fatalf("the swatches read %v, want %s and %s", got, want[0].Hex(), want[1].Hex())
	}
	if c.shown.cards[0].pic.draw == nil {
		t.Error("a swatch draws no colour")
	}
	if c.note.Visible() || c.problem.Visible() || c.shown.current != "0" {
		t.Errorf("note %q, problem %q, picked %q; want the first swatch picked and nothing else said", c.note.Text(), c.problem.Text(), c.shown.current)
	}
	c.shown.cards[1].KeyAction(widget.KeyEnter, 0)
	if got := k.store.svc.Config().Styling.ColorExtractor.MatugenSourceColor; got != 1 {
		t.Errorf("picking the second swatch stored %d, want 1", got)
	}
	if err := k.store.set("styling.matugen-source-color", int64(9)); err != nil {
		t.Fatal(err)
	}
	c.refresh()
	if c.shown.current != "1" {
		t.Errorf("an index past the swatches marks %q, want the last one the extractor falls back to", c.shown.current)
	}
}

func TestTheSourceColourSaysWhyItShowsNoSwatches(t *testing.T) {
	broken := filepath.Join(t.TempDir(), "broken.png")
	if err := os.WriteFile(broken, []byte("not a picture"), 0o600); err != nil {
		t.Fatal(err)
	}
	k, c := sourceSwatchesOf(t, "wallpaper:\n  wallpaper: "+broken+"\n")
	_, decodeErr := imagedecode.Open(broken)
	if !c.problem.Visible() || !strings.Contains(c.problem.Text(), broken) || !strings.Contains(c.problem.Text(), decodeErr.Error()) {
		t.Errorf("an unreadable wallpaper shows the problem %q (shown %v)", c.problem.Text(), c.problem.Visible())
	}
	if n := len(c.shown.cards); n != extract.SourceColorChoices || c.shown.cards[0].pic.draw != nil {
		t.Errorf("an unreadable wallpaper offers %d cards, want %d ranks without a colour", n, extract.SourceColorChoices)
	}
	c.shown.cards[2].KeyAction(widget.KeyEnter, 0)
	if got := k.store.svc.Config().Styling.ColorExtractor.MatugenSourceColor; got != 2 {
		t.Errorf("picking the third rank stored %d, want 2", got)
	}
	_, c = sourceSwatchesOf(t, "wallpaper:\n  cycling-directory: "+t.TempDir()+"\n")
	if !c.note.Visible() || c.problem.Visible() || len(c.shown.cards) != extract.SourceColorChoices {
		t.Errorf("while cycling: note %q (shown %v), problem %q, %d cards", c.note.Text(), c.note.Visible(), c.problem.Text(), len(c.shown.cards))
	}
	_, c = sourceSwatchesOf(t, "")
	if !c.note.Visible() || c.problem.Visible() || c.note.Text() == "" {
		t.Errorf("without a wallpaper: note %q (shown %v), problem %q", c.note.Text(), c.note.Visible(), c.problem.Text())
	}
}
