package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// CPU's Temperature Sensor read the stored "auto" in its field while
// every other picker reads Auto. A choice's field shows a special value
// by its label and writes the label typed back as the value.
func TestAChoicesFieldReadsASpecialValueByItsLabel(t *testing.T) {
	k := testKit(t, "")
	c, ok := fieldControl(k, k.store, field("modules.cpu.temp-sensor")).(*choiceEditor)
	if !ok {
		t.Fatal("the temperature sensor is no choice")
	}
	auto := locale.T().Get("settings-choice-auto")
	if v := k.store.value("modules.cpu.temp-sensor"); v != "auto" {
		t.Fatalf("the test needs the default auto, the store holds %v", v)
	}
	if c.entry.Text() != auto {
		t.Errorf("auto reads %q in the field, want %q", c.entry.Text(), auto)
	}
	c.entry.SetText("coretemp")
	c.entry.OnActivate("coretemp")
	if v := k.store.value("modules.cpu.temp-sensor"); v != "coretemp" {
		t.Fatalf("typing a sensor stored %v", v)
	}
	c.entry.SetText(auto)
	c.entry.OnActivate(auto)
	if v := k.store.value("modules.cpu.temp-sensor"); v != "auto" {
		t.Errorf("typing %q stored %v, want auto", auto, v)
	}
}

// Replay Output read "focused", the raw value: every special value a
// text field of lys-shell's config takes reads as its label, never as
// itself, and a value with no label is refused loudly.
func TestEverySpecialValueReadsAsItsLabel(t *testing.T) {
	seen := 0
	for _, path := range config.LeafPaths() {
		meta, ok := config.Field(path)
		if !ok || meta.Kind != config.FieldText {
			continue
		}
		for _, v := range meta.Variants {
			seen++
			if got := specialLabel(v); got == v || got == "" || !locale.T().Has(specialKeys[v]) {
				t.Errorf("%s: the special value %q reads %q", path, v, got)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no text field takes a special value, so the walk is wrong")
	}
	defer func() {
		if recover() == nil {
			t.Error("a special value with no label read as something")
		}
	}()
	specialLabel("no-such-special")
}
