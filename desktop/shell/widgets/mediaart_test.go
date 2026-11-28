package widgets

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/lib/fileuri"
	"github.com/stubbedev/lys/lib/render"
)

func TestMediaArtLoadsALocalCoverWithEscapes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "My Music")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "cover #1.png")
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	img.SetNRGBA(0, 0, color.NRGBA{R: 0xff, A: 0xff})
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
	art, ok := MediaArt(fileuri.FromPath(path))
	if !ok {
		t.Fatal("a local cover has no art")
	}
	art.Arrange(render.Rect{W: 4, H: 4})
	art.Paint(render.New(make([]byte, render.Stride(4)*4), render.Stride(4), 4, 4))
	if !art.Loaded() || art.Err() != nil {
		t.Errorf("cover at %s did not load: %v", path, art.Err())
	}
}

func TestMediaArtHasNoneForARemoteFileOrAnotherScheme(t *testing.T) {
	for _, url := range []string{"file://otherhost/covers/a.png", "data:image/png;base64,AAAA", ""} {
		if _, ok := MediaArt(url); ok {
			t.Errorf("MediaArt(%q) has art", url)
		}
	}
}
