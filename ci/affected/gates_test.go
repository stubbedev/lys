package main

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var gatesConfig = Config{
	ShardedGates: []string{"test"},
	StaticGates:  []string{"vet"},
	GateRunners:  map[Runner][]string{AArch64: {"test", "build"}},
	Components: map[string]Component{
		"a": {Gates: []string{"vet", "test", "build"}, Race: true, Shards: [][]ShardEntry{
			{{Package: "./x"}, selection("./h", "TestA", "TestB")},
			{selection("./h", "TestC")},
		}},
		"b": {Gates: []string{"vet", "test"}, Race: true},
		"c": {Gates: []string{"vet"}},
	},
}

func gateLines(t *testing.T, r Runner, specs ...string) []string {
	t.Helper()
	targets := make([]Target, len(specs))
	for i, s := range specs {
		targets[i] = target(s)
	}
	units, err := gatesConfig.gateUnits(targets, r)
	if err != nil {
		t.Fatalf("gateUnits(%v) on %s: %v", specs, r, err)
	}
	lines := make([]string, len(units))
	for i, u := range units {
		lines[i] = u.String()
	}
	return lines
}

func TestAShardedComponentRunsAsItsPartsAsCISplitsIt(t *testing.T) {
	parts := []string{
		"a/1 test packages - ./x",
		"a/1 test run ^(TestA|TestB)$ ./h",
		"a/1 race packages - ./x",
		"a/1 race run ^(TestA|TestB)$ ./h",
		"a/2 test run ^(TestC)$ ./h",
		"a/2 race run ^(TestC)$ ./h",
		"a/3 test rest - ./h ./x",
		"a/3 test skip ^(TestA|TestB|TestC)$ ./h",
		"a/3 build whole - -",
		"a/3 race rest - ./h ./x",
		"a/3 race skip ^(TestA|TestB|TestC)$ ./h",
		"a/4 vet whole - -",
	}
	if got := gateLines(t, X86_64, "a"); !slices.Equal(got, parts) {
		t.Fatalf("a on x86_64 runs\n%s\nwant its parts\n%s", strings.Join(got, "\n"), strings.Join(parts, "\n"))
	}
	var named []string
	for _, p := range []string{"a/1", "a/2", "a/3", "a/4"} {
		named = append(named, gateLines(t, X86_64, p)...)
	}
	if !slices.Equal(named, parts) {
		t.Errorf("a/1 to a/4 run\n%s\nwhich a alone does not:\n%s", strings.Join(named, "\n"), strings.Join(parts, "\n"))
	}
	race := []string{
		"a/1 race packages - ./x",
		"a/1 race run ^(TestA|TestB)$ ./h",
		"a/2 race run ^(TestC)$ ./h",
		"a/3 race rest - ./h ./x",
		"a/3 race skip ^(TestA|TestB|TestC)$ ./h",
	}
	if got := gateLines(t, X86_64, "a:race"); !slices.Equal(got, race) {
		t.Errorf("a:race runs\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(race, "\n"))
	}
}

func TestAShardedComponentRunsOnlyThePartsWithAGateOnItsRunner(t *testing.T) {
	want := []string{
		"a/1 test packages - ./x",
		"a/1 test run ^(TestA|TestB)$ ./h",
		"a/2 test run ^(TestC)$ ./h",
		"a/3 test rest - ./h ./x",
		"a/3 test skip ^(TestA|TestB|TestC)$ ./h",
		"a/3 build whole - -",
	}
	if got := gateLines(t, AArch64, "a:gates"); !slices.Equal(got, want) {
		t.Errorf("a:gates on aarch64 runs\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestAComponentWithoutShardsRunsEachRecipeWhole(t *testing.T) {
	want := []string{"b vet whole - -", "b test whole - -", "b race whole - -", "c vet whole - -"}
	if got := gateLines(t, X86_64, "b", "c"); !slices.Equal(got, want) {
		t.Errorf("b c runs %q, want %q", got, want)
	}
}

func TestGateTargetsThatRunNothingOrTwiceAreRefused(t *testing.T) {
	for _, tc := range []struct {
		specs []string
		r     Runner
		want  string
	}{
		{[]string{"nope"}, X86_64, "nope is no component of ci/components.json"},
		{[]string{"b/1"}, X86_64, "b/1 is no part of b, which has no shards"},
		{[]string{"a/5"}, X86_64, "a/5 is no part of a, which has parts 1 to 4 on x86_64"},
		{[]string{"a/4"}, AArch64, "a/4 is no part of a, which has parts 1 to 3 on aarch64"},
		{[]string{"a/4:race"}, X86_64, "a/4:race runs nothing, since a has no race to run on x86_64"},
		{[]string{"c:race"}, X86_64, "c:race runs nothing, since c has no race to run on x86_64"},
		{[]string{"c"}, AArch64, "c runs nothing, since c has no gates race to run on aarch64"},
		{[]string{"a", "a/2:race"}, X86_64, "a/2:race runs a/2 race a second time"},
		{[]string{"b:race", "b"}, X86_64, "b runs b race a second time"},
	} {
		targets := make([]Target, len(tc.specs))
		for i, s := range tc.specs {
			targets[i] = target(s)
		}
		units, err := gatesConfig.gateUnits(targets, tc.r)
		if err == nil || err.Error() != tc.want {
			t.Errorf("gateUnits(%v) on %s = %v, %v; want the error %q", tc.specs, tc.r, units, err, tc.want)
		}
	}
}

func TestTheRepositorysShardedComponentsRunAsTheirParts(t *testing.T) {
	cfg, err := load(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := writeGates(&out, cfg, X86_64, []string{"settings:race"}); err != nil {
		t.Fatal(err)
	}
	var targets []string
	for line := range strings.Lines(out.String()) {
		targets = append(targets, strings.Fields(line)[0])
	}
	targets = slices.Compact(targets)
	var want []string
	for _, part := range cfg.running(cfg.Components["settings"], Race, X86_64) {
		want = append(want, Target{Component: "settings", Part: part}.String())
	}
	if len(want) < 2 || !slices.Equal(targets, want) {
		t.Errorf("settings:race runs as %v, want its parts %v", targets, want)
	}
	for _, args := range [][]string{nil, {"settings/0"}} {
		if err := writeGates(&out, cfg, X86_64, args); err == nil {
			t.Errorf("-gates %v planned a run", args)
		}
	}
	if err := writeGates(&out, cfg, "riscv", []string{"settings"}); err == nil {
		t.Error("-gates riscv planned a run")
	}
}
