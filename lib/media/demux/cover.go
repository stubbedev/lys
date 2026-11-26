package demux

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
)

const (
	pictureFrontCover = 3
	// pictureLink is the MIME type of a picture that is a URL.
	pictureLink = "-->"
	// commentPicture is the Vorbis comment holding a FLAC PICTURE
	// block in base64.
	commentPicture = "METADATA_BLOCK_PICTURE="
)

// cover is the picture a file embeds as its cover: the front cover
// where the file marks one, else the first picture it holds.
type cover struct {
	data  []byte
	front bool
}

// wants reports whether a picture, the front cover or not, would
// replace the one taken so far.
func (c *cover) wants(front bool) bool { return c.data == nil || front && !c.front }

func (c *cover) offer(data []byte, front bool) {
	if len(data) == 0 || !c.wants(front) {
		return
	}
	c.data, c.front = bytes.Clone(data), front
}

// offerFLAC takes the picture of a FLAC PICTURE block's body.
func (c *cover) offerFLAC(b []byte) error {
	f := fields{b: b}
	kind := f.u32()
	mime := f.take(int(f.u32()))
	f.skip(int(f.u32()))
	f.skip(16)
	data := f.take(int(f.u32()))
	if f.short {
		return fmt.Errorf("PICTURE of %d bytes: %w", len(b), ErrMalformed)
	}
	if string(mime) != pictureLink {
		c.offer(data, kind == pictureFrontCover)
	}
	return nil
}

// offerComments takes the pictures of a Vorbis comment block: the
// vendor string and the comments after it, without a framing bit.
func (c *cover) offerComments(b []byte) error {
	malformed := func(what string) error {
		return fmt.Errorf("the Vorbis comments of %d bytes: %s: %w", len(b), what, ErrMalformed)
	}
	if len(b) < 4 {
		return malformed("no vendor length")
	}
	vendor := int64(binary.LittleEndian.Uint32(b))
	if vendor > int64(len(b)-4) {
		return malformed("vendor string")
	}
	b = b[4+vendor:]
	if len(b) < 4 {
		return malformed("no comment count")
	}
	count := binary.LittleEndian.Uint32(b)
	b = b[4:]
	for range count {
		if len(b) < 4 {
			return malformed("comment length")
		}
		n := int64(binary.LittleEndian.Uint32(b))
		if n > int64(len(b)-4) {
			return malformed("comment")
		}
		comment := b[4 : 4+n]
		b = b[4+n:]
		if len(comment) < len(commentPicture) || !strings.EqualFold(string(comment[:len(commentPicture)]), commentPicture) {
			continue
		}
		block, err := base64.StdEncoding.AppendDecode(nil, comment[len(commentPicture):])
		if err != nil {
			return fmt.Errorf("%s: %w: %w", commentPicture, ErrMalformed, err)
		}
		if err := c.offerFLAC(block); err != nil {
			return err
		}
	}
	return nil
}
