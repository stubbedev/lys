package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var testComponents = map[string]Component{
	"repository": {Paths: []string{"go.mod", "nix/"}, AffectsAll: true, Jobs: []Job{{Name: "tidy", Runners: []Runner{X86_64}, Run: "just tidy-check"}}},
	"lib":        {Paths: []string{"docs/lib/xkb.md"}, Jobs: []Job{{Name: "check", Runners: []Runner{X86_64, AArch64}, Run: "just checks lib-package"}}},
	"ui":         {Jobs: []Job{{Name: "check", Runners: []Runner{X86_64}, Run: "just ui::check"}}},
	"wm":         {Depends: []string{"ui"}, Package: true, Jobs: []Job{{Name: "VM", Runners: []Runner{X86_64}, Run: "nix build .#checks.x86_64-linux.wm-vm", KVM: true, Timeout: 20}}},
	"login":      {Depends: []string{"wm"}, Package: true},
	"polkit":     {},
}

var testImports = map[string][]string{"ui": {"lib"}, "wm": {"lib"}, "login": {"lib", "ui"}, "polkit": {"lib"}}

func TestAChangeAffectsItsComponentAndWhatDependsOnIt(t *testing.T) {
	for _, tc := range []struct {
		changed []string
		want    []string
	}{
		{[]string{"ui/widget/box.go"}, []string{"login", "ui", "wm"}},
		{[]string{"wm/README.md"}, []string{"login", "wm"}},
		{[]string{"polkit/main.go"}, []string{"polkit"}},
		{[]string{"lib/dbusx/watch.go"}, []string{"lib", "login", "polkit", "ui", "wm"}},
		{[]string{"docs/lib/xkb.md"}, []string{"lib", "login", "polkit", "ui", "wm"}},
	} {
		if got := affected(testComponents, testImports, tc.changed); !slices.Equal(got, tc.want) {
			t.Errorf("affected(%v) = %v, want %v", tc.changed, got, tc.want)
		}
	}
}

func TestDocsAndProseAffectNothing(t *testing.T) {
	for _, changed := range [][]string{{"docs/ui/widgets.md"}, {"README.md"}, {"docs/index.md", "AGENTS.md"}, nil} {
		if got := affected(testComponents, testImports, changed); len(got) != 0 {
			t.Errorf("affected(%v) = %v, want nothing", changed, got)
		}
	}
}

func TestASharedFileAffectsEverything(t *testing.T) {
	for _, changed := range [][]string{{"go.mod"}, {"nix/go.nix"}, {"README.md", "nix/shell.nix"}} {
		if got := affected(testComponents, testImports, changed); len(got) != len(testComponents) {
			t.Errorf("affected(%v) = %v, want every component", changed, got)
		}
	}
}

func TestADirectoryPathNeedsItsSlash(t *testing.T) {
	if got := affected(testComponents, nil, []string{"nixos/x"}); len(got) != 0 {
		t.Fatalf("nixos/x matched the nix/ path: %v", got)
	}
}

func TestThePlanRunsEachJobOnEachRunner(t *testing.T) {
	p := plan(testComponents, []string{"lib", "login", "wm"})
	if !slices.Equal(p.Packages, []string{"login", "wm"}) {
		t.Errorf("packages %v, want login and wm", p.Packages)
	}
	if len(p.Jobs) != 3 {
		t.Fatalf("%d jobs, want lib's on two runners and wm's VM: %+v", len(p.Jobs), p.Jobs)
	}
	arm := p.Jobs[1]
	if arm.ID != "lib-check-aarch64" || arm.Runner != "ubuntu-24.04-arm" || arm.Run != "just checks lib-package" || arm.Timeout != defaultTimeout {
		t.Errorf("lib on aarch64: %+v", arm)
	}
	if vm := p.Jobs[2]; !vm.KVM || vm.Timeout != 20 || vm.Name != "wm / VM (x86_64)" {
		t.Errorf("wm's VM: %+v", vm)
	}
}

func TestAnEmptyPlanIsEmptyListsNotNull(t *testing.T) {
	p := plan(testComponents, nil)
	if p.Jobs == nil || p.Packages == nil {
		t.Fatalf("an empty plan has nil lists, which GitHub reads as null: %+v", p)
	}
}

func TestValidateRefusesUnknownNames(t *testing.T) {
	bad := map[string]Component{"wm": {Depends: []string{"wn"}, Jobs: []Job{{Name: "x", Runners: []Runner{"riscv"}, Run: "true"}, {Name: "", Runners: []Runner{X86_64}, Run: "true"}}}}
	err := validate(bad)
	if err == nil {
		t.Fatal("validate accepted an unknown dependency, runner and an unnamed job")
	}
	for _, want := range []string{`"wn"`, `"riscv"`, "without a name"} {
		if !containsText(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestImportsOfMapsPackagesAndTestsToComponents(t *testing.T) {
	list := "m/wm/internal/x m/lib/dbusx m/wm/internal/y fmt\n" +
		"m/wm/internal/x [m/wm/internal/x.test] m/ui/widget m/lib/dbusx\n" +
		"m/wm/internal/x.test m/wm/internal/x [m/wm/internal/x.test] testing\n" +
		"m/ci/affected m/lib/srcroot\n"
	got := importsOf("m", list)
	if !slices.Equal(got["wm"], []string{"lib", "ui"}) || !slices.Equal(got["ci"], []string{"lib"}) {
		t.Fatalf("importsOf = %v", got)
	}
}

func TestTheRepositorysComponentsAreValid(t *testing.T) {
	components, err := load(filepath.Join("..", "components.json"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if _, err := os.Stat(filepath.Join("..", "..", e.Name(), "justfile")); err == nil {
			if _, ok := components[e.Name()]; !ok {
				t.Errorf("%s has a justfile but no entry in ci/components.json, so no change to it runs CI", e.Name())
			}
		}
	}
}

func containsText(s, sub string) bool { return strings.Contains(s, sub) }
