package harfbuzz

import (
	"errors"
	"strconv"
	"testing"

	ot "github.com/stubbedev/lys/lib/third_party/typesetting/font/opentype"
)

func TestAFeatureNumberIsA32BitCount(t *testing.T) {
	for src, want := range map[string]Feature{
		"kern=4294967295":         {Value: 4294967295, Start: FeatureGlobalStart, End: FeatureGlobalEnd},
		"kern[4294967295]":        {Value: 1, Start: 4294967295, End: 4294967296},
		"kern[0:4294967295]=7":    {Value: 7, Start: 0, End: 4294967295},
		"kern[4294967294:]=0":     {Value: 0, Start: 4294967294, End: FeatureGlobalEnd},
	} {
		got, err := ParseFeature(src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		want.Tag = ot.MustNewTag("kern")
		if got != want {
			t.Errorf("%s parsed to %+v, want %+v", src, got, want)
		}
	}
	for _, src := range []string{"kern=4294967296", "kern[4294967296]", "kern[0:4294967296]", "kern=18446744073709551617"} {
		if _, err := ParseFeature(src); !errors.Is(err, strconv.ErrRange) {
			t.Errorf("%s parsed with %v, want %v", src, err, strconv.ErrRange)
		}
	}
}
