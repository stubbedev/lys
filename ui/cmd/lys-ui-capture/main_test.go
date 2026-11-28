package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/lib/capture"
)

func TestWritePNGReplacesTheFileWhole(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shots", "a.png")
	frame := &capture.Frame{Width: 2, Height: 1, Stride: 8, Format: capture.FormatXRGB8888, Data: make([]byte, 8)}
	if err := writePNG(frame, path); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if img, err := png.Decode(f); err != nil || img.Bounds().Dx() != 2 || img.Bounds().Dy() != 1 {
		t.Fatalf("decoded %v, %v; want the 2x1 frame", img, err)
	}
	if entries, err := os.ReadDir(filepath.Dir(path)); err != nil || len(entries) != 1 {
		t.Errorf("the directory holds %v, %v; want the PNG alone", entries, err)
	}
}

func TestWritePNGOntoADirectoryFailsNamingIt(t *testing.T) {
	path := t.TempDir()
	frame := &capture.Frame{Width: 1, Height: 1, Stride: 4, Format: capture.FormatXRGB8888, Data: make([]byte, 4)}
	if err := writePNG(frame, path); err == nil {
		t.Fatalf("writing over the directory %s succeeded", path)
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Errorf("the directory became %v, %v", info, err)
	}
}
