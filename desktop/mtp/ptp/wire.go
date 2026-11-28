package ptp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
	"unicode/utf16"

	"github.com/stubbedev/lys/lib/intconv"
)

// ErrProtocol is a container or dataset that breaks the protocol.
var ErrProtocol = errors.New("ptp: protocol violation")

type decoder struct {
	b   []byte
	err error
}

func (d *decoder) take(n int) []byte {
	if d.err != nil {
		return nil
	}
	if len(d.b) < n {
		d.err = fmt.Errorf("%w: %d bytes wanted, %d left", ErrProtocol, n, len(d.b))
		return nil
	}
	v := d.b[:n]
	d.b = d.b[n:]
	return v
}

func (d *decoder) u8() uint8 {
	if b := d.take(1); b != nil {
		return b[0]
	}
	return 0
}

func (d *decoder) u16() uint16 {
	if b := d.take(2); b != nil {
		return binary.LittleEndian.Uint16(b)
	}
	return 0
}

func (d *decoder) u32() uint32 {
	if b := d.take(4); b != nil {
		return binary.LittleEndian.Uint32(b)
	}
	return 0
}

func (d *decoder) u64() uint64 {
	if b := d.take(8); b != nil {
		return binary.LittleEndian.Uint64(b)
	}
	return 0
}

func (d *decoder) str() string {
	n := int(d.u8())
	if n == 0 {
		return ""
	}
	b := d.take(2 * n)
	if b == nil {
		return ""
	}
	units := make([]uint16, n)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	if units[n-1] == 0 {
		units = units[:n-1]
	}
	return string(utf16.Decode(units))
}

func (d *decoder) count(size int) int {
	n := d.u32()
	if d.err == nil && uint64(n)*intconv.Must[uint64](size) > uint64(len(d.b)) {
		d.err = fmt.Errorf("%w: an array of %d %d-byte elements in %d bytes", ErrProtocol, n, size, len(d.b))
		return 0
	}
	return int(n)
}

func (d *decoder) u16s() []uint16 {
	v := make([]uint16, d.count(2))
	for i := range v {
		v[i] = d.u16()
	}
	return v
}

func (d *decoder) u32s() []uint32 {
	v := make([]uint32, d.count(4))
	for i := range v {
		v[i] = d.u32()
	}
	return v
}

func (d *decoder) date(loc *time.Location) time.Time {
	s := d.str()
	if d.err != nil {
		return time.Time{}
	}
	t, err := ParseDate(s, loc)
	if err != nil {
		d.err = err
	}
	return t
}

func (d *decoder) done(what string) error {
	if d.err != nil {
		return fmt.Errorf("ptp: decode %s: %w", what, d.err)
	}
	return nil
}

func appendU16(b []byte, v uint16) []byte { return binary.LittleEndian.AppendUint16(b, v) }
func appendU32(b []byte, v uint32) []byte { return binary.LittleEndian.AppendUint32(b, v) }
func appendU64(b []byte, v uint64) []byte { return binary.LittleEndian.AppendUint64(b, v) }

const maxStringUnits = 255

// AppendString appends s as a PTP string: a count of UTF-16 code units
// with the terminating NUL, then the units; "" is a zero count.
func AppendString(b []byte, s string) ([]byte, error) {
	if s == "" {
		return append(b, 0), nil
	}
	units := utf16.Encode([]rune(s))
	if len(units)+1 > maxStringUnits {
		return b, fmt.Errorf("ptp: %q is %d UTF-16 units, more than a PTP string holds", s, len(units))
	}
	b = append(b, intconv.Must[uint8](len(units)+1))
	for _, u := range units {
		b = appendU16(b, u)
	}
	return appendU16(b, 0), nil
}

func appendU16s(b []byte, v []uint16) []byte {
	b = appendU32(b, intconv.Must[uint32](len(v)))
	for _, x := range v {
		b = appendU16(b, x)
	}
	return b
}

func appendU32s(b []byte, v []uint32) []byte {
	b = appendU32(b, intconv.Must[uint32](len(v)))
	for _, x := range v {
		b = appendU32(b, x)
	}
	return b
}

const dateLayout = "20060102T150405"

// ParseDate reads a PTP DateTime string, YYYYMMDDThhmmss with optional
// tenths of a second and an optional Z or ±hhmm zone. A date without a
// zone is wall clock time in loc; "" is the zero Time.
func ParseDate(s string, loc *time.Location) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if len(s) < len(dateLayout) {
		return time.Time{}, fmt.Errorf("%w: date %q is too short", ErrProtocol, s)
	}
	base, rest := s[:len(dateLayout)], s[len(dateLayout):]
	var tenths time.Duration
	if len(rest) >= 2 && rest[0] == '.' && rest[1] >= '0' && rest[1] <= '9' {
		tenths = time.Duration(rest[1]-'0') * 100 * time.Millisecond
		rest = rest[2:]
	}
	switch {
	case rest == "":
	case rest == "Z":
		loc = time.UTC
	case len(rest) == 5 && (rest[0] == '+' || rest[0] == '-'):
		zone, err := time.Parse("-0700", rest)
		if err != nil {
			return time.Time{}, fmt.Errorf("%w: date %q has a malformed zone: %w", ErrProtocol, s, err)
		}
		loc = zone.Location()
	default:
		return time.Time{}, fmt.Errorf("%w: date %q has trailing %q", ErrProtocol, s, rest)
	}
	t, err := time.ParseInLocation(dateLayout, base, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: date %q: %w", ErrProtocol, s, err)
	}
	return t.Add(tenths), nil
}

// FormatDate is t as a PTP DateTime string in loc's wall clock, the
// zero Time as "".
func FormatDate(t time.Time, loc *time.Location) string {
	if t.IsZero() {
		return ""
	}
	return t.In(loc).Format(dateLayout)
}
