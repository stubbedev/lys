package settings

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func savePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func solidImage(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func wallpaperGridOf(t *testing.T, configYAML string) (*kit, *wallpaperGrid) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	k := testKit(t, configYAML)
	return k, fieldControl(k, k.store, pageRow(t, "wallpaper", "wallpaper.wallpaper")).(*wallpaperGrid)
}

func tilePaths(g *wallpaperGrid) []string {
	var out []string
	for _, tile := range g.tiles {
		out = append(out, tile.path)
	}
	return out
}

func TestTheWallpaperGridShowsThePickerFoldersPicturesAndSetsOne(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.png")
	broken := filepath.Join(dir, "broken.jpg")
	savePNG(t, good, solidImage(600, 300, color.RGBA{R: 0x30, G: 0x60, B: 0xc0, A: 0xff}))
	if err := os.WriteFile(broken, []byte("not a picture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not listed"), 0o600); err != nil {
		t.Fatal(err)
	}
	k, g := wallpaperGridOf(t, "wallpaper:\n  picker-directories: ["+dir+"]\n")
	if got := tilePaths(g); !slices.Equal(got, []string{broken, good}) {
		t.Fatalf("the grid shows %v, want the two pictures of %s", got, dir)
	}
	shown, failed := g.tiles[1], g.tiles[0]
	if shown.thumb.img == nil || shown.thumb.err != nil {
		t.Errorf("the readable picture's tile holds %+v, want its thumbnail", shown.thumb)
	}
	if b := shown.thumb.img.Bounds(); max(b.Dx(), b.Dy()) > 256 {
		t.Errorf("the thumbnail is %v, larger than a large thumbnail", b)
	}
	if name := widget.Describe(shown).Name; name != "good.png" {
		t.Errorf("the readable tile is named %q", name)
	}
	_, decodeErr := imagedecode.Open(broken)
	if failed.thumb.img != nil || failed.thumb.err == nil || !strings.Contains(failed.thumb.err.Error(), broken) || !strings.Contains(failed.thumb.err.Error(), decodeErr.Error()) {
		t.Errorf("the broken picture's tile holds %+v, want an error naming %s and %v", failed.thumb, broken, decodeErr)
	}
	if name := widget.Describe(failed).Name; name != "broken.jpg: "+locale.T().Get("settings-wallpaper-thumb-failed") || rawError(name) {
		t.Errorf("the broken tile is named %q, want its name and the app's summary", name)
	}
	cached, err := filepath.Glob(filepath.Join(os.Getenv("XDG_CACHE_HOME"), "thumbnails", "large", "*.png"))
	if err != nil || len(cached) != 1 {
		t.Errorf("the thumbnail cache holds %v (%v), want the one readable picture's thumbnail", cached, err)
	}
	if g.problem.Visible() || g.note.Visible() {
		t.Errorf("a listed folder shows the note %q or the problem %q", g.note.Text(), g.problem.Text())
	}
	shown.KeyAction(widget.KeyEnter, 0)
	if got := k.store.value("wallpaper.wallpaper"); got != good {
		t.Errorf("choosing the tile set the wallpaper to %v, want %s", got, good)
	}
	if g.current() != good || failed.path == g.current() {
		t.Errorf("the grid marks %q as the wallpaper", g.current())
	}
}

func TestTheWallpaperGridSaysWhyItShowsNoPictures(t *testing.T) {
	empty := t.TempDir()
	_, g := wallpaperGridOf(t, "wallpaper:\n  picker-directories: ["+empty+"]\n")
	if len(g.tiles) != 0 || g.grid.Visible() || !g.note.Visible() || !strings.Contains(g.note.Text(), empty) {
		t.Errorf("an empty folder shows %d tiles and the note %q (shown %v)", len(g.tiles), g.note.Text(), g.note.Visible())
	}
	if g.problem.Visible() {
		t.Errorf("an empty folder is a problem: %q", g.problem.Text())
	}
	notDir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notDir, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	k, g := wallpaperGridOf(t, "wallpaper:\n  picker-directories: ["+notDir+"]\n")
	if !failedWith(g.problem, locale.T().Get("settings-wallpaper-grid-failed"), notDir) {
		t.Errorf("a folder that cannot be read shows the problem %q over %q (shown %v)", g.problem.Text(), g.problem.detail.Text(), g.problem.Visible())
	}
	dir := t.TempDir()
	savePNG(t, filepath.Join(dir, "a.png"), solidImage(8, 8, color.RGBA{R: 0xff, A: 0xff}))
	if err := k.store.set("wallpaper.picker-directories", []any{dir}); err != nil {
		t.Fatal(err)
	}
	g.refresh()
	if got := tilePaths(g); !slices.Equal(got, []string{filepath.Join(dir, "a.png")}) || g.problem.Visible() {
		t.Errorf("after the folders change the grid shows %v, problem %q", got, g.problem.Text())
	}
}

func TestTheThumbnailCacheDecodesOnePictureAtATimeOffTheLoop(t *testing.T) {
	k := testKit(t, "")
	var jobs []func()
	k.background = func(fn func()) { jobs = append(jobs, fn) }
	var decoded []string
	loaded := 0
	c := newThumbCache(k, func(path string) wallpaperThumb {
		decoded = append(decoded, path)
		return wallpaperThumb{err: errors.New(path)}
	}, func() { loaded++ })
	for _, p := range []string{"/a", "/b", "/a", "", "/c"} {
		c.get(p)
	}
	if len(jobs) != 1 || len(decoded) != 0 {
		t.Fatalf("asking for three pictures started %d jobs and decoded %v on the loop", len(jobs), decoded)
	}
	for len(jobs) > 0 {
		job := jobs[0]
		jobs = jobs[1:]
		job()
		if len(jobs) > 1 {
			t.Fatalf("%d decodes run at once", len(jobs))
		}
	}
	if !slices.Equal(decoded, []string{"/a", "/b", "/c"}) || loaded != 3 {
		t.Errorf("decoded %v with %d loads, want each picture once in order", decoded, loaded)
	}
	c.get("/a")
	c.close()
	c.get("/d")
	if len(jobs) != 0 {
		t.Errorf("a cached or closed cache started %d more decodes", len(jobs))
	}
}
