package config

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type aliasedValue struct {
	Name string `cfg:"name,alias=label"`
	Old  string `cfg:"old,deprecated=legacy"`
}

func (aliasedValue) configValue() {}

func TestAValueStructReadsItsAliases(t *testing.T) {
	v, err := decodeLeaf(reflect.TypeFor[aliasedValue](), map[string]any{"label": "a", "legacy": "b"})
	if err != nil {
		t.Fatal(err)
	}
	if got := v.Interface().(aliasedValue); got != (aliasedValue{Name: "a", Old: "b"}) {
		t.Errorf("decoded %+v, want the aliased keys read", got)
	}
}

func TestAValueStructRejectsAnUnknownKey(t *testing.T) {
	tree := newTable()
	tree.set("name", "a")
	tree.set("nam", "b")
	_, err := decodeLeaf(reflect.TypeFor[aliasedValue](), tree)
	at, cause := failedPath("section.value", err)
	if at != "section.value.nam" || cause == nil || cause.Error() != "unknown field, expected `name` or `old`" {
		t.Errorf("failed at %q with %v, want an unknown field at section.value.nam", at, cause)
	}
}

func TestAConfigFileKeyUnknownToAValueStructIsADiagnostic(t *testing.T) {
	path := writeFile(t, t.TempDir(), "config.yaml", "animations:\n  osd: {entr: fade, exit: fade}\n  menu: {enter: fade}\n")
	cfg, err := LoadFile(path)
	var d Diagnostic
	if !errors.As(err, &d) || d.Field("Field") != "animations.osd.entr" || !strings.HasPrefix(d.Field("Error"), "unknown field") {
		t.Fatalf("error %v, want an unknown field at animations.osd.entr", err)
	}
	if cfg.Animations.Osd.Exit != nil {
		t.Errorf("osd exit = %v, want the rejected table not applied", *cfg.Animations.Osd.Exit)
	}
	if got := cfg.Animations.Menu.Enter; got == nil || *got != AnimationFade {
		t.Errorf("menu enter = %v, want the valid sibling applied", got)
	}
}
