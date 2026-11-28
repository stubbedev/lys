package render

import "testing"

func TestParseHexReadsEveryCSSForm(t *testing.T) {
	for s, want := range map[string]Color{
		"#f80":      RGB(0xff, 0x88, 0x00),
		"#F80":      RGB(0xff, 0x88, 0x00),
		"#f808":     RGBA(0xff, 0x88, 0x00, 0x88),
		"#e0947a":   RGB(0xe0, 0x94, 0x7a),
		"#ffffff80": RGBA(0xff, 0xff, 0xff, 0x80),
		"#00000000": 0,
	} {
		if got, ok := ParseHex(s); !ok || got != want {
			t.Errorf("ParseHex(%q) = %#08x, %v; want %#08x", s, uint32(got), ok, uint32(want))
		}
	}
}

func TestParseHexRejectsAnythingElse(t *testing.T) {
	for _, s := range []string{"", "#", "f80", "x112233", "#12", "#12345", "#1234567", "#123456789", "#gg0000", "#12 345", " #123456", "#+12345"} {
		if got, ok := ParseHex(s); ok {
			t.Errorf("ParseHex(%q) = %#08x, want rejected", s, uint32(got))
		}
	}
}

func TestFormatHexRoundTrips(t *testing.T) {
	for _, s := range []string{"#000000", "#e0947a", "#ffffff", "#ffffff80", "#00000080", "#00000000"} {
		c, ok := ParseHex(s)
		if got := FormatHex(c); !ok || got != s {
			t.Errorf("FormatHex(ParseHex(%q)) = %q, %v", s, got, ok)
		}
	}
	if got := FormatHex(RGBA(0x10, 0x20, 0x30, 0)); got != "#00000000" {
		t.Errorf("a transparent color formats as %q, want #00000000", got)
	}
}
