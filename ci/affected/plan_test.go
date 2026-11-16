package main

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var testComponents = map[string]Component{
	"repository": {Paths: []string{"go.mod", "nix/"}, AffectsAll: true, Jobs: []Job{{Name: "tidy", Runners: []Runner{X86_64}, Run: "just tidy-check"}}},
	"lib":        {Paths: []string{"docs/lib/xkb.md"}, Gates: []string{"vet", "test"}, Race: true},
	"ui":         {Gates: []string{"test"}, Jobs: []Job{{Name: "GPU", Runners: []Runner{X86_64}, Run: "just in ui gpu"}}},
	"wm":         {Depends: []string{"ui"}, Package: true, VMs: []string{"wm-vm", "wm-vm-apps"}, FreeDisk: true},
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

func TestThePlanRunsTheGoGatesOncePerRunnerAndRaceOnce(t *testing.T) {
	p := plan(testComponents, []string{"lib", "polkit", "ui"})
	want := []Matrix{
		{ID: "go-x86-64", Name: "Go gates (x86_64)", Runner: "ubuntu-latest", Run: "just go-gates lib ui", UserNS: true},
		{ID: "race-x86-64", Name: "race (x86_64)", Runner: "ubuntu-latest", Run: "just race-gates lib", UserNS: true},
		{ID: "go-aarch64", Name: "Go gates (aarch64)", Runner: "ubuntu-24.04-arm", Run: "just go-gates lib ui", UserNS: true},
		{ID: "ui-gpu-x86-64", Name: "ui / GPU (x86_64)", Runner: "ubuntu-latest", Run: "just in ui gpu"},
	}
	if !slices.Equal(p.Jobs, want) {
		t.Fatalf("jobs\n%+v\nwant\n%+v", p.Jobs, want)
	}
}

func TestEachVMCheckRunsInItsOwnJobOnTheBuildStagesClosures(t *testing.T) {
	p := plan(testComponents, []string{"login", "wm"})
	if !slices.Equal(p.Packages, []string{"login", "wm"}) {
		t.Errorf("packages %v, want login and wm", p.Packages)
	}
	want := []Matrix{
		{ID: "wm-vm-x86-64", Name: "wm-vm (x86_64)", Runner: "ubuntu-latest", Run: "just vm-checks wm-vm", KVM: true, FreeDisk: true},
		{ID: "wm-vm-apps-x86-64", Name: "wm-vm-apps (x86_64)", Runner: "ubuntu-latest", Run: "just vm-checks wm-vm-apps", KVM: true, FreeDisk: true},
	}
	if !slices.Equal(p.Jobs, want) {
		t.Fatalf("jobs %+v, want %+v", p.Jobs, want)
	}
	builds := []Build{{Arch: X86_64, Runner: "ubuntu-latest", Components: "login wm", VMs: "wm-vm wm-vm-apps"}}
	if !slices.Equal(p.Builds, builds) {
		t.Fatalf("builds %+v, want %+v", p.Builds, builds)
	}
}

func TestEachArchitectureWithAJobGetsOneBuildOfEveryAffectedComponent(t *testing.T) {
	p := plan(testComponents, []string{"lib", "ui"})
	want := []Build{
		{Arch: X86_64, Runner: "ubuntu-latest", Components: "lib ui"},
		{Arch: AArch64, Runner: "ubuntu-24.04-arm", Components: "lib ui"},
	}
	if !slices.Equal(p.Builds, want) {
		t.Fatalf("builds %+v, want %+v", p.Builds, want)
	}
}

func TestAPlanWithoutJobsBuildsNothing(t *testing.T) {
	if p := plan(testComponents, []string{"polkit"}); len(p.Builds) != 0 || len(p.Jobs) != 0 {
		t.Fatalf("polkit has no jobs, yet the plan builds %+v and runs %+v", p.Builds, p.Jobs)
	}
}

func TestAnEmptyPlanIsEmptyListsNotNull(t *testing.T) {
	p := plan(testComponents, nil)
	if p.Jobs == nil || p.Packages == nil || p.Builds == nil {
		t.Fatalf("an empty plan has nil lists, which GitHub reads as null: %+v", p)
	}
}

func TestValidateRefusesUnknownNames(t *testing.T) {
	bad := map[string]Component{
		"wm": {Depends: []string{"wn"}, VMs: []string{"login-vm"}, Jobs: []Job{
			{Name: "x", Runners: []Runner{"riscv"}, Run: "true"},
			{Name: "", Runners: []Runner{X86_64}, Run: "true"},
			{Name: "idle", Runners: []Runner{X86_64}},
			{Name: "nowhere", Run: "true"},
		}},
		"ui": {FreeDisk: true},
	}
	err := validate(bad)
	if err == nil {
		t.Fatal("validate accepted an unknown dependency, runner, an unnamed job and a foreign VM check")
	}
	for _, want := range []string{`"wn"`, `"riscv"`, "without a name", "wm / idle has no command", "wm / nowhere runs on no runner", `"login-vm", which is not wm's`, "ui frees disk for VM checks it does not have"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestValidateAcceptsWellFormedJobs(t *testing.T) {
	if err := validate(testComponents); err != nil {
		t.Fatalf("validate refused the test components: %v", err)
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

var recipeLine = regexp.MustCompile(`(?m)^@?([a-z][a-z0-9_-]*)([^\n]*?):(?:[ \t]|$)`)

func recipesOf(justfile string) []string {
	var recipes []string
	for _, m := range recipeLine.FindAllStringSubmatch(justfile, -1) {
		if rest := strings.TrimSpace(m[2]); !strings.HasPrefix(rest, ":=") && !strings.Contains(rest, " := ") {
			recipes = append(recipes, m[1])
		}
	}
	return recipes
}

func TestEveryGateIsARecipeOfItsComponent(t *testing.T) {
	components, err := load(filepath.Join("..", "components.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range slices.Sorted(maps.Keys(components)) {
		c := components[name]
		want := slices.Clone(c.Gates)
		if c.Race {
			want = append(want, "race")
		}
		if len(want) == 0 {
			continue
		}
		data, err := os.ReadFile(filepath.Join("..", "..", name, "justfile"))
		if err != nil {
			t.Fatalf("%s runs gates, but its justfile does not read: %v", name, err)
		}
		have := recipesOf(string(data))
		for _, recipe := range want {
			if !slices.Contains(have, recipe) {
				t.Errorf("ci/components.json runs %s's gate %q, which %s/justfile has no recipe for", name, recipe, name)
			}
		}
	}
}

func TestTheRecipeMatcherFindsRecipesAndSkipsAssignments(t *testing.T) {
	justfile := "set shell := [\"bash\", \"-c\"]\ntest_dir := \"/tmp/a: b\"\ntest *args:\n    go test\nrace:\n    go test -race\n[private]\nwrite-baselines args=\"\":\ninitramfs-tools image=\"ubuntu:24.04\":\n"
	if got, want := recipesOf(justfile), []string{"test", "race", "write-baselines", "initramfs-tools"}; !slices.Equal(got, want) {
		t.Fatalf("recipes %v, want %v", got, want)
	}
}
