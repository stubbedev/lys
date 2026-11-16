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
	"repository": {Paths: []string{"go.mod", "nix/"}, GoPackages: "nix/go-packages.json", AffectsAll: true, Jobs: []Job{{Name: "tidy", Runners: []Runner{X86_64}, Run: "just tidy-check"}}},
	"lib":        {Paths: []string{"docs/lib/xkb.md"}, Gates: []string{"vet", "test"}, Race: true},
	"ui":         {Gates: []string{"test"}, Shards: [][]string{{"./widget"}, {"./app", "./anim/..."}}, Jobs: []Job{{Name: "GPU", Runners: []Runner{X86_64}, Run: "just in ui gpu"}}},
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

func TestThePlanRunsEachComponentsGoGatesOnEveryRunnerAndItsRaceSuiteOnce(t *testing.T) {
	p := plan(testComponents, []string{"lib", "polkit", "ui"})
	want := []Matrix{
		{ID: "go-lib-x86-64", Name: "Go gates lib (x86_64)", Runner: "ubuntu-latest", Run: "just go-gates lib", UserNS: true},
		{ID: "race-lib-x86-64", Name: "race lib (x86_64)", Runner: "ubuntu-latest", Run: "just race-gates lib", UserNS: true},
		{ID: "go-ui-1-x86-64", Name: "Go gates ui 1/3 (x86_64)", Runner: "ubuntu-latest", Run: "just go-gates ui/1", UserNS: true},
		{ID: "go-ui-2-x86-64", Name: "Go gates ui 2/3 (x86_64)", Runner: "ubuntu-latest", Run: "just go-gates ui/2", UserNS: true},
		{ID: "go-ui-3-x86-64", Name: "Go gates ui 3/3 (x86_64)", Runner: "ubuntu-latest", Run: "just go-gates ui/3", UserNS: true},
		{ID: "go-lib-aarch64", Name: "Go gates lib (aarch64)", Runner: "ubuntu-24.04-arm", Run: "just go-gates lib", UserNS: true},
		{ID: "go-ui-1-aarch64", Name: "Go gates ui 1/3 (aarch64)", Runner: "ubuntu-24.04-arm", Run: "just go-gates ui/1", UserNS: true},
		{ID: "go-ui-2-aarch64", Name: "Go gates ui 2/3 (aarch64)", Runner: "ubuntu-24.04-arm", Run: "just go-gates ui/2", UserNS: true},
		{ID: "go-ui-3-aarch64", Name: "Go gates ui 3/3 (aarch64)", Runner: "ubuntu-24.04-arm", Run: "just go-gates ui/3", UserNS: true},
		{ID: "ui-gpu-x86-64", Name: "ui / GPU (x86_64)", Runner: "ubuntu-latest", Run: "just in ui gpu"},
	}
	if !slices.Equal(p.Jobs, want) {
		t.Fatalf("jobs\n%+v\nwant\n%+v", p.Jobs, want)
	}
}

func TestShardsSplitOnlyTheTestGateAndTheRaceSuite(t *testing.T) {
	components := map[string]Component{"shell": {Gates: []string{"vet", "lint"}, Race: true, Shards: [][]string{{"./bar"}}}}
	p := plan(components, []string{"shell"})
	want := []Matrix{
		{ID: "go-shell-x86-64", Name: "Go gates shell (x86_64)", Runner: "ubuntu-latest", Run: "just go-gates shell", UserNS: true},
		{ID: "race-shell-1-x86-64", Name: "race shell 1/2 (x86_64)", Runner: "ubuntu-latest", Run: "just race-gates shell/1", UserNS: true},
		{ID: "race-shell-2-x86-64", Name: "race shell 2/2 (x86_64)", Runner: "ubuntu-latest", Run: "just race-gates shell/2", UserNS: true},
		{ID: "go-shell-aarch64", Name: "Go gates shell (aarch64)", Runner: "ubuntu-24.04-arm", Run: "just go-gates shell", UserNS: true},
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
	builds := []Matrix{{ID: "build-wm-x86-64", Name: "wm (x86_64)", Runner: "ubuntu-latest", Run: "just build-stage wm wm-vm wm-vm-apps", FreeDisk: true}}
	if !slices.Equal(p.Builds, builds) {
		t.Fatalf("builds %+v, want %+v", p.Builds, builds)
	}
}

func TestEachGatedComponentBuildsOnEveryRunnerOnItsOwn(t *testing.T) {
	p := plan(testComponents, []string{"lib", "ui"})
	want := []Matrix{
		{ID: "build-lib-x86-64", Name: "lib (x86_64)", Runner: "ubuntu-latest", Run: "just build-stage lib"},
		{ID: "build-ui-x86-64", Name: "ui (x86_64)", Runner: "ubuntu-latest", Run: "just build-stage ui"},
		{ID: "build-lib-aarch64", Name: "lib (aarch64)", Runner: "ubuntu-24.04-arm", Run: "just build-stage lib"},
		{ID: "build-ui-aarch64", Name: "ui (aarch64)", Runner: "ubuntu-24.04-arm", Run: "just build-stage ui"},
	}
	if !slices.Equal(p.Builds, want) {
		t.Fatalf("builds %+v, want %+v", p.Builds, want)
	}
}

func TestAComponentWithNothingToBuildHasNoBuild(t *testing.T) {
	p := plan(testComponents, []string{"login", "polkit", "repository"})
	if len(p.Builds) != 0 {
		t.Fatalf("login, polkit and repository have no gates or VM checks, yet the plan builds %+v", p.Builds)
	}
	if len(p.Jobs) != 1 || p.Jobs[0].Run != "just tidy-check" {
		t.Fatalf("jobs %+v, want only the repository's", p.Jobs)
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
		"ui":    {FreeDisk: true},
		"shell": {Gates: []string{"test"}, Shards: [][]string{{"./bar", "./..."}, {}, {"bar", "./bar"}}},
		"lib":   {Gates: []string{"vet"}, Shards: [][]string{{"./xkb"}}},
	}
	err := validate(bad)
	if err == nil {
		t.Fatal("validate accepted an unknown dependency, runner, an unnamed job and a foreign VM check")
	}
	for _, want := range []string{
		`"wn"`, `"riscv"`, "without a name", "wm / idle has no command", "wm / nowhere runs on no runner", `"login-vm", which is not wm's`, "ui frees disk for VM checks it does not have",
		`shell shard 1 names "./...", which is no package directory`, "shell shard 2 names no packages",
		`shell shard 3 names "bar", which is no package directory`, `shell names "./bar" in two shards`, "lib has shards but neither a test gate nor a race suite",
	} {
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

func TestEveryShardNamesTestedPackagesOfItsComponent(t *testing.T) {
	components, err := load(filepath.Join("..", "components.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range slices.Sorted(maps.Keys(components)) {
		for _, shard := range components[name].Shards {
			for _, pattern := range shard {
				dir := filepath.Join("..", "..", name, strings.TrimSuffix(pattern, "/..."))
				tests, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
				if err != nil {
					t.Fatal(err)
				}
				if strings.HasSuffix(pattern, "/...") {
					tests, err = testFilesUnder(dir)
					if err != nil {
						t.Fatalf("%s shard %s: %v", name, pattern, err)
					}
				}
				if len(tests) == 0 {
					t.Errorf("%s's shard names %s, which holds no test", name, pattern)
				}
			}
		}
	}
}

func testFilesUnder(dir string) ([]string, error) {
	var tests []string
	err := filepath.WalkDir(dir, func(path string, _ os.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(path, "_test.go") {
			tests = append(tests, path)
		}
		return err
	})
	return tests, err
}

func TestEveryShardedRecipeTestsTheShardsPackages(t *testing.T) {
	components, err := load(filepath.Join("..", "components.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range slices.Sorted(maps.Keys(components)) {
		c := components[name]
		if len(c.Shards) == 0 {
			continue
		}
		data, err := os.ReadFile(filepath.Join("..", "..", name, "justfile"))
		if err != nil {
			t.Fatalf("%s has shards, but its justfile does not read: %v", name, err)
		}
		var sharded []string
		if slices.Contains(c.Gates, shardedGate) {
			sharded = append(sharded, shardedGate)
		}
		if c.Race {
			sharded = append(sharded, "race")
		}
		for _, recipe := range sharded {
			body := recipeBody(string(data), recipe)
			if !strings.Contains(body, testPackages) || strings.Contains(body, " ./...") {
				t.Errorf("%s/justfile's %s recipe does not test exactly $%s, so its shards would not split it:\n%s", name, recipe, testPackages, body)
			}
		}
	}
}

func recipeBody(justfile, recipe string) string {
	var body strings.Builder
	in := false
	for line := range strings.Lines(justfile) {
		if m := recipeLine.FindStringSubmatch(line); m != nil && !strings.HasPrefix(line, " ") {
			in = m[1] == recipe
			continue
		}
		if in && line != "\n" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			in = false
		}
		if in {
			body.WriteString(line)
		}
	}
	return body.String()
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

const testPackages = "LYS_TEST_PACKAGES"

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
