package main

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

var testComponents = map[string]Component{
	"repository": {Paths: []string{"go.mod", "nix/"}, GoPackages: "nix/go-packages.json", AffectsAll: true, Jobs: []Job{{Name: "tidy", Runners: []Runner{X86_64}, Run: "just tidy-check"}}},
	"lib":        {Paths: []string{"docs/lib/xkb.md"}, Gates: []string{"vet", "test"}, Race: true},
	"ui":         {Gates: []string{"test"}, Shards: [][]ShardEntry{packages("./widget"), packages("./app", "./anim/...")}, Jobs: []Job{{Name: "GPU", Runners: []Runner{X86_64}, Run: "just in ui gpu"}}},
	"wm":         {Depends: []string{"ui"}, Package: true, VMs: [][]string{{"wm-vm"}, {"wm-vm-apps", "wm-vm-share"}}},
	"login":      {Depends: []string{"wm"}, Package: true, VMs: [][]string{{"login-vm"}}},
	"polkit":     {},
}

var testConfig = Config{
	Components:   testComponents,
	ShardedGates: []string{"test"},
	GoJobs: map[Runner][][]Target{
		X86_64:  {{target("ui/3")}, {target("ui/1"), target("ui/2"), target("lib:race")}, {target("lib:gates")}},
		AArch64: {{target("lib"), target("ui")}},
	},
}

func packages(patterns ...string) []ShardEntry {
	entries := make([]ShardEntry, len(patterns))
	for i, p := range patterns {
		entries[i] = ShardEntry{Package: p}
	}
	return entries
}

func target(s string) Target {
	t, err := parseTarget(s)
	if err != nil {
		panic(err)
	}
	return t
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
		if got := affected(testComponents, testImports, tc.changed, nil); !slices.Equal(got, tc.want) {
			t.Errorf("affected(%v) = %v, want %v", tc.changed, got, tc.want)
		}
	}
}

func TestDocsAndProseAffectNothing(t *testing.T) {
	for _, changed := range [][]string{{"docs/ui/widgets.md"}, {"README.md"}, {"docs/index.md", "AGENTS.md"}, nil} {
		if got := affected(testComponents, testImports, changed, nil); len(got) != 0 {
			t.Errorf("affected(%v) = %v, want nothing", changed, got)
		}
	}
}

func TestASharedFileAffectsEverything(t *testing.T) {
	for _, changed := range [][]string{{"go.mod"}, {"nix/go.nix"}, {"README.md", "nix/shell.nix"}} {
		if got := affected(testComponents, testImports, changed, nil); len(got) != len(testComponents) {
			t.Errorf("affected(%v) = %v, want every component", changed, got)
		}
	}
}

func TestADirectoryPathNeedsItsSlash(t *testing.T) {
	if got := affected(testComponents, nil, []string{"nixos/x"}, nil); len(got) != 0 {
		t.Fatalf("nixos/x matched the nix/ path: %v", got)
	}
}

func TestAChangeToTheGoPackageIndexAffectsTheRepositoryAndTheChangedGoPackagesComponents(t *testing.T) {
	for _, tc := range []struct {
		changed []string
		want    []string
	}{
		{[]string{"nix/go-packages.json", "polkit/main.go"}, []string{"polkit", "repository"}},
		{[]string{"nix/go-packages.json", "ui/widget/box.go"}, []string{"login", "repository", "ui", "wm"}},
		{[]string{"nix/go-packages.json"}, []string{"repository"}},
	} {
		if got := affected(testComponents, testImports, tc.changed, nil); !slices.Equal(got, tc.want) {
			t.Errorf("affected(%v) = %v, want %v", tc.changed, got, tc.want)
		}
	}
}

const goPackagesBefore = `{
  "ci/affected": {"imports": ["lib/srcroot"]},
  "lib/peercred": {},
  "lib/wmipc": {},
  "polkit/agent": {"imports": ["lib/wmipc"]},
  "wm/internal/ipc": {"imports": ["lib/wmipc"]}
}
`

func goPackagesAffected(t *testing.T, before, after string) []string {
	t.Helper()
	dirs, err := changedGoPackages("nix/go-packages.json", []byte(before), []byte(after))
	if err != nil {
		t.Fatal(err)
	}
	return affected(testComponents, testImports, []string{"nix/go-packages.json"}, dirs)
}

func TestAGoPackageIndexChangeToAComponentsEntryAffectsItAndWhatDependsOnIt(t *testing.T) {
	for _, tc := range []struct {
		after string
		want  []string
	}{
		{strings.Replace(goPackagesBefore, `"wm/internal/ipc": {"imports": ["lib/wmipc"]}`, `"wm/internal/ipc": {"imports": ["lib/peercred", "lib/wmipc"]}`, 1), []string{"login", "repository", "wm"}},
		{strings.Replace(goPackagesBefore, `"lib/peercred": {},`, ``, 1), []string{"lib", "login", "polkit", "repository", "ui", "wm"}},
		{strings.Replace(goPackagesBefore, `"lib/wmipc": {},`, `"lib/wmipc": {}, "polkit/rules": {},`, 1), []string{"polkit", "repository"}},
	} {
		if got := goPackagesAffected(t, goPackagesBefore, tc.after); !slices.Equal(got, tc.want) {
			t.Errorf("index\n%s\naffects %v, want %v", tc.after, got, tc.want)
		}
	}
}

func TestAGoPackageIndexRegenerationNoComponentBuildsFromAffectsOnlyTheRepository(t *testing.T) {
	for _, after := range []string{
		goPackagesBefore,
		strings.ReplaceAll(goPackagesBefore, "\n  ", "\n    "),
		strings.Replace(goPackagesBefore, `"ci/affected": {"imports": ["lib/srcroot"]}`, `"ci/affected": {"imports": ["lib/atomicfile", "lib/srcroot"]}`, 1),
	} {
		if got := goPackagesAffected(t, goPackagesBefore, after); !slices.Equal(got, []string{"repository"}) {
			t.Errorf("index\n%s\naffects %v, want only repository", after, got)
		}
	}
}

func TestAGoPackageIndexThatDoesNotParseFailsNamingTheSide(t *testing.T) {
	for _, tc := range []struct{ before, after, want string }{
		{"{", goPackagesBefore, "parse nix/go-packages.json at the base"},
		{goPackagesBefore, "[]", "parse nix/go-packages.json at HEAD"},
	} {
		_, err := changedGoPackages("nix/go-packages.json", []byte(tc.before), []byte(tc.after))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("changedGoPackages(%q, %q) = %v, want an error naming %q", tc.before, tc.after, err, tc.want)
		}
	}
}

func TestTheGoPackageIndexLeavesTheRestOfItsDirectoryShared(t *testing.T) {
	for _, changed := range [][]string{{"nix/go.nix"}, {"nix/go-packages.json", "nix/go.nix"}, {"polkit/main.go", "nix/go-packages.jsonx"}} {
		if got := affected(testComponents, testImports, changed, nil); len(got) != len(testComponents) {
			t.Errorf("affected(%v) = %v, want every component", changed, got)
		}
	}
}

func TestAGoJobRunsItsAffectedTargetsAndOffTheRaceRunnerOnlyTheirGoGates(t *testing.T) {
	p := plan(testConfig, []string{"lib", "ui"})
	want := []Matrix{
		{ID: "ui-gpu-x86-64", Name: "ui / GPU (x86_64)", Runner: "ubuntu-latest", Run: "just in ui gpu"},
		{ID: "go-ui-3-x86-64", Name: "Go ui/3 (x86_64)", Runner: "ubuntu-latest", Run: "just gates ui/3", UserNS: true},
		{ID: "go-ui-1-ui-2-lib-race-x86-64", Name: "Go ui/1 ui/2 lib:race (x86_64)", Runner: "ubuntu-latest", Run: "just gates ui/1 ui/2 lib:race", UserNS: true},
		{ID: "go-lib-gates-x86-64", Name: "Go lib:gates (x86_64)", Runner: "ubuntu-latest", Run: "just gates lib:gates", UserNS: true},
		{ID: "go-lib-ui-aarch64", Name: "Go lib:gates ui:gates (aarch64)", Runner: "ubuntu-24.04-arm", Run: "just gates lib:gates ui:gates", UserNS: true},
	}
	if !slices.Equal(p.Jobs, want) {
		t.Fatalf("jobs\n%+v\nwant\n%+v", p.Jobs, want)
	}
}

func TestAGoJobKeepsItsIDAndCacheWhenOnlyPartOfItIsAffected(t *testing.T) {
	p := plan(testConfig, []string{"lib"})
	want := []Matrix{
		{ID: "go-ui-1-ui-2-lib-race-x86-64", Name: "Go lib:race (x86_64)", Runner: "ubuntu-latest", Run: "just gates lib:race", UserNS: true},
		{ID: "go-lib-gates-x86-64", Name: "Go lib:gates (x86_64)", Runner: "ubuntu-latest", Run: "just gates lib:gates", UserNS: true},
		{ID: "go-lib-ui-aarch64", Name: "Go lib:gates (aarch64)", Runner: "ubuntu-24.04-arm", Run: "just gates lib:gates", UserNS: true},
	}
	if !slices.Equal(p.Jobs, want) {
		t.Fatalf("jobs\n%+v\nwant\n%+v", p.Jobs, want)
	}
}

func TestEachComponentWithVMChecksRunsThemAndItsJobsAfterTheBuildInALaneOfItsOwn(t *testing.T) {
	cfg := testConfig
	cfg.Components = maps.Clone(testComponents)
	wm := cfg.Components["wm"]
	wm.Jobs = []Job{{Name: "image", Runners: []Runner{X86_64}, Run: "just wm::image", AfterBuild: true}}
	cfg.Components["wm"] = wm
	p := plan(cfg, []string{"login", "ui", "wm"})
	if !slices.Equal(p.Packages, []string{"login", "wm"}) {
		t.Errorf("packages %v, want login and wm", p.Packages)
	}
	want := []Lane{
		{
			Name:  "login",
			Build: Matrix{ID: "build-login-x86-64", Name: "login (x86_64)", Runner: "ubuntu-latest", Run: "just build-stage login login-vm"},
			Jobs: []Matrix{
				{ID: "login-vm-x86-64", Name: "login-vm (x86_64)", Runner: "ubuntu-latest", Run: "just vm-checks login-vm", KVM: true},
			},
		},
		{
			Name:  "wm",
			Build: Matrix{ID: "build-wm-x86-64", Name: "wm (x86_64)", Runner: "ubuntu-latest", Run: "just build-stage wm wm-vm wm-vm-apps wm-vm-share"},
			Jobs: []Matrix{
				{ID: "wm-vm-x86-64", Name: "wm-vm (x86_64)", Runner: "ubuntu-latest", Run: "just vm-checks wm-vm", KVM: true},
				{ID: "wm-vm-apps-wm-vm-share-x86-64", Name: "wm-vm-apps + wm-vm-share (x86_64)", Runner: "ubuntu-latest", Run: "just vm-checks wm-vm-apps wm-vm-share", KVM: true},
				{ID: "wm-image-x86-64", Name: "wm / image (x86_64)", Runner: "ubuntu-latest", Run: "just wm::image"},
			},
		},
	}
	if !reflect.DeepEqual(p.Lanes, want) {
		t.Fatalf("lanes\n%+v\nwant\n%+v", p.Lanes, want)
	}
	if len(p.Jobs) == 0 || p.Jobs[0].Run != "just in ui gpu" || slices.ContainsFunc(p.Jobs, func(m Matrix) bool { return m.KVM || m.Run == "just wm::image" }) {
		t.Fatalf("jobs %+v, want the further jobs first, and no VM job or job after a build", p.Jobs)
	}
	if slices.ContainsFunc(p.Builds, func(m Matrix) bool { return strings.Contains(m.Run, "wm") || strings.Contains(m.Run, "login") }) {
		t.Fatalf("builds %+v, want login's and wm's builds only in their lanes", p.Builds)
	}
}

func TestTheDevShellsBuildPerRunnerForItsGoSuitesAndPackages(t *testing.T) {
	p := plan(testConfig, []string{"lib", "login", "ui", "wm"})
	want := []Matrix{
		{ID: "shells-lib-login-ui-wm-x86-64", Name: "dev shells (x86_64)", Runner: "ubuntu-latest", Run: "just build-shells lib login ui wm"},
		{ID: "shells-lib-ui-aarch64", Name: "dev shells (aarch64)", Runner: "ubuntu-24.04-arm", Run: "just build-shells lib ui"},
	}
	if !slices.Equal(p.Shells, want) {
		t.Fatalf("shells\n%+v\nwant\n%+v", p.Shells, want)
	}
	if p := plan(testConfig, []string{"login"}); len(p.Shells) != 0 {
		t.Fatalf("login runs only a VM job, yet the plan builds the dev shells %+v", p.Shells)
	}
}

func TestARunnerInGateRunnersRunsOnlyThoseGates(t *testing.T) {
	cfg := Config{
		Components:  map[string]Component{"a": {Gates: []string{"lint", "test"}}, "b": {Gates: []string{"lint"}}},
		GateRunners: map[Runner][]string{AArch64: {"test"}},
		GoJobs:      map[Runner][][]Target{X86_64: {{target("a"), target("b")}}, AArch64: {{target("a")}}},
	}
	if err := validate(cfg); err != nil {
		t.Fatalf("validate refused gates split by runner: %v", err)
	}
	if got := cfg.gates(cfg.Components["a"], AArch64); !slices.Equal(got, []string{"test"}) {
		t.Errorf("a's gates on aarch64 = %v, want only test", got)
	}
	if got := cfg.gates(cfg.Components["a"], X86_64); !slices.Equal(got, []string{"lint", "test"}) {
		t.Errorf("a's gates on x86_64 = %v, want every gate", got)
	}
	if got := cfg.suites(cfg.Components["b"], AArch64); len(got) != 0 {
		t.Errorf("b has no gate aarch64 runs, yet runs %v there", got)
	}
	cfg.GoJobs[AArch64] = [][]Target{{target("a"), target("b")}}
	wantErrors(t, validate(cfg), []string{"go-jobs on aarch64 names b, but b runs no such suite on aarch64"})
}

func TestValidateRefusesGateRunnersThatDropAGateOrNameAnUnknownOne(t *testing.T) {
	cfg := Config{
		Components:  map[string]Component{"a": {Gates: []string{"lint", "test"}}},
		GateRunners: map[Runner][]string{X86_64: {"test"}, AArch64: {"test", "tset"}, "riscv": {}},
	}
	wantErrors(t, validate(cfg), []string{
		"a's gate lint runs on no runner",
		`gate-runners lets aarch64 run "tset", which is no component's gate`,
		`gate-runners names "riscv", which is no runner`,
		"gate-runners lets riscv run no gate",
	})
}

func TestValidateRefusesAJobAfterABuildItsComponentDoesNotHave(t *testing.T) {
	cfg := Config{Components: map[string]Component{"a": {Jobs: []Job{{Name: "x", Runners: []Runner{X86_64}, Run: "true", AfterBuild: true}}}}}
	wantErrors(t, validate(cfg), []string{"a / x runs after its component's build, but a has no VM checks"})
}

func TestValidateRefusesAJobAfterABuildOnARunnerWithoutThatBuild(t *testing.T) {
	cfg := Config{Components: map[string]Component{"a-vms": {
		VMs:  [][]string{{"a-vms-x"}},
		Jobs: []Job{{Name: "image", Runners: []Runner{X86_64, AArch64}, Run: "true", AfterBuild: true}},
	}}}
	wantErrors(t, validate(cfg), []string{"a-vms / image runs after its component's build, which only x86_64 has, yet runs on [x86_64 aarch64]"})
	cfg.Components["a-vms"].Jobs[0].Runners = []Runner{X86_64}
	if err := validate(cfg); err != nil {
		t.Fatalf("validate refused a job after the build on x86_64: %v", err)
	}
}

func TestComponentsWithVMChecksBuildInTheirLanesAndTheRestTogether(t *testing.T) {
	p := plan(testConfig, []string{"lib", "login", "ui", "wm"})
	want := []Matrix{
		{ID: "build-lib-ui-x86-64", Name: "lib ui (x86_64)", Runner: "ubuntu-latest", Run: "just build-stage lib ui"},
		{ID: "build-lib-ui-aarch64", Name: "lib ui (aarch64)", Runner: "ubuntu-24.04-arm", Run: "just build-stage lib ui"},
	}
	if !slices.Equal(p.Builds, want) {
		t.Fatalf("builds\n%+v\nwant\n%+v", p.Builds, want)
	}
	lanes := make([]string, len(p.Lanes))
	for i, l := range p.Lanes {
		lanes[i] = l.Build.ID
	}
	if !slices.Equal(lanes, []string{"build-login-x86-64", "build-wm-x86-64"}) {
		t.Fatalf("lane builds %v, want login's and wm's", lanes)
	}
}

func TestAComponentWithNothingToBuildHasNoBuild(t *testing.T) {
	p := plan(testConfig, []string{"polkit", "repository"})
	if len(p.Builds) != 0 || len(p.Lanes) != 0 {
		t.Fatalf("polkit and repository have no Go suites or VM checks, yet the plan builds %+v and %+v", p.Builds, p.Lanes)
	}
	if len(p.Jobs) != 1 || p.Jobs[0].Run != "just tidy-check" {
		t.Fatalf("jobs %+v, want only the repository's", p.Jobs)
	}
}

func TestAPlanWithoutJobsBuildsNothing(t *testing.T) {
	if p := plan(testConfig, []string{"polkit"}); len(p.Builds) != 0 || len(p.Jobs) != 0 {
		t.Fatalf("polkit has no jobs, yet the plan builds %+v and runs %+v", p.Builds, p.Jobs)
	}
}

func TestAnEmptyPlanIsEmptyListsNotNull(t *testing.T) {
	p := plan(testConfig, nil)
	if p.Jobs == nil || p.Packages == nil || p.Builds == nil || p.Shells == nil || p.Lanes == nil {
		t.Fatalf("an empty plan has nil lists, which GitHub reads as null: %+v", p)
	}
}

func wantErrors(t *testing.T, err error, wants []string) {
	t.Helper()
	if err == nil {
		t.Fatal("validate accepted the configuration")
	}
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestValidateRefusesUnknownNames(t *testing.T) {
	bad := map[string]Component{
		"wm": {Depends: []string{"wn"}, VMs: [][]string{{"login-vm"}, {}}, Jobs: []Job{
			{Name: "x", Runners: []Runner{"riscv"}, Run: "true"},
			{Name: "", Runners: []Runner{X86_64}, Run: "true"},
			{Name: "idle", Runners: []Runner{X86_64}},
			{Name: "nowhere", Run: "true"},
		}},
		"shell": {Shards: [][]ShardEntry{packages("./bar", "./..."), {}, packages("bar", "./bar")}, Race: true},
		"lib":   {Shards: [][]ShardEntry{packages("./xkb")}},
	}
	wantErrors(t, validate(Config{Components: bad, ShardedGates: []string{"test", "test-lysdebug"}}), []string{
		`"wn"`, `"riscv"`, "without a name", "wm / idle has no command", "wm / nowhere runs on no runner", `"login-vm", which is not wm's`,
		"wm's VM job 2 runs no VM check",
		`shell shard 1 names "./...", which is no package directory`, "shell shard 2 names no packages",
		`shell shard 3 names "bar", which is no package directory`, `shell names "./bar" in two shards`, "lib has shards but neither a test or test-lysdebug gate nor a race suite",
	})
}

func TestValidateRefusesGoJobsThatDoNotRunEverySuiteExactlyOnce(t *testing.T) {
	cfg := Config{
		ShardedGates: []string{"test"},
		Components: map[string]Component{
			"a": {Gates: []string{"test"}, Race: true, Shards: [][]ShardEntry{packages("./x")}},
			"b": {Gates: []string{"vet"}},
			"c": {Race: true},
			"d": {Gates: []string{"test"}, Shards: [][]ShardEntry{packages("./y")}},
		},
		GoJobs: map[Runner][][]Target{
			X86_64:  {{target("a/1"), target("a")}, {target("b:race")}, {target("nope")}, {target("b/2")}, {}, {target("d")}},
			AArch64: {{target("a/2:gates")}, {target("a:race")}, {target("c")}, {target("d")}},
			"riscv": {{target("b")}},
		},
	}
	wantErrors(t, validate(cfg), []string{
		"a's gates suite runs on x86_64 as a/1, a, not once whole or once in each of its parts 1 to 2",
		"go-jobs on x86_64 names b:race, but b runs no such suite on x86_64",
		"go-jobs on x86_64 names nope, which is no component",
		"go-jobs on x86_64 names b/2, but b has no parts",
		"go-jobs job 5 on x86_64 names no target",
		"b's gates suite runs in no Go job on x86_64",
		"c's race suite runs in no Go job on x86_64",
		"a's gates suite runs on aarch64 as a/2:gates, not once whole or once in each of its parts 1 to 2",
		"go-jobs on aarch64 names a:race, but a runs no such suite on aarch64",
		"go-jobs on aarch64 names c, but c runs no such suite on aarch64",
		"b's gates suite runs in no Go job on aarch64",
		`go-jobs runs on "riscv", which is no runner`,
		"d has shards, but no Go job runs one of its parts",
	})
}

func TestValidateAcceptsAComponentRunWholeOnOneSuiteAndInPartsOnAnother(t *testing.T) {
	cfg := Config{
		ShardedGates: []string{"test"},
		Components:   map[string]Component{"a": {Gates: []string{"test"}, Race: true, Shards: [][]ShardEntry{packages("./x")}}},
		GoJobs: map[Runner][][]Target{
			X86_64:  {{target("a:gates"), target("a/1:race")}, {target("a/2:race")}},
			AArch64: {{target("a/1")}, {target("a/2")}},
		},
	}
	if err := validate(cfg); err != nil {
		t.Fatalf("validate refused a well-formed split: %v", err)
	}
}

var staticConfig = Config{
	ShardedGates: []string{"test"},
	StaticGates:  []string{"vet", "lint"},
	GateRunners:  map[Runner][]string{AArch64: {"vet", "test", "build"}},
	Components: map[string]Component{
		"a": {Gates: []string{"vet", "lint", "test", "build"}, Race: true, Shards: [][]ShardEntry{packages("./x")}},
		"b": {Gates: []string{"test", "build"}, Race: true, Shards: [][]ShardEntry{packages("./y")}},
		"c": {Gates: []string{"vet", "lint", "test"}, Race: true},
	},
	GoJobs: map[Runner][][]Target{
		X86_64:  {{target("a/1"), target("a/2")}, {target("a/3")}, {target("b/1"), target("b/2")}, {target("c")}},
		AArch64: {{target("a/1"), target("a/2")}, {target("a/3")}, {target("b"), target("c")}},
	},
}

func TestTheStaticGatesOfAComponentWithShardsRunInAPartOfTheirOwn(t *testing.T) {
	if err := validate(staticConfig); err != nil {
		t.Fatalf("validate refused static gates in a part of their own: %v", err)
	}
	a := staticConfig.Components["a"]
	for _, tc := range []struct {
		s    Suite
		r    Runner
		want int
	}{{GoGates, X86_64, 3}, {Race, X86_64, 2}, {GoGates, AArch64, 3}} {
		if got := staticConfig.parts(a, tc.s, tc.r); got != tc.want {
			t.Errorf("a's %s suite on %s has %d parts, want %d", tc.s, tc.r, got, tc.want)
		}
	}
	var runs []string
	for _, m := range plan(staticConfig, []string{"a", "b", "c"}).Jobs {
		runs = append(runs, m.Name+": "+m.Run)
	}
	want := []string{
		"Go a/1 a/2 (x86_64): just gates a/1 a/2",
		"Go a/3 (x86_64): just gates a/3",
		"Go b/1 b/2 (x86_64): just gates b/1 b/2",
		"Go c (x86_64): just gates c",
		"Go a/1:gates a/2:gates (aarch64): just gates a/1:gates a/2:gates",
		"Go a/3:gates (aarch64): just gates a/3:gates",
		"Go b:gates c:gates (aarch64): just gates b:gates c:gates",
	}
	if !slices.Equal(runs, want) {
		t.Fatalf("jobs\n%s\nwant\n%s", strings.Join(runs, "\n"), strings.Join(want, "\n"))
	}
}

func TestAComponentWithoutStaticGatesOrWithoutShardsHasNoStaticPart(t *testing.T) {
	for _, tc := range []struct {
		name string
		r    Runner
		want int
	}{{"b", X86_64, 2}, {"c", X86_64, 0}} {
		if got := staticConfig.parts(staticConfig.Components[tc.name], GoGates, tc.r); got != tc.want {
			t.Errorf("%s's gates suite on %s has %d parts, want %d", tc.name, tc.r, got, tc.want)
		}
	}
	cfg := staticConfig
	cfg.GateRunners = map[Runner][]string{AArch64: {"lint", "test", "build"}}
	cfg.StaticGates = []string{"vet"}
	if got := cfg.parts(cfg.Components["a"], GoGates, AArch64); got != 2 {
		t.Errorf("a runs no static gate on aarch64, yet its gates suite has %d parts there, want 2", got)
	}
}

func TestValidateRefusesAStaticPartTheComponentLacksOrGoJobsThatSkipIt(t *testing.T) {
	cfg := staticConfig
	cfg.GoJobs = map[Runner][][]Target{
		X86_64:  {{target("a/1"), target("a/2"), target("a/3:race")}, {target("b/1"), target("b/2"), target("b/3")}, {target("c/2")}, {target("c")}},
		AArch64: {{target("a/1"), target("a/2")}, {target("b"), target("c")}},
	}
	wantErrors(t, validate(cfg), []string{
		"go-jobs on x86_64 names a/3:race, but a's race suite has parts 1 to 2 on x86_64",
		"a's gates suite runs on x86_64 as a/1, a/2, not once whole or once in each of its parts 1 to 3",
		"go-jobs on x86_64 names b/3, but b's gates suite has parts 1 to 2 on x86_64 and b's race suite has parts 1 to 2 on x86_64",
		"go-jobs on x86_64 names c/2, but c has no parts",
		"a's gates suite runs on aarch64 as a/1, a/2, not once whole or once in each of its parts 1 to 3",
	})
}

func TestValidateRefusesAStaticGateNoComponentRunsOrOneShardsSplit(t *testing.T) {
	cfg := staticConfig
	cfg.StaticGates = []string{"vet", "lnit", "test"}
	wantErrors(t, validate(cfg), []string{
		`static-gates names "lnit", which is no component's gate`,
		`static-gates names "test", which sharded-gates splits by package`,
	})
}

func TestShardsNeedAGateListedToSplit(t *testing.T) {
	cfg := Config{Components: map[string]Component{"a": {Gates: []string{"test"}, Shards: [][]ShardEntry{packages("./x")}}}}
	wantErrors(t, errors.Join(validateShards("a", cfg.Components["a"], nil)...), []string{"a has shards, but sharded-gates names no gate a shard splits"})
	if errs := validateShards("a", cfg.Components["a"], []string{"vet", "test"}); len(errs) != 0 {
		t.Errorf("a test gate split by a sharded-gates naming test was refused: %v", errs)
	}
}

func TestValidateAcceptsWellFormedJobs(t *testing.T) {
	if err := validate(testConfig); err != nil {
		t.Fatalf("validate refused the test configuration: %v", err)
	}
}

func TestATargetNamesAComponentAndOptionallyAPartAndASuite(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Target
	}{
		{"wm", Target{Component: "wm"}},
		{"wm/12", Target{Component: "wm", Part: 12}},
		{"wm:race", Target{Component: "wm", Suite: Race}},
		{"wm/2:gates", Target{Component: "wm", Part: 2, Suite: GoGates}},
	} {
		got, err := parseTarget(tc.in)
		if err != nil || got != tc.want || got.String() != tc.in {
			t.Errorf("parseTarget(%q) = %+v, %v; want %+v printing as itself", tc.in, got, err, tc.want)
		}
	}
	for _, tc := range []struct{ in, want string }{
		{":race", "names no component"},
		{"wm/0", `part "0"`},
		{"wm/02", `part "02"`},
		{"wm/x", `part "x"`},
		{"wm:lint", `the suite "lint"`},
	} {
		if _, err := parseTarget(tc.in); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("parseTarget(%q) = %v, want an error naming %s", tc.in, err, tc.want)
		}
	}
}

func TestGoJobsDecodeTheirTargetsWhenTheConfigurationLoads(t *testing.T) {
	var cfg Config
	if err := json.Unmarshal([]byte(`{"go-jobs": {"x86_64": [["wm/2:race", "lib"]]}}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.GoJobs[X86_64], [][]Target{{{Component: "wm", Part: 2, Suite: Race}, {Component: "lib"}}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("go-jobs %+v, want %+v", got, want)
	}
	for _, bad := range []string{`{"go-jobs": {"x86_64": [[3]]}}`, `{"go-jobs": {"x86_64": [["wm:lint"]]}}`} {
		if err := json.Unmarshal([]byte(bad), &cfg); err == nil {
			t.Errorf("%s decoded", bad)
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
	cfg, err := load(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	components := cfg.Components
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

func TestTheRecipeBodyIsTheIndentedLinesUnderItsHeader(t *testing.T) {
	justfile := "test:\n    go test ./...\n\n    echo done\n# race\nrace:\n    go test -race $X\nbuild:\n    go build\n"
	if got, want := recipeBody(justfile, "race"), "    go test -race $X\n"; got != want {
		t.Fatalf("race body %q, want %q", got, want)
	}
	if got, want := recipeBody(justfile, "test"), "    go test ./...\n\n    echo done\n"; got != want {
		t.Fatalf("test body %q, want %q", got, want)
	}
}

func TestEveryGateIsARecipeOfItsComponent(t *testing.T) {
	root := writeTree(t, map[string]string{
		"a/justfile": "lint:\n    golangci-lint run\ntest *args:\n    go test ./...\nrace:\n    go test -race ./...\n",
		"b/justfile": "lint:\n    golangci-lint run\n",
	})
	cfg := Config{Components: map[string]Component{
		"a": {Gates: []string{"lint", "test"}, Race: true},
		"b": {Gates: []string{"lint"}},
		"c": {},
	}}
	if err := checkTree(root, cfg); err != nil {
		t.Fatalf("checkTree refused gates every justfile has: %v", err)
	}
	cfg.Components["b"] = Component{Gates: []string{"lint", "vet"}, Race: true}
	cfg.Components["d"] = Component{Gates: []string{"test"}}
	wantErrors(t, checkTree(root, cfg), []string{
		`ci/components.json runs b's gate "vet", which b/justfile has no recipe for`,
		`ci/components.json runs b's gate "race", which b/justfile has no recipe for`,
		"d runs gates, but its justfile does not read",
	})
}

func TestTheRecipeMatcherFindsRecipesAndSkipsAssignments(t *testing.T) {
	justfile := "set shell := [\"bash\", \"-c\"]\ntest_dir := \"/tmp/a: b\"\ntest *args:\n    go test\nrace:\n    go test -race\n[private]\nwrite-baselines args=\"\":\ninitramfs-tools image=\"ubuntu:24.04\":\n"
	if got, want := recipesOf(justfile), []string{"test", "race", "write-baselines", "initramfs-tools"}; !slices.Equal(got, want) {
		t.Fatalf("recipes %v, want %v", got, want)
	}
}

func TestAShardEntryIsAPackageOrSomeOfItsTestsByName(t *testing.T) {
	var shards [][]ShardEntry
	if err := json.Unmarshal([]byte(`[["./a", {"package": "./b", "tests": ["TestX", "TestY"]}]]`), &shards); err != nil {
		t.Fatal(err)
	}
	want := [][]ShardEntry{{{Package: "./a"}, {Package: "./b", Tests: []string{"TestX", "TestY"}}}}
	if !reflect.DeepEqual(shards, want) {
		t.Fatalf("shards %+v, want %+v", shards, want)
	}
	for _, tc := range []struct{ in, want string }{
		{`[[{"package": "./b"}]]`, "selects no test of ./b"},
		{`[[{"package": "./b", "tests": []}]]`, "selects no test of ./b"},
		{`[[{"package": "./b", "test": ["TestX"]}]]`, `unknown field "test"`},
		{`[[3]]`, "a shard entry is a package directory"},
	} {
		if err := json.Unmarshal([]byte(tc.in), &shards); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s decoded to %v, want an error naming %s", tc.in, err, tc.want)
		}
	}
}

func selection(pkg string, tests ...string) ShardEntry {
	return ShardEntry{Package: pkg, Tests: tests}
}

func TestValidateAcceptsAPackageSplitByTestNameOverShards(t *testing.T) {
	c := Component{Race: true, Shards: [][]ShardEntry{
		{selection("./x", "TestA", "TestB"), {Package: "./y/..."}},
		{selection("./x", "TestC"), selection("./z", "TestA", "Test")},
	}}
	if errs := validateShards("a", c, []string{"test"}); len(errs) != 0 {
		t.Fatalf("validate refused a package split by test name: %v", errs)
	}
	if got := c.selectedPackages(); !slices.Equal(got, []string{"./x", "./z"}) {
		t.Errorf("selected packages %v, want ./x and ./z", got)
	}
}

func TestValidateRefusesATestSelectionThatIsNotExactOrNotOnce(t *testing.T) {
	c := Component{Race: true, Shards: [][]ShardEntry{
		{selection("./x", "TestA", "^TestB$", "Testa", "Test-B", "TestA"), selection("./t/...", "TestA")},
		{selection("./x", "TestC"), selection("./x", "TestD"), selection("./w/v", "TestA")},
		{{Package: "./w/..."}, {Package: "./t/..."}},
	}}
	wantErrors(t, errors.Join(validateShards("a", c, []string{"test"})...), []string{
		`a shard 1 selects "^TestB$" of ./x, which is no test function's name`,
		`a shard 1 selects "Testa" of ./x, which is no test function's name`,
		`a shard 1 selects "Test-B" of ./x, which is no test function's name`,
		"a selects ./x's TestA twice",
		`a shard 1 selects tests of "./t/...", which is no single package`,
		`a shard 2 selects tests of "./x" twice`,
		"a tests ./w/v whole as ./w/... in one shard and by test name in another",
		"a tests ./t/... whole as ./t/... in one shard and by test name in another",
	})
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for path, body := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const splitJustfile = "test:\n    go test -run \"${LYS_TEST_RUN-}\" -skip \"${LYS_TEST_SKIP-}\" ${LYS_TEST_PACKAGES:-./...}\n"

func TestTheTreeHoldsEveryTestAShardSelects(t *testing.T) {
	root := writeTree(t, map[string]string{
		"a/justfile":           splitJustfile,
		"a/x/x_test.go":        "package x\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n\nfunc TestB(t *testing.T) {}\n\nfunc helper() {}\n\ntype s struct{}\n\nfunc (s) TestC(t *testing.T) {}\n",
		"a/x/y_test.go":        "package x_test\n\nimport \"testing\"\n\nfunc TestD(t *testing.T) {}\n",
		"a/y/y.go":             "package y\n",
		"a/z/deep/z_test.go":   "package deep\n",
		"a/empty/doc.go":       "package empty\n",
		"a/broken/bad_test.go": "package broken\nfunc (",
	})
	cfg := Config{ShardedGates: []string{"test"}, Components: map[string]Component{"a": {Gates: []string{"test"}, Shards: [][]ShardEntry{
		{selection("./x", "TestA", "TestD"), {Package: "./z/..."}},
		{selection("./x", "TestB")},
	}}}}
	if err := checkTree(root, cfg); err != nil {
		t.Fatalf("checkTree refused shards whose tests exist: %v", err)
	}
	cfg.Components["a"] = Component{Gates: []string{"test"}, Shards: [][]ShardEntry{
		{selection("./x", "TestA", "TestC", "TestE"), {Package: "./y"}},
		{selection("./empty", "TestA"), selection("./broken", "TestA"), {Package: "./missing/..."}},
	}}
	wantErrors(t, checkTree(root, cfg), []string{
		"a shard 1 selects TestC of ./x, which no test file there declares",
		"a shard 1 selects TestE of ./x, which no test file there declares",
		"a shard 1 names ./y, which holds no test",
		"a shard 2 selects tests of ./empty: " + filepath.Join(root, "a", "empty") + " holds no test file",
		"a shard 2 selects tests of ./broken: parse " + filepath.Join(root, "a", "broken", "bad_test.go"),
		"a shard 2 names ./missing/...: ",
	})
}

func TestEveryShardedRecipeReadsTheShardVariables(t *testing.T) {
	whole := Component{Gates: []string{"test", "vet"}, Race: true, Shards: [][]ShardEntry{packages("./x")}}
	split := Component{Gates: []string{"test", "vet"}, Race: true, Shards: [][]ShardEntry{{selection("./x", "TestA")}}}
	packagesOnly := "test:\n    go test ${LYS_TEST_PACKAGES:-./...}\nrace:\n    go test -race ${LYS_TEST_PACKAGES:-./...}\nvet:\n    go vet ./...\n"
	both := strings.ReplaceAll(packagesOnly, "go test", "go test -run \"${LYS_TEST_RUN-}\" -skip \"${LYS_TEST_SKIP-}\"")
	root := writeTree(t, map[string]string{"packages/justfile": packagesOnly, "both/justfile": both, "whole/justfile": "test:\n    go test ./...\nrace:\n    go test -race -run \"$LYS_TEST_RUN\" ${LYS_TEST_PACKAGES}\n"})
	for _, tc := range []struct {
		dir string
		c   Component
	}{{"packages", whole}, {"both", whole}, {"both", split}} {
		if errs := checkShardedRecipes(filepath.Join(root, tc.dir, "justfile"), "a", tc.c, []string{"test"}); len(errs) != 0 {
			t.Errorf("%s/justfile refused for %+v: %v", tc.dir, tc.c.Shards, errs)
		}
	}
	wantErrors(t, errors.Join(checkShardedRecipes(filepath.Join(root, "packages", "justfile"), "a", split, []string{"test"})...), []string{
		"a/justfile's test recipe does not read $LYS_TEST_RUN, so a's shards would not split it",
		"a/justfile's test recipe does not read $LYS_TEST_SKIP",
		"a/justfile's race recipe does not read $LYS_TEST_RUN",
		"a/justfile's race recipe does not read $LYS_TEST_SKIP",
	})
	wantErrors(t, errors.Join(checkShardedRecipes(filepath.Join(root, "whole", "justfile"), "a", split, []string{"test"})...), []string{
		"a/justfile's test recipe does not read $LYS_TEST_PACKAGES",
		"a/justfile's test recipe tests ./... rather than $LYS_TEST_PACKAGES",
		"a/justfile's race recipe does not read $LYS_TEST_SKIP",
	})
	wantErrors(t, errors.Join(checkShardedRecipes(filepath.Join(root, "nowhere", "justfile"), "a", split, nil)...), []string{"a has shards, but its justfile does not read"})
}
