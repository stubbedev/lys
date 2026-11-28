// Package imagedecode opens raster images in the formats wallpapers can
// use: PNG, JPEG, GIF, BMP, TIFF, WebP (stdlib and golang.org/x/image),
// HEIF and JPEG XL (github.com/gen2brain/h265 and jxl, pure Go), plus
// farbfeld, PNM (P1-P6), and TGA, which Go has no decoder for and this
// package registers itself. SVG and AVIF are listed as wallpaper
// extensions but are not decoded.
package imagedecode

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"  // GIF decoder
	_ "image/jpeg" // JPEG decoder
	_ "image/png"  // PNG decoder
	"io"
	"os"
	"slices"

	"github.com/gen2brain/h265/heic"
	"github.com/gen2brain/jxl"
	_ "golang.org/x/image/bmp"  // BMP decoder
	_ "golang.org/x/image/tiff" // TIFF decoder
	_ "golang.org/x/image/webp" // WebP decoder
)

func init() {
	image.RegisterFormat("farbfeld", "farbfeld", decodeFarbfeld, decodeFarbfeldConfig)
	for _, magic := range []string{"P1", "P2", "P3", "P4", "P5", "P6"} {
		image.RegisterFormat("pnm", magic, decodePNM, decodePNMConfig)
	}
}

// The MIME types of the formats measure knows without a format name.
const (
	typeJPEG = "image/jpeg"
	typeTGA  = "image/x-tga"
)

// What decoding a HEIF or JPEG XL picture holds besides its pixels: a
// HEIF decoder's own heicFixedBytes, and jxlThreadBytes for each of the
// jxlThreads goroutines a JPEG XL picture decodes on.
const (
	heicFixedBytes = 4 << 20
	jxlThreads     = 4
	jxlThreadBytes = 2 << 20
)

// format is a format Open decodes: the names image.DecodeConfig gives
// it, its MIME type and file extensions, the decoder lys reads it
// through where its package's image.Decode registration does not show
// it as lys does, and what decoding it allocates besides the raster: an
// amount a pixel and a fixed one.
type format struct {
	names      []string
	mimeType   string
	exts       []string
	decode     func(io.Reader) (image.Image, error)
	workBytes  int64
	fixedBytes int64
}

// formats are the formats Open decodes (a PNG is "apng" where
// github.com/kettek/apng is linked, as lib/render links it; TGA, which
// has no magic, has no name). A HEIF picture decodes turned as its
// container says, which its registration leaves as stored; a JPEG XL
// one decodes at any level, where its registration refuses level 10,
// which libjxl writes for a lossless picture with more than 8 bits or
// alpha.
var formats = []format{
	{names: []string{"png", "apng"}, mimeType: "image/png", exts: []string{"png"}},
	{names: []string{"jpeg"}, mimeType: typeJPEG, exts: []string{"jpg", "jpeg"}},
	{names: []string{"gif"}, mimeType: "image/gif", exts: []string{"gif"}},
	{names: []string{"webp"}, mimeType: "image/webp", exts: []string{"webp"}},
	{names: []string{"bmp"}, mimeType: "image/bmp", exts: []string{"bmp"}},
	{names: []string{"tiff"}, mimeType: "image/tiff", exts: []string{"tiff", "tif"}},
	{mimeType: typeTGA, exts: []string{"tga"}},
	{names: []string{"pnm"}, mimeType: "image/x-portable-anymap", exts: []string{"pnm", "pbm", "pgm", "ppm"}},
	{names: []string{"farbfeld"}, mimeType: "image/x-farbfeld", exts: []string{"ff", "farbfeld"}},
	{
		names: []string{"heic"}, mimeType: "image/heif", exts: []string{"heic", "heif", "hif"}, workBytes: 10, fixedBytes: heicFixedBytes,
		decode: func(r io.Reader) (image.Image, error) { return heic.Decode(r, heic.Options{AutoRotate: true}) },
	},
	{
		names: []string{"jxl"}, mimeType: "image/jxl", exts: []string{"jxl"}, workBytes: 38, fixedBytes: jxlThreads * jxlThreadBytes,
		decode: func(r io.Reader) (image.Image, error) {
			return jxl.Decode(r, jxl.Options{Level: 10, Threads: jxlThreads})
		},
	},
}

// Types are the MIME types of the formats Open decodes.
func Types() []string {
	out := make([]string, len(formats))
	for i, f := range formats {
		out[i] = f.mimeType
	}
	return out
}

// Extensions are the file extensions of the formats Open decodes,
// lower case and without their dot.
func Extensions() []string {
	var out []string
	for _, f := range formats {
		out = append(out, f.exts...)
	}
	return out
}

// formatNamed is the format image.DecodeConfig names name.
func formatNamed(name string) (format, error) {
	for _, f := range formats {
		if slices.Contains(f.names, name) {
			return f, nil
		}
	}
	return format{}, fmt.Errorf("imagedecode: the format %q is not one lys decodes", name)
}

// decodeImage decodes the image rs holds from its start, through its
// format's own decoder where it has one.
func decodeImage(rs io.ReadSeeker) (image.Image, error) {
	_, name, err := image.DecodeConfig(bufio.NewReader(rs))
	if err != nil {
		return nil, err
	}
	f, err := formatNamed(name)
	if err != nil {
		return nil, err
	}
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	if f.decode != nil {
		return f.decode(bufio.NewReader(rs))
	}
	img, _, err := image.Decode(bufio.NewReader(rs))
	return img, err
}

// Decode decodes image bytes, sniffed as Open sniffs a file.
func Decode(data []byte) (image.Image, error) {
	img, err := decodeImage(bytes.NewReader(data))
	if err == nil {
		return img, nil
	}
	if tga, tgaErr := decodeTGA(bytes.NewReader(data)); tgaErr == nil {
		return tga, nil
	}
	return nil, err
}

// Open decodes the image file at path. The format is sniffed from the
// content, except TGA, which has no magic and is tried last.
func Open(path string) (image.Image, error) {
	return OpenContext(context.Background(), path)
}

// OpenContext decodes the image file at path as Open does, and gives
// up with ctx's error at the decoder's next read once ctx is done.
func OpenContext(ctx context.Context, path string) (image.Image, error) {
	f, err := os.Open(path) //nolint:gosec // the caller's image path
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return decodeFile(ctx, f, path)
}

// decodeFile decodes the image file f, opened from path and read from
// its start, as OpenContext does.
func decodeFile(ctx context.Context, f *os.File, path string) (image.Image, error) {
	r := ctxReader{ctx, f}
	img, err := decodeImage(r)
	if err == nil {
		return img, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, ctxErr)
	}
	if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
		return nil, err
	}
	if tga, tgaErr := decodeTGA(bufio.NewReader(r)); tgaErr == nil {
		return tga, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, ctxErr)
	}
	return nil, fmt.Errorf("%s: %w", path, err)
}

// ctxReader reads r until ctx is done, then fails with ctx's error.
type ctxReader struct {
	ctx context.Context
	r   io.ReadSeeker
}

func (c ctxReader) Seek(offset int64, whence int) (int64, error) { return c.r.Seek(offset, whence) }

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
