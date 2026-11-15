package audiolevel

import (
	"strings"
	"testing"
)

func TestParseAndApply(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    Level
		from    float64
		reached float64
	}{
		{"40", Level{Percent: 40}, 75, 40},
		{"+5", Level{Percent: 5, Relative: true}, 50, 55},
		{"-10", Level{Percent: -10, Relative: true}, 50, 40},
		{"+20", Level{Percent: 20, Relative: true}, 95, 100},
		{"-20", Level{Percent: -20, Relative: true}, 5, 0},
		{"150", Level{Percent: 150}, 0, 100},
	} {
		got, err := Parse(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", tc.in, got, err, tc.want)
			continue
		}
		if r := got.Apply(tc.from); r != tc.reached {
			t.Errorf("%q from %v reaches %v, want %v", tc.in, tc.from, r, tc.reached)
		}
	}
}

func TestParseRefusesWhatIsNoLevel(t *testing.T) {
	for in, want := range map[string]string{
		"loud": `parse volume level "loud"`,
		"+a":   `parse volume delta "+a"`,
		"-":    `parse volume delta "-"`,
		"":     `parse volume level ""`,
	} {
		if _, err := Parse(in); err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("Parse(%q) = %v, want %s", in, err, want)
		}
	}
}
