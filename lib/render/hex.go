package render

import "strings"

// ParseHex reads a CSS hex color — #rgb, #rgba, #rrggbb or #rrggbbaa,
// in straight alpha, a short form doubling each digit — into a
// premultiplied Color through [RGBA].
func ParseHex(s string) (Color, bool) {
	digits, ok := strings.CutPrefix(s, "#")
	if !ok {
		return 0, false
	}
	ch := [4]uint8{3: 0xff}
	switch len(digits) {
	case 3, 4:
		for i := range len(digits) {
			v, ok := hexDigit(digits[i])
			if !ok {
				return 0, false
			}
			ch[i] = v * 0x11
		}
	case 6, 8:
		for i := range len(digits) / 2 {
			hi, ok1 := hexDigit(digits[2*i])
			lo, ok2 := hexDigit(digits[2*i+1])
			if !ok1 || !ok2 {
				return 0, false
			}
			ch[i] = hi<<4 | lo
		}
	default:
		return 0, false
	}
	return RGBA(ch[0], ch[1], ch[2], ch[3]), true
}

// FormatHex writes c as the hex form [ParseHex] reads back, in straight
// alpha: #rrggbb when opaque, #rrggbbaa otherwise.
func FormatHex(c Color) string {
	const digits = "0123456789abcdef"
	ch := c.Straight()
	n := len(ch)
	if ch[3] == 0xff {
		n--
	}
	b := make([]byte, 1, 1+2*len(ch))
	b[0] = '#'
	for _, v := range ch[:n] {
		b = append(b, digits[v>>4], digits[v&0xf])
	}
	return string(b)
}

func hexDigit(c byte) (uint8, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
