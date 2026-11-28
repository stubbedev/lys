package config

import (
	"strings"
	"testing"
)

func TestParseHexColorTakesTheFourCSSForms(t *testing.T) {
	for s, want := range map[string][4]uint8{
		"#abc":      {0xaa, 0xbb, 0xcc, 0xff},
		"#abc8":     {0xaa, 0xbb, 0xcc, 0x88},
		"#A1b2C3":   {0xa1, 0xb2, 0xc3, 0xff},
		"#a1b2c300": {0xa1, 0xb2, 0xc3, 0x00},
	} {
		c, err := ParseHexColor(s)
		if err != nil {
			t.Errorf("ParseHexColor(%q): %v", s, err)
			continue
		}
		if r, g, b, a := c.RGBA(); [4]uint8{r, g, b, a} != want || c.String() != s {
			t.Errorf("ParseHexColor(%q) = %q %v, want %v", s, c, [4]uint8{r, g, b, a}, want)
		}
	}
}

func TestParseHexColorRefusesWhatIsNoHexColor(t *testing.T) {
	for _, s := range []string{"", "#", "abc", "#ab", "#abcde", "#abcdefa", "#abcdefabc", "#ggg", "#12 4", "red"} {
		c, err := ParseHexColor(s)
		if err == nil || !strings.Contains(err.Error(), "hex color") || c != (HexColor{}) {
			t.Errorf("ParseHexColor(%q) = %q, %v, want a hex color error", s, c, err)
		}
	}
}
