// Package ipp is the slice of the Internet Printing Protocol (RFC 8010
// and 8011) the print dialog needs from CUPS: list the printers
// (CUPS-Get-Printers) and send a job (Print-Job), over the CUPS domain
// socket or CUPS_SERVER. Pure Go; no libcups.
package ipp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/stubbedev/lys/lib/intconv"
)

// Operations.
const (
	OpPrintJob               uint16 = 0x0002
	OpGetPrinterAttributes   uint16 = 0x000B
	OpCUPSGetDefault         uint16 = 0x4001
	OpCUPSGetPrinters        uint16 = 0x4002
	OpCUPSCreateLocalPrinter uint16 = 0x4028
)

// Attribute group delimiters.
const (
	GroupOperation byte = 0x01
	GroupJob       byte = 0x02
	groupEnd       byte = 0x03
	GroupPrinter   byte = 0x04
)

// Value tags.
const (
	TagInteger byte = 0x21
	TagBoolean byte = 0x22
	TagEnum    byte = 0x23
	TagRange   byte = 0x33
	// TagCollection opens a collection value, whose members follow as
	// memberAttrName values each before its own values, up to
	// tagEndCollection.
	TagCollection    byte = 0x34
	tagEndCollection byte = 0x37
	tagMemberName    byte = 0x4A
	TagText          byte = 0x41
	TagName          byte = 0x42
	TagKeyword       byte = 0x44
	TagURI           byte = 0x45
	TagCharset       byte = 0x47
	TagLanguage      byte = 0x48
	TagMimeMedia     byte = 0x49
	tagFirstValue    byte = 0x10
)

// Value is one attribute value: an int for integer, enum and boolean
// (0 or 1), a [2]int32 for a range, a string for the text kinds, the
// members of a collection.
type Value struct {
	Tag     byte
	Int     int32
	Low     int32 // range lower bound; Int is the upper
	Text    string
	Members []Attr
}

// Attr is one named attribute with its values.
type Attr struct {
	Name   string
	Values []Value
}

// Int is an integer attribute.
func Int(name string, v int32) Attr { return Attr{name, []Value{{Tag: TagInteger, Int: v}}} }

// Enum is an enum attribute.
func Enum(name string, v int32) Attr { return Attr{name, []Value{{Tag: TagEnum, Int: v}}} }

// String is a text-kind attribute (name, keyword, uri, ...).
func String(tag byte, name string, vs ...string) Attr {
	a := Attr{Name: name}
	for _, v := range vs {
		a.Values = append(a.Values, Value{Tag: tag, Text: v})
	}
	return a
}

// Collection is a collection attribute of one value, its members.
func Collection(name string, members ...Attr) Attr {
	return Attr{name, []Value{CollectionValue(members...)}}
}

// CollectionValue is a collection value, a member of another one.
func CollectionValue(members ...Attr) Value { return Value{Tag: TagCollection, Members: members} }

// Ranges is a 1-based rangeOfInteger attribute.
func Ranges(name string, rs [][2]int32) Attr {
	a := Attr{Name: name}
	for _, r := range rs {
		a.Values = append(a.Values, Value{Tag: TagRange, Low: r[0], Int: r[1]})
	}
	return a
}

// Group is one attribute group.
type Group struct {
	Tag   byte
	Attrs []Attr
}

// Get is the group's attribute name, false when absent.
func (g Group) Get(name string) (Attr, bool) {
	for _, a := range g.Attrs {
		if a.Name == name {
			return a, true
		}
	}
	return Attr{}, false
}

// Text is the first value of name as text, "" when absent.
func (g Group) Text(name string) string {
	if a, ok := g.Get(name); ok && len(a.Values) > 0 {
		return a.Values[0].Text
	}
	return ""
}

// Texts is every value of name as text, none when absent.
func (g Group) Texts(name string) []string {
	a, _ := g.Get(name)
	out := make([]string, len(a.Values))
	for i, v := range a.Values {
		out[i] = v.Text
	}
	return out
}

// Ints is every value of name as an integer, none when absent.
func (g Group) Ints(name string) []int32 {
	a, _ := g.Get(name)
	out := make([]int32, len(a.Values))
	for i, v := range a.Values {
		out[i] = v.Int
	}
	return out
}

// Int is the first value of name as an integer, false when absent.
func (g Group) Int(name string) (int32, bool) {
	if a, ok := g.Get(name); ok && len(a.Values) > 0 {
		return a.Values[0].Int, true
	}
	return 0, false
}

// Message is a request (Code is the operation) or a response (Code is
// the status).
type Message struct {
	Code      uint16
	RequestID uint32
	Groups    []Group
}

func (m Message) checkLengths() error {
	for _, g := range m.Groups {
		for _, a := range g.Attrs {
			if _, err := fieldLength(a.Name); err != nil {
				return fmt.Errorf("ipp: attribute name %.32q…: %w", a.Name, err)
			}
			for _, v := range a.Values {
				if _, err := fieldLength(v.Text); err != nil {
					return fmt.Errorf("ipp: %s value: %w", a.Name, err)
				}
			}
		}
	}
	return nil
}

func fieldLength(s string) (int16, error) {
	n, err := intconv.Exact[int16](len(s))
	if err != nil {
		return 0, fmt.Errorf("%d bytes exceed the %d a SIGNED-SHORT length field holds (RFC 8010): %w", len(s), math.MaxInt16, err)
	}
	return n, nil
}

// Encode writes the message: version 2.0, the code, the request id,
// the groups, the end tag. It panics on a name or value longer than
// 32767 bytes.
func (m Message) Encode() []byte {
	var b bytes.Buffer
	b.Write([]byte{2, 0})
	_ = binary.Write(&b, binary.BigEndian, m.Code)
	_ = binary.Write(&b, binary.BigEndian, m.RequestID)
	for _, g := range m.Groups {
		b.WriteByte(g.Tag)
		for _, a := range g.Attrs {
			for i, v := range a.Values {
				name := a.Name
				if i > 0 {
					name = "" // an additional value of the same attribute
				}
				b.WriteByte(v.Tag)
				writeString(&b, name)
				writeValue(&b, v)
			}
		}
	}
	b.WriteByte(groupEnd)
	return b.Bytes()
}

func writeString(b *bytes.Buffer, s string) {
	n, err := fieldLength(s)
	if err != nil {
		panic(fmt.Sprintf("ipp: encoding a string: %v", err))
	}
	_ = binary.Write(b, binary.BigEndian, n)
	b.WriteString(s)
}

func writeValue(b *bytes.Buffer, v Value) {
	switch v.Tag {
	case TagInteger, TagEnum:
		_ = binary.Write(b, binary.BigEndian, uint16(4))
		_ = binary.Write(b, binary.BigEndian, v.Int)
	case TagBoolean:
		_ = binary.Write(b, binary.BigEndian, uint16(1))
		b.WriteByte(byte(v.Int & 1))
	case TagRange:
		_ = binary.Write(b, binary.BigEndian, uint16(8))
		_ = binary.Write(b, binary.BigEndian, v.Low)
		_ = binary.Write(b, binary.BigEndian, v.Int)
	case TagCollection:
		writeString(b, "")
		for _, m := range v.Members {
			b.WriteByte(tagMemberName)
			writeString(b, "")
			writeString(b, m.Name)
			for _, mv := range m.Values {
				b.WriteByte(mv.Tag)
				writeString(b, "")
				writeValue(b, mv)
			}
		}
		b.WriteByte(tagEndCollection)
		writeString(b, "")
		writeString(b, "")
	default:
		writeString(b, v.Text)
	}
}

// errShort is a message cut off mid-field.
var errShort = errors.New("ipp: truncated message")

// Decode parses a message; trailing data after the end tag (a job's
// document) is ignored.
func Decode(data []byte) (Message, error) {
	const headLen = 8
	if len(data) < headLen {
		return Message{}, errShort
	}
	m := Message{Code: binary.BigEndian.Uint16(data[2:4]), RequestID: binary.BigEndian.Uint32(data[4:8])}
	d := decoder{data[headLen:]}
	var cur *Group
	for {
		tag, err := d.tag()
		if err != nil {
			return Message{}, err
		}
		switch {
		case tag == groupEnd:
			return m, nil
		case tag < tagFirstValue:
			m.Groups = append(m.Groups, Group{Tag: tag})
			cur = &m.Groups[len(m.Groups)-1]
			continue
		case cur == nil:
			return Message{}, fmt.Errorf("ipp: value tag %#x outside a group", tag)
		}
		name, err := d.field()
		if err != nil {
			return Message{}, err
		}
		v, err := d.value(tag)
		if err != nil {
			return Message{}, err
		}
		if len(name) == 0 {
			if len(cur.Attrs) == 0 {
				return Message{}, errors.New("ipp: additional value with no attribute")
			}
			last := &cur.Attrs[len(cur.Attrs)-1]
			last.Values = append(last.Values, v)
			continue
		}
		cur.Attrs = append(cur.Attrs, Attr{Name: string(name), Values: []Value{v}})
	}
}

type decoder struct{ rest []byte }

func (d *decoder) tag() (byte, error) {
	if len(d.rest) == 0 {
		return 0, errShort
	}
	t := d.rest[0]
	d.rest = d.rest[1:]
	return t, nil
}

func (d *decoder) field() ([]byte, error) {
	if len(d.rest) < 2 {
		return nil, errShort
	}
	n := int(binary.BigEndian.Uint16(d.rest))
	if n > math.MaxInt16 {
		return nil, fmt.Errorf("ipp: length field %#04x exceeds the %d a SIGNED-SHORT holds (RFC 8010)", n, math.MaxInt16)
	}
	body := d.rest[2:]
	if n > len(body) {
		return nil, fmt.Errorf("ipp: a %d-byte field with %d bytes left: %w", n, len(body), errShort)
	}
	d.rest = body[n:]
	return body[:n:n], nil
}

func (d *decoder) value(tag byte) (Value, error) {
	raw, err := d.field()
	if err != nil {
		return Value{}, err
	}
	v, err := parseValue(tag, raw)
	if err != nil || tag != TagCollection {
		return v, err
	}
	v.Members, err = d.members()
	return v, err
}

func (d *decoder) members() ([]Attr, error) {
	var members []Attr
	for {
		tag, err := d.tag()
		if err != nil {
			return nil, err
		}
		if _, err := d.field(); err != nil {
			return nil, err
		}
		switch {
		case tag == tagEndCollection:
			if _, err := d.field(); err != nil {
				return nil, err
			}
			return members, nil
		case tag == tagMemberName:
			name, err := d.field()
			if err != nil {
				return nil, err
			}
			members = append(members, Attr{Name: string(name)})
			continue
		case len(members) == 0:
			return nil, fmt.Errorf("ipp: collection value tag %#x before a member name", tag)
		}
		v, err := d.value(tag)
		if err != nil {
			return nil, err
		}
		last := &members[len(members)-1]
		last.Values = append(last.Values, v)
	}
}

func parseValue(tag byte, raw []byte) (Value, error) {
	v := Value{Tag: tag}
	switch tag {
	case TagInteger, TagEnum:
		if len(raw) != 4 {
			return v, fmt.Errorf("ipp: integer of %d bytes", len(raw))
		}
		v.Int = int32(binary.BigEndian.Uint32(raw))
	case TagBoolean:
		if len(raw) != 1 {
			return v, fmt.Errorf("ipp: boolean of %d bytes", len(raw))
		}
		v.Int = int32(raw[0])
	case TagRange:
		if len(raw) != 8 {
			return v, fmt.Errorf("ipp: range of %d bytes", len(raw))
		}
		v.Low = int32(binary.BigEndian.Uint32(raw[:4]))
		v.Int = int32(binary.BigEndian.Uint32(raw[4:]))
	default:
		v.Text = string(raw)
	}
	return v, nil
}

// StatusOK reports a successful status (successful-ok and its
// "ignored or substituted" variants).
func StatusOK(code uint16) bool { return code < 0x0100 }
