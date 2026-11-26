package app

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/third_party/typesetting/fontscan"
)

func TestUnsharedFontsAreTheUIAndDefaultFacesSharedWithNothing(t *testing.T) {
	first, err := UnsharedFonts()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 3 {
		t.Fatalf("%d faces, want Geist, its bold and Geist Mono at least", len(first))
	}
	if f := first[0].Describe().Family; f != "Geist" {
		t.Errorf("the first face is %s, want Geist", f)
	}
	if v := first[1].Variations(); len(v) != 1 || v[0].Tag != "wght" || v[0].Value != 700 {
		t.Errorf("the second face is at %v, want Geist at wght 700", v)
	}
	if f := first[2].Describe().Family; f != "Geist Mono" {
		t.Errorf("the third face is %s, want Geist Mono", f)
	}
	families := make([]string, 0, len(first))
	for _, f := range first[2:] {
		if fam := f.Describe().Family; slices.Contains(families, fam) {
			t.Errorf("%s is read twice", fam)
		} else {
			families = append(families, fam)
		}
	}
	for _, generic := range []string{fontscan.SansSerif, fontscan.Serif, fontscan.Monospace} {
		shared, err := Font(generic, 14)
		if err != nil {
			t.Fatal(err)
		}
		if fam := shared.Describe().Family; fam != "Geist" && !slices.Contains(families, fam) {
			t.Errorf("%s resolves to %s, which is not among %v", generic, fam, families)
		}
	}

	again, err := UnsharedFonts()
	if err != nil {
		t.Fatal(err)
	}
	ui, err := UIFont(14)
	if err != nil {
		t.Fatal(err)
	}
	mono, err := UIMonoFont(14)
	if err != nil {
		t.Fatal(err)
	}
	shared := []*render.Typeface{ui, mono}
	for _, g := range []string{fontscan.SansSerif, fontscan.Serif, fontscan.Monospace} {
		f, err := Font(g, 14)
		if err != nil {
			t.Fatal(err)
		}
		shared = append(shared, f)
	}
	for i, f := range first {
		if slices.Contains(shared, f) || slices.Contains(again, f) {
			t.Errorf("face %d (%s) is shared", i, f.Describe().Family)
		}
	}
}
