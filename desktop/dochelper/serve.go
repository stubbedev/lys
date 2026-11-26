package dochelper

import (
	"bufio"
	"errors"
	"fmt"
	"image"
	"io"
)

// Handler is what a serving helper reads one document with: Open opens
// it and says what it holds, before any of the others; Render draws a
// tile CheckTile allowed (a tile drawn in part is returned with the
// error saying what was left out); Text and Links read a page of it.
type Handler interface {
	Open(path string) (Info, error)
	Render(t Tile) (*image.RGBA, error)
	Text(page int) (Text, error)
	Links(page int) ([]Link, error)
	Close() error
}

// Serve answers the requests read from r on w with h until a close
// request or the end of r, closing what h opened. A request h fails is
// answered with its failure and serving goes on; a frame the protocol
// does not allow, or a reply that cannot be written, ends it with the
// error.
func Serve(r io.Reader, w io.Writer, h Handler) (err error) {
	s := server{h: h, w: bufio.NewWriter(w)}
	defer func() {
		if s.info.Pages() > 0 {
			err = errors.Join(err, h.Close())
		}
	}()
	br := bufio.NewReader(r)
	for {
		k, body, err := readFrame(br)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := s.answer(k, body); err != nil {
			return err
		}
		if k == kindClose {
			return nil
		}
	}
}

type server struct {
	h    Handler
	w    *bufio.Writer
	info Info
}

// answer answers one request of kind k.
func (s *server) answer(k kind, body []byte) error {
	if k == kindFailed {
		return fmt.Errorf("%w: a %v request", errProtocol, k)
	}
	opened := s.info.Pages() > 0
	switch {
	case k == kindOpen && opened:
		return s.fail(fmt.Errorf("%s: an open request with a document open", Program))
	case k != kindOpen && k != kindClose && !opened:
		return s.fail(fmt.Errorf("%s: a %v request with no document open", Program, k))
	}
	switch k {
	case kindOpen:
		var req openRequest
		if err := readJSON(k, body, &req); err != nil {
			return err
		}
		info, err := s.h.Open(req.Path)
		if err == nil {
			err = info.validate()
		}
		if err != nil {
			return s.fail(err)
		}
		s.info = info
		return writeJSON(s.w, k, info)
	case kindRender:
		var t Tile
		if err := readJSON(k, body, &t); err != nil {
			return err
		}
		return s.render(t)
	case kindText:
		page, err := s.page(k, body)
		if err != nil || page < 0 {
			return err
		}
		text, err := s.h.Text(page)
		if err != nil {
			return s.fail(err)
		}
		return writeJSON(s.w, k, text)
	case kindLinks:
		page, err := s.page(k, body)
		if err != nil || page < 0 {
			return err
		}
		links, err := s.h.Links(page)
		if err != nil {
			return s.fail(err)
		}
		return writeJSON(s.w, k, links)
	case kindClose:
		if len(body) != 0 {
			return fmt.Errorf("%w: a close request of %d bytes", errProtocol, len(body))
		}
		return writeFrame(s.w, k)
	}
	return fmt.Errorf("%w: a %v request", errProtocol, k)
}

// page is the page a request of kind k asks about, or -1 having
// answered a page the document does not have.
func (s *server) page(k kind, body []byte) (int, error) {
	var req pageRequest
	if err := readJSON(k, body, &req); err != nil {
		return -1, err
	}
	if err := s.info.checkPage(req.Page); err != nil {
		return -1, s.fail(err)
	}
	return req.Page, nil
}

// render answers a render request for t.
func (s *server) render(t Tile) error {
	if err := s.info.CheckTile(t); err != nil {
		return s.fail(err)
	}
	img, left := s.h.Render(t)
	if img == nil {
		if left == nil {
			left = fmt.Errorf("%s: page %d drew nothing", Program, t.Page+1)
		}
		return s.fail(left)
	}
	if img.Rect.Size() != t.Rect.Size() {
		return s.fail(fmt.Errorf("%s: tile %v of page %d drew %v", Program, t.Rect, t.Page+1, img.Rect.Size()))
	}
	note := ""
	if left != nil {
		note = left.Error()
	}
	return writeFrame(s.w, kindRender, tileParts(img, note)...)
}

// fail answers a request with err.
func (s *server) fail(err error) error { return writeFrame(s.w, kindFailed, failureBody(err)) }
