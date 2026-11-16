package oksvg

import (
	"image/color"
	"testing"
)

func TestAnRGBComponentIsClampedToItsByte(t *testing.T) {
	for src, want := range map[string]color.NRGBA{
		"rgb(255,100%,0)":                         {255, 255, 0, 0xFF},
		"rgb(0,0%,254)":                           {0, 0, 254, 0xFF},
		"rgb(256,101%,-1)":                        {255, 255, 0, 0xFF},
		"rgb(1000,200%,-10)":                      {255, 255, 0, 0xFF},
		"rgb(-1%,9223372036854775807,9223372036854775807%)": {0, 255, 255, 0xFF},
	} {
		got, err := ParseSVGColor(src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if got != want {
			t.Errorf("%s parsed to %v, want %v", src, got, want)
		}
	}
	for _, src := range []string{"rgb(x,0,0)", "rgb(0,y%,0)", "rgb(0,0,9223372036854775808)"} {
		if _, err := ParseSVGColor(src); err == nil {
			t.Errorf("%s parsed", src)
		}
	}
}
