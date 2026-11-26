package demux

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	id3HeaderSize   = 10
	id3Unsync       = 0x80
	id3Extended     = 0x40
	id3Footer       = 0x10
	id3v24Unsync    = 0x0002
	id3v24Length    = 0x0001
	id3v24Hidden    = 0x000c
	id3v23Hidden    = 0x00c0
	id3v23Group     = 0x0020
	id3TextUTF16BOM = 1
	id3TextUTF16BE  = 2
)

// id3Size is the length of the ID3v2 tag at b's start, its header and
// footer included; ok is false where none starts.
func id3Size(b []byte) (int64, bool) {
	if len(b) < id3HeaderSize || string(b[:3]) != "ID3" || b[3] == 0xff || b[4] == 0xff || b[6]|b[7]|b[8]|b[9] >= 0x80 {
		return 0, false
	}
	n := int64(id3HeaderSize) + int64(syncsafe(b[6:10]))
	if b[5]&id3Footer != 0 {
		n += id3HeaderSize
	}
	return n, true
}

func syncsafe(b []byte) uint32 {
	var v uint32
	for _, c := range b {
		v = v<<7 | uint32(c&0x7f)
	}
	return v
}

// undoUnsync takes out the zero bytes unsynchronisation puts after
// each 0xff.
func undoUnsync(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte{0xff, 0}, []byte{0xff})
}

// offerID3 takes the pictures of an ID3v2 tag, tag holding it whole:
// APIC frames of versions 2.3 and 2.4, PIC frames of 2.2.
func (c *cover) offerID3(tag []byte) error {
	malformed := func(why string) error {
		return fmt.Errorf("ID3v2 tag of %d bytes: %s: %w", len(tag), why, ErrMalformed)
	}
	version, flags := tag[3], tag[5]
	end := id3HeaderSize + int(syncsafe(tag[6:10]))
	if end > len(tag) {
		return malformed("size past its bytes")
	}
	body := tag[id3HeaderSize:end]
	if flags&id3Unsync != 0 && version < 4 {
		body = undoUnsync(body)
	}
	if flags&id3Extended != 0 && version >= 3 {
		if len(body) < 4 {
			return malformed("extended header")
		}
		n := int(binary.BigEndian.Uint32(body)) + 4
		if version == 4 {
			n = int(syncsafe(body[:4]))
		}
		if n > len(body) || n < 4 {
			return malformed("extended header size")
		}
		body = body[n:]
	}
	idSize, headSize := 4, 10
	if version == 2 {
		idSize, headSize = 3, 6
	}
	for len(body) >= headSize && body[0] != 0 {
		id := string(body[:idSize])
		var size int
		var frameFlags uint16
		switch version {
		case 2:
			size = int(body[3])<<16 | int(body[4])<<8 | int(body[5])
		case 3:
			size = int(binary.BigEndian.Uint32(body[4:]))
			frameFlags = binary.BigEndian.Uint16(body[8:])
		default:
			size = int(syncsafe(body[4:8]))
			frameFlags = binary.BigEndian.Uint16(body[8:])
		}
		if size < 0 || size > len(body)-headSize {
			return malformed(fmt.Sprintf("frame %q of %d bytes", id, size))
		}
		data := body[headSize : headSize+size]
		body = body[headSize+size:]
		if id != "APIC" && id != "PIC" {
			continue
		}
		data, ok := id3FrameData(version, frameFlags, data)
		if !ok {
			continue
		}
		if err := c.offerID3Picture(version, data); err != nil {
			return fmt.Errorf("ID3v2 %s frame: %w", id, err)
		}
	}
	return nil
}

// id3FrameData is a frame's data as its flags leave it; ok is false for
// a compressed or encrypted frame.
func id3FrameData(version byte, flags uint16, data []byte) ([]byte, bool) {
	switch version {
	case 3:
		if flags&id3v23Hidden != 0 {
			return nil, false
		}
		if flags&id3v23Group != 0 && len(data) > 0 {
			data = data[1:]
		}
	case 4:
		if flags&id3v24Hidden != 0 {
			return nil, false
		}
		if flags&id3v24Length != 0 {
			if len(data) < 4 {
				return nil, false
			}
			data = data[4:]
		}
		if flags&id3v24Unsync != 0 {
			data = undoUnsync(data)
		}
	}
	return data, true
}

// offerID3Picture takes the image of an APIC or PIC frame's data.
func (c *cover) offerID3Picture(version byte, b []byte) error {
	if len(b) < 2 {
		return fmt.Errorf("%d bytes: %w", len(b), ErrMalformed)
	}
	encoding := b[0]
	b = b[1:]
	var mime string
	if version == 2 {
		if len(b) < 3 {
			return fmt.Errorf("no image format: %w", ErrMalformed)
		}
		mime, b = string(b[:3]), b[3:]
	} else {
		i := bytes.IndexByte(b, 0)
		if i < 0 {
			return fmt.Errorf("unterminated MIME type: %w", ErrMalformed)
		}
		mime, b = string(b[:i]), b[i+1:]
	}
	if len(b) < 1 {
		return fmt.Errorf("no picture type: %w", ErrMalformed)
	}
	kind := b[0]
	b = b[1:]
	end := -1
	switch encoding {
	case id3TextUTF16BOM, id3TextUTF16BE:
		for i := 0; i+1 < len(b); i += 2 {
			if b[i] == 0 && b[i+1] == 0 {
				end = i + 2
				break
			}
		}
	default:
		if i := bytes.IndexByte(b, 0); i >= 0 {
			end = i + 1
		}
	}
	if end < 0 {
		return fmt.Errorf("unterminated description: %w", ErrMalformed)
	}
	if mime != pictureLink {
		c.offer(b[end:], kind == pictureFrontCover)
	}
	return nil
}
