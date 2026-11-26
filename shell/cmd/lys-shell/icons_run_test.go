package main

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

var pickerNames = []string{"ld-volume-symbolic", "tb-power", "ld-power-symbolic", "ld-power-off-symbolic", "si-firefox"}

func TestIconsRankBestMatchFirst(t *testing.T) {
	got := rankIcons(iconMatcher(pickerNames), pickerNames, "ldpow")
	if len(got) != 2 || !slices.Contains(got, "ld-power-symbolic") || !slices.Contains(got, "ld-power-off-symbolic") {
		t.Fatalf("ldpow matched %q, want the two ld-power icons", got)
	}
	if got := rankIcons(iconMatcher(pickerNames), pickerNames, "power"); len(got) != 3 || got[0] != "tb-power" {
		t.Errorf("power ranked %q, want the shortest exact match first among the three", got)
	}
}

func TestIconsNoMatchIsEmpty(t *testing.T) {
	if got := rankIcons(iconMatcher(pickerNames), pickerNames, "zzq"); len(got) != 0 {
		t.Errorf("zzq matched %q", got)
	}
}

func TestThePickerCopiesTheNumberedMatch(t *testing.T) {
	var out strings.Builder
	var picked []string
	err := pickIcon(strings.NewReader("zzq\nldpow\n1\n"), &out, pickerNames, func(n string) error {
		picked = append(picked, n)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	first := rankIcons(iconMatcher(pickerNames), pickerNames, "ldpow")[0]
	if !slices.Equal(picked, []string{first}) {
		t.Errorf("picked %q, want [%s]", picked, first)
	}
	if !strings.Contains(out.String(), "No icons match 'zzq'\n") || !strings.Contains(out.String(), "  1  "+first+"\n") || !strings.Contains(out.String(), "Pick 1-2, or search again") {
		t.Errorf("output %q", out.String())
	}
}

func TestThePickerPicksNothingOnAnEmptyLineOrTheEnd(t *testing.T) {
	for _, in := range []string{"", "power\n", "power\n\n", "zzq\n9\n"} {
		err := pickIcon(strings.NewReader(in), &strings.Builder{}, pickerNames, func(n string) error {
			t.Errorf("input %q picked %s", in, n)
			return nil
		})
		if err != nil {
			t.Errorf("input %q: %v", in, err)
		}
	}
}

func TestThePickersCopyFailureIsTheError(t *testing.T) {
	refused := errors.New("no shell")
	if err := pickIcon(strings.NewReader("power\n2\n"), &strings.Builder{}, pickerNames, func(string) error { return refused }); !errors.Is(err, refused) {
		t.Errorf("err %v, want the copy's", err)
	}
}
