package launcher

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	img.Set(1, 1, color.NRGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

var ctx = context.Background()

func TestPreviewOfEncodedBytesDecodesThem(t *testing.T) {
	got, err := LoadPreview(ctx, Preview{Load: func() ([]byte, error) { return pngBytes(t), nil }})
	if err != nil || got.Image == nil {
		t.Fatalf("got %+v, %v", got, err)
	}
	if _, err := LoadPreview(ctx, Preview{Load: func() ([]byte, error) { return []byte("junk"), nil }}); err == nil {
		t.Error("undecodable bytes are an error")
	}
	gone := errors.New("gone")
	if _, err := LoadPreview(ctx, Preview{Load: func() ([]byte, error) { return nil, gone }}); !errors.Is(err, gone) {
		t.Errorf("err = %v", err)
	}
}

func TestPreviewOfTextIsThatText(t *testing.T) {
	got, err := LoadPreview(ctx, Preview{Text: "x"})
	if err != nil || got.Text != "x" {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestPreviewOfTextBytesThatAreNotTextIsNothing(t *testing.T) {
	for _, data := range [][]byte{{}, []byte("a\x00b"), {0xff, 0xfe}} {
		if _, err := LoadPreview(ctx, Preview{Load: func() ([]byte, error) { return data, nil }, LoadsText: true}); !errors.Is(err, ErrNoPreview) {
			t.Errorf("%q: err = %v, want ErrNoPreview", data, err)
		}
	}
	got, err := LoadPreview(ctx, Preview{Load: func() ([]byte, error) { return []byte("é ok"), nil }, LoadsText: true})
	if err != nil || got.Text != "é ok" {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestFactsAloneHaveNothingToLoad(t *testing.T) {
	if _, err := LoadPreview(ctx, Preview{Facts: []PreviewFact{{"a", "b"}}}); !errors.Is(err, ErrNoPreview) {
		t.Errorf("facts: %v", err)
	}
	if _, err := LoadPreview(ctx, Preview{}); !errors.Is(err, ErrNoPreview) {
		t.Errorf("zero preview: %v", err)
	}
}
