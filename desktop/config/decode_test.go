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

func TestAListElementThatFailsToDecodeIsNamedByItsIndex(t *testing.T) {
	for _, tc := range []struct {
		entries, field string
	}{
		{"      - {abov: 70}\n      - {above: 90}\n", "modules.audio.thresholds[0].abov"},
		{"      - {above: 70}\n      - {abov: 90}\n", "modules.audio.thresholds[1].abov"},
	} {
		_, err := LoadFile(writeConfig(t, "modules:\n  audio:\n    thresholds:\n"+tc.entries))
		diags := diagnosticsTitled(err, "invalid config value")
		if len(diags) != 1 || diags[0].Field("Field") != tc.field || !strings.HasPrefix(diags[0].Field("Error"), "unknown field") {
			t.Errorf("diagnostics %+v from %v, want one unknown field at %s", diags, err, tc.field)
		}
	}
}

func TestAMapEntryThatFailsToDecodeIsNamedByItsKey(t *testing.T) {
	_, err := LoadFile(writeConfig(t, "modules:\n  workspaces:\n    workspace-map:\n      \"1\": {label: web}\n      \"2\": {lable: term}\n"))
	diags := diagnosticsTitled(err, "invalid config value")
	if len(diags) != 1 || diags[0].Field("Field") != "modules.workspaces.workspace-map.2.lable" {
		t.Errorf("diagnostics %+v from %v, want one at modules.workspaces.workspace-map.2.lable", diags, err)
	}
}

func TestAValidListAndMapDecodeWithoutADiagnostic(t *testing.T) {
	cfg, err := LoadFile(writeConfig(t, "modules:\n  audio:\n    thresholds:\n      - {above: 70}\n      - {below: 10}\n  workspaces:\n    workspace-map:\n      \"1\": {label: web}\n"))
	if err != nil {
		t.Fatalf("a valid list and map: %v", err)
	}
	if len(cfg.Audio.Thresholds) != 2 || *cfg.Audio.Thresholds[1].Below != 10 || *cfg.Workspaces.WorkspaceMap["1"].Label != "web" {
		t.Errorf("thresholds %+v, tag-map %+v", cfg.Audio.Thresholds, cfg.Workspaces.WorkspaceMap)
	}
}

func TestFailedPathRendersEachStepItPassed(t *testing.T) {
	err := &fieldError{Key: "list", Err: &indexError{Index: 2, Err: &fieldError{Key: "key", Err: errors.New("bad")}}}
	at, cause := failedPath("section", err)
	if at != "section.list[2].key" || cause == nil || cause.Error() != "bad" {
		t.Errorf("failed at %q with %v, want section.list[2].key: bad", at, cause)
	}
}
