package dbus_test

import (
	"testing"

	"github.com/stubbedev/lys/lib/third_party/godbus"
)

func TestAVariantUnicodeEscapeIsACodePoint(t *testing.T) {
	for src, want := range map[string]string{
		`"￿"`:     "￿",
		`"\U0010ffff"`: "\U0010ffff",
		`"\U00000041"`: "A",
	} {
		v, err := dbus.ParseVariant(src, dbus.Signature{})
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if got := v.Value(); got != want {
			t.Errorf("%s parsed to %q, want %q", src, got, want)
		}
	}
	for _, src := range []string{`"\U00110000"`, `"\U7fffffff"`, `"\U80000000"`, `"\Uffffffff"`, `"\ud800"`, `"\U0000d800"`, `"\u12"`} {
		if v, err := dbus.ParseVariant(src, dbus.Signature{}); err == nil {
			t.Errorf("%s parsed to %q", src, v.Value())
		}
	}
}
