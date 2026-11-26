package dochelper

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"strconv"
	"unicode/utf8"
)

// The protocol a serving helper speaks on its standard input and
// output: the host sends a request frame and reads one reply frame
// before it sends the next. A frame is its length (a big-endian uint32
// counting what follows), a kind byte and the kind's body. A request is
// answered by a frame of its own kind or by kindFailed. Bodies are JSON
// but for a rendered tile's (its width, height and note length as
// big-endian uint32s, the note, then its RGBA rows) and a failure's
// (its exit status, then its message).
type kind uint8

const (
	kindOpen kind = iota + 1
	kindRender
	kindText
	kindLinks
	kindClose
	kindFailed
)

func (k kind) String() string {
	switch k {
	case kindOpen:
		return "open"
	case kindRender:
		return "render"
	case kindText:
		return "text"
	case kindLinks:
		return "links"
	case kindClose:
		return "close"
	case kindFailed:
		return "failed"
	}
	return "kind(" + strconv.Itoa(int(k)) + ")"
}

// The size of a frame's length and kind, of a tile's header, the
// longest note a tile carries, and the longest frame either side
// reads.
const (
	frameHeader = 5
	tileHeader  = 12
	maxNote     = 4 << 10
	maxFrame    = 1 + tileHeader + maxNote + 4*MaxSize*MaxSize
)

// The limits a document's description is held to: its pages, a page's
// side in points, how deep and how long its outline runs, and the
// largest scale a page renders at, in device pixels per point.
const (
	MaxPages        = 1 << 20
	maxPagePoints   = 1 << 20
	maxOutlineDepth = 64
	maxOutlineItems = 1 << 16
	MaxScale        = 64
)

// errProtocol is a frame or a body the protocol does not allow.
var errProtocol = errors.New(Program + ": a malformed message")

// Size is a page's size in points, at 72 to the inch.
type Size struct{ W, H float64 }

// At is the page's size in whole device pixels at scale device pixels
// per point.
func (s Size) At(scale float64) image.Point {
	return image.Pt(int(math.Ceil(s.W*scale)), int(math.Ceil(s.H*scale)))
}

// Box is an area of a page in points, from its top left corner, x
// rightwards and y downwards.
type Box struct{ X0, Y0, X1, Y1 float64 }

// Contains reports whether the point x, y lies in b.
func (b Box) Contains(x, y float64) bool { return x >= b.X0 && x < b.X1 && y >= b.Y0 && y < b.Y1 }

func (b Box) finite() bool {
	for _, v := range []float64{b.X0, b.Y0, b.X1, b.Y1} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return b.X0 <= b.X1 && b.Y0 <= b.Y1
}

// Outline is one entry of a document's table of contents: its title,
// the page it leads to (-1 for none) or the URI it points out of the
// document at, and the entries under it.
type Outline struct {
	Title    string
	Page     int
	URI      string
	Children []Outline
}

// Info is what an open document says of itself: the size of each of
// its pages, their labels (none, or one for each page; an empty label
// is the page's number) and its outline.
type Info struct {
	Sizes   []Size
	Labels  []string
	Outline []Outline
}

// Pages is how many pages the document has.
func (i Info) Pages() int { return len(i.Sizes) }

// Label is what page (from zero) is called: its label, else its number
// from one.
func (i Info) Label(page int) string {
	if page < len(i.Labels) && i.Labels[page] != "" {
		return i.Labels[page]
	}
	return strconv.Itoa(page + 1)
}

// Tile is part of a page drawn at a scale: Rect, in the device pixels
// of the whole page at Scale device pixels per point.
type Tile struct {
	Page  int
	Scale float64
	Rect  image.Rectangle
}

// CheckTile reports a tile the document cannot draw: of a page it does
// not have, at a scale out of range, outside its page or larger than
// MaxSize along a side.
func (i Info) CheckTile(t Tile) error {
	if err := i.checkPage(t.Page); err != nil {
		return err
	}
	if !(t.Scale > 0 && t.Scale <= MaxScale) {
		return fmt.Errorf("%s: a scale of %g is not within 0 to %d", Program, t.Scale, MaxScale)
	}
	page := image.Rectangle{Max: i.Sizes[t.Page].At(t.Scale)}
	if t.Rect.Empty() || !t.Rect.In(page) {
		return fmt.Errorf("%s: tile %v is not within page %d's %v", Program, t.Rect, t.Page+1, page)
	}
	if t.Rect.Dx() > MaxSize || t.Rect.Dy() > MaxSize {
		return fmt.Errorf("%w: %s: tile %v is over %d pixels a side", ErrTooLarge, Program, t.Rect, MaxSize)
	}
	return nil
}

func (i Info) checkPage(page int) error {
	if page < 0 || page >= i.Pages() {
		return fmt.Errorf("%s: page %d is not one of the document's %d", Program, page+1, i.Pages())
	}
	return nil
}

func (i Info) validate() error {
	n := i.Pages()
	if n == 0 || n > MaxPages {
		return fmt.Errorf("%w: %d pages", errProtocol, n)
	}
	for p, s := range i.Sizes {
		if !(s.W > 0 && s.W <= maxPagePoints && s.H > 0 && s.H <= maxPagePoints) {
			return fmt.Errorf("%w: page %d is %gx%g points", errProtocol, p+1, s.W, s.H)
		}
	}
	if len(i.Labels) != 0 && len(i.Labels) != n {
		return fmt.Errorf("%w: %d labels for %d pages", errProtocol, len(i.Labels), n)
	}
	items := 0
	return validateOutline(i.Outline, n, 1, &items)
}

func validateOutline(entries []Outline, pages, depth int, items *int) error {
	if len(entries) > 0 && depth > maxOutlineDepth {
		return fmt.Errorf("%w: an outline over %d deep", errProtocol, maxOutlineDepth)
	}
	for _, e := range entries {
		if *items++; *items > maxOutlineItems {
			return fmt.Errorf("%w: an outline of over %d entries", errProtocol, maxOutlineItems)
		}
		if e.Page < -1 || e.Page >= pages {
			return fmt.Errorf("%w: outline entry %q leads to page %d of %d", errProtocol, e.Title, e.Page+1, pages)
		}
		if err := validateOutline(e.Children, pages, depth+1, items); err != nil {
			return err
		}
	}
	return nil
}

// Char is one character of a page and the box it covers.
type Char struct {
	Rune rune
	Box  Box
}

// Line is a run of characters along one baseline, in reading order.
type Line struct{ Chars []Char }

// Text is what a page says, line by line in reading order.
type Text struct{ Lines []Line }

func (t Text) validate() error {
	for _, l := range t.Lines {
		for _, c := range l.Chars {
			if !utf8.ValidRune(c.Rune) || !c.Box.finite() {
				return fmt.Errorf("%w: character %q at %+v", errProtocol, c.Rune, c.Box)
			}
		}
	}
	return nil
}

// Link is an area of a page that leads somewhere: to Page (from zero)
// of the document, -1 for one leading out of it to URI.
type Link struct {
	Box  Box
	Page int
	URI  string
}

func validateLinks(links []Link, pages int) error {
	for _, l := range links {
		if !l.Box.finite() || l.Page < -1 || l.Page >= pages || (l.Page == -1) == (l.URI == "") {
			return fmt.Errorf("%w: link %+v of a %d page document", errProtocol, l, pages)
		}
	}
	return nil
}

type openRequest struct{ Path string }

type pageRequest struct{ Page int }

// writeFrame writes one frame of kind k whose body is parts, and
// flushes it.
func writeFrame(w *bufio.Writer, k kind, parts ...[]byte) error {
	n := 1
	for _, p := range parts {
		n += len(p)
	}
	if n > maxFrame {
		return fmt.Errorf("%w: a %v frame of %d bytes", errProtocol, k, n)
	}
	var head [frameHeader]byte
	binary.BigEndian.PutUint32(head[:4], uint32(n))
	head[4] = byte(k)
	if _, err := w.Write(head[:]); err != nil {
		return fmt.Errorf("%s: writing a %v frame: %w", Program, k, err)
	}
	for _, p := range parts {
		if _, err := w.Write(p); err != nil {
			return fmt.Errorf("%s: writing a %v frame: %w", Program, k, err)
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("%s: writing a %v frame: %w", Program, k, err)
	}
	return nil
}

// writeJSON writes one frame of kind k whose body is v as JSON.
func writeJSON(w *bufio.Writer, k kind, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("%s: encoding a %v frame: %w", Program, k, err)
	}
	return writeFrame(w, k, body)
}

// readFrame reads one frame: io.EOF when the stream ends before it
// starts, errProtocol for one cut short, of an unknown kind or over
// maxFrame.
func readFrame(r *bufio.Reader) (kind, []byte, error) {
	var head [frameHeader]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		if errors.Is(err, io.EOF) {
			return 0, nil, io.EOF
		}
		return 0, nil, fmt.Errorf("%w: a frame cut short: %w", errProtocol, err)
	}
	n := binary.BigEndian.Uint32(head[:4])
	k := kind(head[4])
	if n < 1 || n > maxFrame {
		return 0, nil, fmt.Errorf("%w: a %v frame of %d bytes", errProtocol, k, n)
	}
	if k < kindOpen || k > kindFailed {
		return 0, nil, fmt.Errorf("%w: a frame of %v", errProtocol, k)
	}
	body := make([]byte, n-1)
	if _, err := io.ReadFull(r, body); err != nil {
		return 0, nil, fmt.Errorf("%w: a %v frame cut short: %w", errProtocol, k, err)
	}
	return k, body, nil
}

// readJSON decodes body, of kind k, into v, refusing what v has no
// field for and anything after it.
func readJSON(k kind, body []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: a %v body: %w", errProtocol, k, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: a %v body with more after it", errProtocol, k)
	}
	return nil
}

// tileParts is the body of the reply drawing img, with the note saying
// what it left out (cut to maxNote).
func tileParts(img *image.RGBA, note string) [][]byte {
	if len(note) > maxNote {
		note = note[:maxNote]
	}
	w, h := img.Rect.Dx(), img.Rect.Dy()
	head := make([]byte, tileHeader, tileHeader+len(note))
	binary.BigEndian.PutUint32(head[0:], uint32(w))         //nolint:gosec // within MaxSize
	binary.BigEndian.PutUint32(head[4:], uint32(h))         //nolint:gosec // within MaxSize
	binary.BigEndian.PutUint32(head[8:], uint32(len(note))) //nolint:gosec // within maxNote
	parts := [][]byte{append(head, note...)}
	if img.Stride == 4*w {
		return append(parts, img.Pix[:4*w*h])
	}
	for y := range h {
		parts = append(parts, img.Pix[y*img.Stride:y*img.Stride+4*w])
	}
	return parts
}

// readTile is the tile body describes, which must be want pixels; its
// pixels are body's own. A note is what the helper left out of it.
func readTile(body []byte, want image.Point) (*image.RGBA, string, error) {
	if len(body) < tileHeader {
		return nil, "", fmt.Errorf("%w: a tile of %d bytes", errProtocol, len(body))
	}
	w, h := binary.BigEndian.Uint32(body[0:]), binary.BigEndian.Uint32(body[4:])
	notes := binary.BigEndian.Uint32(body[8:])
	if int64(w) != int64(want.X) || int64(h) != int64(want.Y) {
		return nil, "", fmt.Errorf("%w: a %dx%d tile, asked for %v", errProtocol, w, h, want)
	}
	if notes > maxNote || int64(len(body)) != tileHeader+int64(notes)+4*int64(w)*int64(h) {
		return nil, "", fmt.Errorf("%w: a %dx%d tile with a %d byte note in %d bytes", errProtocol, w, h, notes, len(body))
	}
	pix := body[tileHeader+notes:]
	return &image.RGBA{Pix: pix, Stride: 4 * want.X, Rect: image.Rectangle{Max: want}}, string(body[tileHeader : tileHeader+notes]), nil
}

// failureBody is the body of a reply failing with err: the status the
// helper would exit with after it, then what it says.
func failureBody(err error) []byte {
	msg := err.Error()
	if len(msg) > maxNote {
		msg = msg[:maxNote]
	}
	return append([]byte{byte(ExitCode(err))}, msg...) //nolint:gosec // exit statuses are below 7
}

// readFailure is the error a failure's body tells: one of the errors
// its status tells, saying what the helper said.
func readFailure(body []byte) error {
	if len(body) == 0 {
		return fmt.Errorf("%w: an empty failure", errProtocol)
	}
	code, msg := int(body[0]), string(body[1:])
	if told := ExitError(code, nil); told != nil {
		return fmt.Errorf("%w: %s", told, msg)
	}
	if code != exitFailed {
		return fmt.Errorf("%w: a failure of status %d: %s", errProtocol, code, msg)
	}
	return fmt.Errorf("%s: %s", Program, msg)
}
