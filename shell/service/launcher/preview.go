package launcher

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"unicode/utf8"

	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/desktop/thumbnail"
)

// ErrNoPreview is a row with nothing to load: facts alone, text that is
// not text.
var ErrNoPreview = errors.New("launcher: nothing to preview")

// Preview is what a row shows in the side pane. The zero Preview is
// none.
type Preview struct {
	// Path is a file, which the pane shows through package preview as
	// Quick Look does; LoadPreview loads the rest.
	Path string
	// Text is text to show as it is.
	Text string
	// Load produces an encoded image, or text with LoadsText, off the UI loop
	// and only when the row is selected.
	Load      func() ([]byte, error)
	LoadsText bool
	// Image produces a decoded image off the UI loop, only when the row is
	// selected: a window's contents, captured from the compositor.
	Image func(context.Context) (image.Image, error)
	// Facts describe the row beside whatever loads: a window's title, app
	// and workspace, a clipboard entry's type and size.
	Facts []PreviewFact
}

// PreviewFact is one labelled line of a preview.
type PreviewFact struct{ Label, Value string }

// Any reports whether there is anything to preview.
func (p Preview) Any() bool {
	return p.Path != "" || p.Text != "" || p.Load != nil || p.Image != nil || len(p.Facts) > 0
}

// PreviewContent is a loaded preview: an image or text.
type PreviewContent struct {
	Image image.Image
	Text  string
}

// Previews is the thumbnailer a file's preview shows a document's
// thumbnail through: -preview-cmd's when it is set, nil when it does
// not parse.
func (t Thumbnailer) Previews() *thumbnail.Thumbnailer { return t.files }

// LoadPreview loads a preview that is not a file's, off the UI loop.
// Facts alone have nothing to load: ErrNoPreview.
func LoadPreview(ctx context.Context, p Preview) (PreviewContent, error) {
	switch {
	case p.Image != nil:
		img, err := p.Image(ctx)
		if err != nil {
			return PreviewContent{}, err
		}
		return PreviewContent{Image: img}, nil
	case p.Load != nil:
		data, err := p.Load()
		if err != nil {
			return PreviewContent{}, err
		}
		if p.LoadsText {
			text, ok := textOf(data)
			if !ok {
				return PreviewContent{}, ErrNoPreview
			}
			return PreviewContent{Text: text}, nil
		}
		img, err := imagedecode.Decode(data)
		if err != nil {
			return PreviewContent{}, fmt.Errorf("preview: decode: %w", err)
		}
		return PreviewContent{Image: img}, nil
	case p.Text != "":
		return PreviewContent{Text: p.Text}, nil
	}
	return PreviewContent{}, ErrNoPreview
}

// textOf is data as text when it reads as text: no NUL, valid UTF-8.
func textOf(data []byte) (string, bool) {
	if len(data) == 0 || bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return "", false
	}
	return string(data), true
}
