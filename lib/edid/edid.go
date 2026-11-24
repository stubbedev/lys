// Package edid reads what a display's EDID says about it: its make, model
// and serial, as outputs are described and matched by, its colour and its
// variable refresh range.
package edid

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

const (
	blockSize       = 128
	descriptorStart = 54
	descriptorSize  = 18
	descriptorCount = 4
	tagSerial       = 0xff
	tagName         = 0xfc
)

// Info is a display's identity: its PNP manufacturer ID, its make (the
// manufacturer's name, or the PNP ID when unknown), its model and its
// serial.
type Info struct {
	PNP    string
	Make   string
	Model  string
	Serial string
}

var header = []byte{0, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0}

// ErrInvalid is a blob that is not an EDID: too short, without the
// header, or with a bad base block checksum.
var ErrInvalid = errors.New("edid: invalid")

// Parse reads the identity from EDID b's base block. The model and
// serial come from the name and serial descriptors, falling back to
// the product code and the numeric serial in hex; a zero numeric
// serial leaves Serial empty. It fails with ErrInvalid.
func Parse(b []byte) (Info, error) {
	if err := checkBase(b); err != nil {
		return Info{}, err
	}
	id := binary.BigEndian.Uint16(b[8:])
	pnp := string([]byte{byte(id>>10&0x1f) + 'A' - 1, byte(id>>5&0x1f) + 'A' - 1, byte(id&0x1f) + 'A' - 1})
	info := Info{PNP: pnp, Make: pnp}
	if name, ok := Vendor(pnp); ok {
		info.Make = name
	}
	eachDisplayDescriptor(b, func(tag byte, d []byte) {
		switch tag {
		case tagName:
			info.Model = descriptorText(d[5:])
		case tagSerial:
			info.Serial = descriptorText(d[5:])
		}
	})
	if info.Model == "" {
		info.Model = fmt.Sprintf("0x%04X", binary.LittleEndian.Uint16(b[10:]))
	}
	if serial := binary.LittleEndian.Uint32(b[12:]); info.Serial == "" && serial != 0 {
		info.Serial = fmt.Sprintf("0x%08X", serial)
	}
	return info, nil
}

func eachDisplayDescriptor(b []byte, visit func(tag byte, d []byte)) {
	for i := range descriptorCount {
		d := b[descriptorStart+i*descriptorSize:][:descriptorSize]
		if d[0] == 0 && d[1] == 0 {
			visit(d[3], d)
		}
	}
}

func checkBase(b []byte) error {
	if len(b) < blockSize || !bytes.Equal(b[:8], header) {
		return fmt.Errorf("%w: %d bytes without the EDID header", ErrInvalid, len(b))
	}
	if sum := checksum(b[:blockSize]); sum != 0 {
		return fmt.Errorf("%w: base block checksum is off by %d", ErrInvalid, sum)
	}
	return nil
}

func descriptorText(b []byte) string {
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		b = b[:i]
	}
	return strings.TrimSpace(string(b))
}

// Description is make, model and serial joined by spaces, empty ones
// left out.
func (i Info) Description() string {
	parts := make([]string, 0, 3)
	for _, p := range []string{i.Make, i.Model, i.Serial} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " ")
}
