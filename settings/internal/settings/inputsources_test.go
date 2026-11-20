package settings

import (
	"errors"
	"io/fs"
	"os"
	"slices"
	"testing"
)

func inputSources(t *testing.T, k *kit, w *wmBackend) control {
	t.Helper()
	p, ok := pageByID(layout(nil, bothParts), "wm-keyboard")
	if !ok {
		t.Fatal("no keyboard page")
	}
	for _, sec := range p.sections {
		for _, r := range sec.rows {
			if r.path == wmLayoutPath {
				return fieldControl(k, w, r)
			}
		}
	}
	t.Fatal("the keyboard page has no layouts row")
	return nil
}

func TestInputSourcesAreAListOfLayoutsThatKeepsTheVariantsAligned(t *testing.T) {
	k, w, _, files := wmKit(t, "input:\n  keyboard:\n    layout: us,dk\n    variant: \",nodeadkeys\"\n")
	c, ok := inputSources(t, k, w).(*choiceListEditor)
	if !ok {
		t.Fatal("the layouts are not a list of input sources")
	}
	k.pickers = &fakePickers{}
	if got := c.values(); !slices.Equal(got, []any{"us", "dk(nodeadkeys)"}) {
		t.Fatalf("rows %v", got)
	}
	if !containsLabel(c, "Danish (no dead keys)") {
		t.Error("a source does not read as its name")
	}
	p := openAdd(t, c)
	if slices.Contains(p.all, "us") || !slices.Contains(p.all, "us(intl)") {
		t.Fatalf("offered %v", p.all)
	}
	pickName(t, p, "us(intl)")
	if got := fileText(t, files.runtime); got != "input:\n  keyboard:\n    layout: us,dk,us\n    variant: ',nodeadkeys,intl'\n" {
		t.Fatalf("adding wrote %q", got)
	}
	c.move(c.items[2], -1)
	if l, v := w.value(wmLayoutPath), w.value(wmVariantPath); l != "us,us,dk" || v != ",intl,nodeadkeys" {
		t.Fatalf("moving wrote %v / %v", l, v)
	}
	c.remove(c.items[2])
	if l, v := w.value(wmLayoutPath), w.value(wmVariantPath); l != "us,us" || v != ",intl" {
		t.Fatalf("removing wrote %v / %v", l, v)
	}
	c.remove(c.items[1])
	if l, v := w.value(wmLayoutPath), w.value(wmVariantPath); l != "us" || v != "" {
		t.Fatalf("removing the last variant left %v / %v", l, v)
	}
}

func TestSourcesThatDoNotPairUpAreShownAsWrittenAndKept(t *testing.T) {
	const yaml = "input:\n  keyboard:\n    layout: us\n    variant: \"intl,nodeadkeys\"\n"
	k, w, _, files := wmKit(t, yaml)
	c, ok := inputSources(t, k, w).(*misalignedSources)
	if !ok {
		t.Fatal("misaligned layouts and variants are read as a list")
	}
	if len(c.controls) != 2 {
		t.Fatalf("%d controls, want the layout and the variant", len(c.controls))
	}
	if l, v := w.value(wmLayoutPath), w.value(wmVariantPath); l != "us" || v != "intl,nodeadkeys" {
		t.Fatalf("the keys read %v / %v", l, v)
	}
	if _, err := os.Stat(files.runtime); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("showing them wrote the runtime file: %v", err)
	}
}

func TestAnInputSourceIsALayoutOrALayoutWithItsVariant(t *testing.T) {
	x := xkbLists{
		layouts:  []choice{{value: "us", label: "English (US)"}},
		variants: []choice{{value: "intl", label: "English (US, intl.)", group: "us"}},
	}
	got := xkbSources(x)
	if !slices.Equal(values(got), []string{"us", "us(intl)"}) || got[1].label != "English (US, intl.)" {
		t.Fatalf("sources %+v", got)
	}
	for code, want := range map[string][2]string{"us": {"us", ""}, "us(intl)": {"us", "intl"}} {
		if l, v := splitLayoutCode(code); [2]string{l, v} != want {
			t.Errorf("%q splits to %q %q", code, l, v)
		}
	}
}
