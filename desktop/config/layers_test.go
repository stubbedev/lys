package config

import (
	"slices"
	"strings"
	"testing"
)

func TestAnUnknownConfigFileKeyIsAWarningAtItsPath(t *testing.T) {
	cfg, err := LoadFile(writeConfig(t, "bar:\n  locaton: top\n  location: bottom\nmodules:\n  clok: {}\nstyle: {}\n"))
	diags := diagnosticsTitled(err, "unknown config key")
	var fields []string
	for _, d := range diags {
		if d.Kind != DiagnosticWarning {
			t.Errorf("diagnostic %+v, want a warning", d)
		}
		fields = append(fields, d.Field("Field"))
	}
	want := []string{"style", "bar.locaton", "modules.clok"}
	if !slices.Equal(fields, want) {
		t.Fatalf("unknown-key warnings at %v from %v, want %v", fields, err, want)
	}
	if got := diags[1].Field("Error"); !strings.HasPrefix(got, "unknown field, expected one of `layout`") || !strings.Contains(got, "`location`") {
		t.Errorf("error %q, want the section's keys listed", got)
	}
	if cfg.Bar.Location != LocationBottom {
		t.Errorf("bar location = %v, want the known sibling applied", cfg.Bar.Location)
	}
}

func TestKnownAliasedRemovedAndUnappliedKeysAreNotUnknown(t *testing.T) {
	body := "imports: []\nbar:\n  location: top\nmodules:\n  hyprsunset: {}\n  dashboard:\n    dropdown-reboot-command: reboot-now\n"
	_, err := LoadFile(writeConfig(t, body))
	if diags := diagnosticsTitled(err, "unknown config key"); len(diags) != 0 {
		t.Errorf("unknown-key warnings %+v, want none", diags)
	}
	if diags := diagnosticsTitled(err, "removed config key"); len(diags) != 1 {
		t.Errorf("removed-key diagnostics %+v from %v, want the removed key named once", diags, err)
	}
}

func TestARuntimeOverrideAtAnUnknownPathIsNoUnknownKeyWarning(t *testing.T) {
	var diags []Diagnostic
	apply := &layerApply{kind: runtimeLayer, sink: func(d Diagnostic) { diags = append(diags, d) }, staged: newTable()}
	if err := apply.applyContainer(valueOf(Defaults()), map[string]any{"bar": map[string]any{"locaton": "top"}}, ""); err != nil {
		t.Fatal(err)
	}
	if len(diags) != 0 {
		t.Errorf("diagnostics %+v, want SetByPath's own error to name the path instead", diags)
	}
}
