package screenshot

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/ocr"
	"github.com/stubbedev/lys/lib/portaldialogs"
)

// ReadKind is what Read looks for in a region of the screen.
type ReadKind string

// What Read can read.
const (
	ReadText ReadKind = "text"
	ReadQR   ReadKind = "qr"
)

// ParseReadKind is the kind a CLI or D-Bus argument names.
func ParseReadKind(s string) (ReadKind, error) {
	switch k := ReadKind(s); k {
	case ReadText, ReadQR:
		return k, nil
	}
	return "", fmt.Errorf("unknown screen reading %q: want %s or %s", s, ReadText, ReadQR)
}

const ocrTimeout = 30 * time.Second

var (
	// ErrNoCode answers a QR reading of a region that holds no code.
	ErrNoCode = errors.New("no code found")

	errOCRTimeout = fmt.Errorf("reading the text did not finish within %s", ocrTimeout)
)

// Read lets the user drag a region and reads it: its text, or the QR
// code in it. Nothing is saved. Text lands on the clipboard and in a
// toast; a code is announced with buttons to copy it or, for a URL, open
// it. "" is a cancelled selection. A region with no text in it fails
// with ocr.ErrNoText, and Dismiss with the caller's handle stops a
// running text reading.
func (h *Host) Read(c portaldialogs.Caller, kind ReadKind) (string, error) {
	img, ok, err := h.captureRegion(c, false)
	if err != nil || !ok {
		return "", err
	}
	switch kind {
	case ReadText:
		return h.readText(c, img)
	case ReadQR:
		return h.readCode(img)
	}
	return "", fmt.Errorf("unknown screen reading %q", kind)
}

func (h *Host) readText(c portaldialogs.Caller, img *image.RGBA) (string, error) {
	ctx, done := h.dismissable(c.Handle)
	defer done()
	ctx, cancel := context.WithTimeoutCause(ctx, ocrTimeout, errOCRTimeout)
	defer cancel()
	if h.Working != nil {
		finished := h.Working(done)
		defer finished()
	}
	text, err := h.recognize(ctx, img)
	if err != nil {
		return "", err
	}
	if err := h.CopyText(text); err != nil {
		return "", fmt.Errorf("cannot copy the text: %w", err)
	}
	first, _, _ := strings.Cut(text, "\n")
	h.Toast(first, "ld-scan-text-symbolic")
	return text, nil
}

func (h *Host) recognize(ctx context.Context, img *image.RGBA) (string, error) {
	faces, err := h.Faces()
	if err != nil {
		return "", fmt.Errorf("cannot load the fonts to read the text in: %w", err)
	}
	r, err := ocr.New(faces...)
	if err != nil {
		return "", err
	}
	return r.Read(ctx, img)
}

func (h *Host) readCode(img *image.RGBA) (string, error) {
	payload, err := DecodeQR(img)
	if err != nil {
		return "", err
	}
	actions := []Action{ActionCopy}
	if IsURL(payload) {
		actions = []Action{ActionOpen, ActionCopy}
	}
	h.announce(Notice{
		Summary: i18n.T("screenshot-qr-code"),
		Body:    payload,
		Icon:    "ld-scan-qr-code-symbolic",
		Actions: actions,
		Respond: func(a Action) {
			switch a {
			case ActionCopy:
				if err := h.CopyText(payload); err != nil {
					log.Printf("screenshot: copy the QR code: %v", err)
					h.Toast(i18n.T("screenshot-qr-copy-failed"), "ld-scan-qr-code-symbolic")
				}
			case ActionOpen:
				h.OpenURL(payload)
			}
		},
	})
	return payload, nil
}

// IsURL reports whether a decoded payload is a link to open.
func IsURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}

// DecodeQR is the payload of the QR code in img, ErrNoCode when it holds none.
func DecodeQR(img *image.RGBA) (string, error) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	src, err := gozxing.NewPlanarYUVLuminanceSource(luminance(img), w, h, 0, 0, w, h, false)
	if err != nil {
		return "", fmt.Errorf("cannot read the region for a QR code: %w", err)
	}
	bmp, err := gozxing.NewBinaryBitmap(gozxing.NewHybridBinarizer(src))
	if err != nil {
		return "", fmt.Errorf("cannot read the region for a QR code: %w", err)
	}
	res, err := qrcode.NewQRCodeReader().Decode(bmp, map[gozxing.DecodeHintType]any{gozxing.DecodeHintType_TRY_HARDER: true})
	if _, missing := errors.AsType[gozxing.NotFoundException](err); missing {
		return "", ErrNoCode
	}
	if err != nil {
		return "", fmt.Errorf("cannot decode the QR code: %w", err)
	}
	return res.GetText(), nil
}

func luminance(img *image.RGBA) []byte {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	out := make([]byte, 0, w*h)
	for y := range h {
		at := img.PixOffset(img.Rect.Min.X, img.Rect.Min.Y+y)
		for px := range slices.Chunk(img.Pix[at:at+w*4], 4) {
			r, g, b, a := uint32(px[0])*0x101, uint32(px[1])*0x101, uint32(px[2])*0x101, uint32(px[3])*0x101
			lum := (r + 2*g + b) * 255 / (4 * 0xffff)
			out = append(out, byte((lum*a+(0xffff-a)*255)/0xffff)) //nolint:gosec // a blend of two values at most 255
		}
	}
	return out
}
