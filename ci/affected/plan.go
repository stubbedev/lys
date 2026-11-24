package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

type Runner string

const (
	X86_64  Runner = "x86_64"
	AArch64 Runner = "aarch64"
)

var runners = []Runner{X86_64, AArch64}

var runnerLabels = map[Runner]string{X86_64: "ubuntu-latest", AArch64: "ubuntu-24.04-arm"}

const (
	vmRunner      = X86_64
	raceRunner    = X86_64
	packageRunner = X86_64
)

type Suite string

const (
	GoGates Suite = "gates"
	Race    Suite = "race"
)

type Job struct {
	Name       string   `json:"name"`
	Runners    []Runner `json:"runners"`
	Run        string   `json:"run"`
	AfterBuild bool     `json:"after-build"`
}

type Component struct {
	Paths      []string   `json:"paths"`
	GoPackages string     `json:"go-packages"`
	Depends    []string   `json:"depends"`
	AffectsAll bool       `json:"affects-all"`
	Package    bool       `json:"package"`
	Gates      []string   `json:"gates"`
	Race       bool       `json:"race"`
	Jobs       []Job      `json:"jobs"`
	VMs        [][]string `json:"vms"`
	FreeDisk   bool       `json:"free-disk"`
	Shards     [][]string `json:"shards"`
}

type Target struct {
	Component string
	Part      int
	Suite     Suite
}

type Config struct {
	Components   map[string]Component  `json:"components"`
	GateRunners  map[Runner][]string   `json:"gate-runners"`
	ShardedGates []string              `json:"sharded-gates"`
	GoJobs       map[Runner][][]Target `json:"go-jobs"`
}

type Matrix struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Runner   string `json:"runner"`
	Run      string `json:"run"`
	KVM      bool   `json:"kvm"`
	UserNS   bool   `json:"userns"`
	FreeDisk bool   `json:"free-disk"`
}

type Lane struct {
	Name  string   `json:"name"`
	Build Matrix   `json:"build"`
	Jobs  []Matrix `json:"jobs"`
}

type Plan struct {
	Components []string `json:"components"`
	Shells     []Matrix `json:"shells"`
	Builds     []Matrix `json:"builds"`
	Jobs       []Matrix `json:"jobs"`
	Lanes      []Lane   `json:"lanes"`
	Packages   []string `json:"packages"`
}

func (t *Target) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("a Go job target is a string like wm/2:race, not %s: %w", data, err)
	}
	parsed, err := parseTarget(s)
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}

func parseTarget(s string) (Target, error) {
	rest, suite, hasSuite := strings.Cut(s, ":")
	name, part, hasPart := strings.Cut(rest, "/")
	if name == "" {
		return Target{}, fmt.Errorf("the Go job target %q names no component", s)
	}
	t := Target{Component: name}
	if hasPart {
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || strconv.Itoa(n) != part {
			return Target{}, fmt.Errorf("the Go job target %q names part %q, which is no number from 1 up", s, part)
		}
		t.Part = n
	}
	if hasSuite {
		switch Suite(suite) {
		case GoGates, Race:
			t.Suite = Suite(suite)
		default:
			return Target{}, fmt.Errorf("the Go job target %q names the suite %q, which is neither %s nor %s", s, suite, GoGates, Race)
		}
	}
	return t, nil
}

func (t Target) String() string {
	s := t.Component
	if t.Part > 0 {
		s += "/" + strconv.Itoa(t.Part)
	}
	if t.Suite != "" {
		s += ":" + string(t.Suite)
	}
	return s
}

func (t Target) on(r Runner) Target {
	if t.Suite == "" && r != raceRunner {
		t.Suite = GoGates
	}
	return t
}

func (cfg Config) gates(c Component, r Runner) []string {
	only, restricted := cfg.GateRunners[r]
	if !restricted {
		return c.Gates
	}
	return slices.DeleteFunc(slices.Clone(c.Gates), func(g string) bool { return !slices.Contains(only, g) })
}

func (cfg Config) suites(c Component, r Runner) []Suite {
	var s []Suite
	if len(cfg.gates(c, r)) > 0 {
		s = append(s, GoGates)
	}
	if c.Race && r == raceRunner {
		s = append(s, Race)
	}
	return s
}

func (c Component) vmChecks(r Runner) []string {
	if r != vmRunner {
		return nil
	}
	return slices.Concat(c.VMs...)
}

func validate(cfg Config) error {
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(cfg.Components)) {
		c := cfg.Components[name]
		for _, dep := range c.Depends {
			if _, ok := cfg.Components[dep]; !ok {
				errs = append(errs, fmt.Errorf("%s depends on %q, which is no component", name, dep))
			}
		}
		for _, j := range c.Jobs {
			if j.AfterBuild && len(c.VMs) == 0 {
				errs = append(errs, fmt.Errorf("%s / %s runs after its component's build, but %s has no VM checks and so no build of its own", name, j.Name, name))
			}
			if j.AfterBuild && slices.ContainsFunc(j.Runners, func(r Runner) bool { return r != vmRunner }) {
				errs = append(errs, fmt.Errorf("%s / %s runs after its component's build, which only %s has, yet runs on %v", name, j.Name, vmRunner, j.Runners))
			}
			if j.Name == "" {
				errs = append(errs, fmt.Errorf("%s has a job without a name", name))
			}
			if j.Run == "" {
				errs = append(errs, fmt.Errorf("%s / %s has no command", name, j.Name))
			}
			if len(j.Runners) == 0 {
				errs = append(errs, fmt.Errorf("%s / %s runs on no runner", name, j.Name))
			}
			for _, r := range j.Runners {
				if _, ok := runnerLabels[r]; !ok {
					errs = append(errs, fmt.Errorf("%s / %s runs on %q, which is no runner", name, j.Name, r))
				}
			}
		}
		for i, group := range c.VMs {
			if len(group) == 0 {
				errs = append(errs, fmt.Errorf("%s's VM job %d runs no VM check", name, i+1))
			}
			for _, vm := range group {
				if !strings.HasPrefix(vm, name+"-") {
					errs = append(errs, fmt.Errorf("%s runs VM check %q, which is not %s's", name, vm, name))
				}
			}
		}
		if c.FreeDisk && len(c.VMs) == 0 {
			errs = append(errs, fmt.Errorf("%s frees disk for VM checks it does not have", name))
		}
		errs = append(errs, validateShards(name, c, cfg.ShardedGates)...)
	}
	errs = append(errs, validateGateRunners(cfg)...)
	errs = append(errs, validateGoJobs(cfg)...)
	return errors.Join(errs...)
}

func validateGateRunners(cfg Config) []error {
	var errs []error
	known := map[string]bool{}
	for _, c := range cfg.Components {
		for _, g := range c.Gates {
			known[g] = true
		}
	}
	for _, r := range slices.Sorted(maps.Keys(cfg.GateRunners)) {
		if _, ok := runnerLabels[r]; !ok {
			errs = append(errs, fmt.Errorf("gate-runners names %q, which is no runner", r))
		}
		if len(cfg.GateRunners[r]) == 0 {
			errs = append(errs, fmt.Errorf("gate-runners lets %s run no gate", r))
		}
		for _, g := range cfg.GateRunners[r] {
			if !known[g] {
				errs = append(errs, fmt.Errorf("gate-runners lets %s run %q, which is no component's gate", r, g))
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(cfg.Components)) {
		for _, g := range cfg.Components[name].Gates {
			if !slices.ContainsFunc(runners, func(r Runner) bool { return slices.Contains(cfg.gates(cfg.Components[name], r), g) }) {
				errs = append(errs, fmt.Errorf("%s's gate %s runs on no runner", name, g))
			}
		}
	}
	return errs
}

func validateShards(name string, c Component, shardedGates []string) []error {
	if len(c.Shards) == 0 {
		return nil
	}
	var errs []error
	if len(shardedGates) == 0 {
		errs = append(errs, fmt.Errorf("%s has shards, but sharded-gates names no gate a shard splits", name))
	} else if !slices.ContainsFunc(c.Gates, func(g string) bool { return slices.Contains(shardedGates, g) }) && !c.Race {
		errs = append(errs, fmt.Errorf("%s has shards but neither a %s gate nor a race suite to split", name, strings.Join(shardedGates, " or ")))
	}
	seen := map[string]bool{}
	for i, shard := range c.Shards {
		if len(shard) == 0 {
			errs = append(errs, fmt.Errorf("%s shard %d names no packages", name, i+1))
		}
		for _, pattern := range shard {
			if !strings.HasPrefix(pattern, "./") || pattern == "./..." {
				errs = append(errs, fmt.Errorf("%s shard %d names %q, which is no package directory under %s/", name, i+1, pattern, name))
			}
			if seen[pattern] {
				errs = append(errs, fmt.Errorf("%s names %q in two shards", name, pattern))
			}
			seen[pattern] = true
		}
	}
	return errs
}

type suiteOf struct {
	component string
	suite     Suite
}

func validateGoJobs(cfg Config) []error {
	var errs []error
	for _, r := range slices.Sorted(maps.Keys(cfg.GoJobs)) {
		if _, ok := runnerLabels[r]; !ok {
			errs = append(errs, fmt.Errorf("go-jobs runs on %q, which is no runner", r))
		}
	}
	split := map[string]bool{}
	for _, r := range runners {
		covered := map[suiteOf][]Target{}
		for i, job := range cfg.GoJobs[r] {
			if len(job) == 0 {
				errs = append(errs, fmt.Errorf("go-jobs job %d on %s names no target", i+1, r))
			}
			for _, t := range job {
				c, ok := cfg.Components[t.Component]
				if !ok {
					errs = append(errs, fmt.Errorf("go-jobs on %s names %s, which is no component", r, t))
					continue
				}
				if t.Part > len(c.Shards)+1 || t.Part > 0 && len(c.Shards) == 0 {
					errs = append(errs, fmt.Errorf("go-jobs on %s names %s, but %s has %s", r, t, t.Component, partsOf(c)))
					continue
				}
				have := cfg.suites(c, r)
				if t.Suite != "" && !slices.Contains(have, t.Suite) || len(have) == 0 {
					errs = append(errs, fmt.Errorf("go-jobs on %s names %s, but %s runs no such suite on %s", r, t, t.Component, r))
					continue
				}
				if t.Part > 0 {
					split[t.Component] = true
				}
				for _, s := range have {
					if t.Suite == "" || t.Suite == s {
						covered[suiteOf{t.Component, s}] = append(covered[suiteOf{t.Component, s}], t)
					}
				}
			}
		}
		for _, name := range slices.Sorted(maps.Keys(cfg.Components)) {
			c := cfg.Components[name]
			for _, s := range cfg.suites(c, r) {
				if err := coverage(r, name, s, c, covered[suiteOf{name, s}]); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(cfg.Components)) {
		if len(cfg.Components[name].Shards) > 0 && !split[name] {
			errs = append(errs, fmt.Errorf("%s has shards, but no Go job runs one of its parts", name))
		}
	}
	return errs
}

func coverage(r Runner, name string, s Suite, c Component, targets []Target) error {
	parts := make([]int, len(targets))
	for i, t := range targets {
		parts[i] = t.Part
	}
	slices.Sort(parts)
	whole := []int{0}
	every := make([]int, len(c.Shards)+1)
	for i := range every {
		every[i] = i + 1
	}
	if slices.Equal(parts, whole) || len(c.Shards) > 0 && slices.Equal(parts, every) {
		return nil
	}
	if len(targets) == 0 {
		return fmt.Errorf("%s's %s suite runs in no Go job on %s", name, s, r)
	}
	names := make([]string, len(targets))
	for i, t := range targets {
		names[i] = t.String()
	}
	return fmt.Errorf("%s's %s suite runs on %s as %s, not once whole or once in each of its %s", name, s, r, strings.Join(names, ", "), partsOf(c))
}

func partsOf(c Component) string {
	if len(c.Shards) == 0 {
		return "no parts"
	}
	return fmt.Sprintf("parts 1 to %d", len(c.Shards)+1)
}

func owns(name string, c Component, file string) bool {
	return strings.HasPrefix(file, name+"/") || slices.ContainsFunc(c.Paths, func(p string) bool { return matches(p, file) })
}

func affected(components map[string]Component, imports map[string][]string, changed, goPackages []string) []string {
	hit := map[string]bool{}
	for _, file := range changed {
		for name, c := range components {
			switch {
			case file == c.GoPackages:
				hit[name] = true
			case owns(name, c, file):
				if c.AffectsAll {
					return slices.Sorted(maps.Keys(components))
				}
				hit[name] = true
			}
		}
	}
	for _, dir := range goPackages {
		name := componentOfDir(dir)
		if _, ok := components[name]; ok {
			hit[name] = true
		}
	}
	for grew := true; grew; {
		grew = false
		for name, c := range components {
			if hit[name] {
				continue
			}
			if slices.ContainsFunc(append(slices.Clone(c.Depends), imports[name]...), func(d string) bool { return hit[d] }) {
				hit[name], grew = true, true
			}
		}
	}
	return slices.Sorted(maps.Keys(hit))
}

func componentOfDir(dir string) string {
	name, _, _ := strings.Cut(dir, "/")
	return name
}

func changedGoPackages(path string, old, current []byte) ([]string, error) {
	var before, after map[string]any
	if err := json.Unmarshal(old, &before); err != nil {
		return nil, fmt.Errorf("parse %s at the base: %w", path, err)
	}
	if err := json.Unmarshal(current, &after); err != nil {
		return nil, fmt.Errorf("parse %s at HEAD: %w", path, err)
	}
	var dirs []string
	for _, dir := range slices.Sorted(maps.Keys(after)) {
		if entry, ok := before[dir]; !ok || !reflect.DeepEqual(entry, after[dir]) {
			dirs = append(dirs, dir)
		}
	}
	for _, dir := range slices.Sorted(maps.Keys(before)) {
		if _, ok := after[dir]; !ok {
			dirs = append(dirs, dir)
		}
	}
	return dirs, nil
}

func matches(path, file string) bool {
	if strings.HasSuffix(path, "/") {
		return strings.HasPrefix(file, path)
	}
	return file == path
}

func plan(cfg Config, names []string) Plan {
	p := Plan{Components: names, Shells: []Matrix{}, Builds: []Matrix{}, Jobs: []Matrix{}, Lanes: []Lane{}, Packages: []string{}}
	for _, name := range names {
		c := cfg.Components[name]
		if c.Package {
			p.Packages = append(p.Packages, name)
		}
		if l, ok := lane(name, c); ok {
			p.Lanes = append(p.Lanes, l)
		}
	}
	for _, name := range names {
		for _, j := range cfg.Components[name].Jobs {
			if j.AfterBuild {
				continue
			}
			for _, r := range j.Runners {
				p.Jobs = append(p.Jobs, componentJob(name, j, r))
			}
		}
	}
	for _, r := range runners {
		for _, job := range cfg.GoJobs[r] {
			if m, ok := goJob(job, names, r); ok {
				p.Jobs = append(p.Jobs, m)
			}
		}
	}
	for _, r := range runners {
		if b, ok := cfg.sharedBuild(names, r); ok {
			p.Builds = append(p.Builds, b)
		}
		if s, ok := cfg.shells(p, names, r); ok {
			p.Shells = append(p.Shells, s)
		}
	}
	return p
}

func lane(name string, c Component) (Lane, bool) {
	vms := c.vmChecks(vmRunner)
	if len(vms) == 0 {
		return Lane{}, false
	}
	b := build([]string{name}, vms, vmRunner)
	b.FreeDisk = c.FreeDisk
	l := Lane{Name: name, Build: b, Jobs: []Matrix{}}
	for _, group := range c.VMs {
		m := matrix(strings.Join(group, " "), strings.Join(group, " + "), vmRunner, "just vm-checks "+strings.Join(group, " "))
		m.KVM = true
		l.Jobs = append(l.Jobs, m)
	}
	for _, j := range c.Jobs {
		if j.AfterBuild {
			l.Jobs = append(l.Jobs, componentJob(name, j, vmRunner))
		}
	}
	return l, true
}

func componentJob(name string, j Job, r Runner) Matrix {
	return matrix(name+"-"+j.Name, name+" / "+j.Name, r, j.Run)
}

func (cfg Config) shells(p Plan, names []string, r Runner) (Matrix, bool) {
	if !slices.ContainsFunc(p.Jobs, func(m Matrix) bool { return m.Runner == runnerLabels[r] }) {
		return Matrix{}, false
	}
	var shells []string
	for _, name := range names {
		c := cfg.Components[name]
		if len(cfg.suites(c, r)) > 0 || c.Package && r == packageRunner {
			shells = append(shells, name)
		}
	}
	if len(shells) == 0 {
		return Matrix{}, false
	}
	return matrix("shells "+strings.Join(shells, " "), "dev shells", r, "just build-shells "+strings.Join(shells, " ")), true
}

func goJob(job []Target, names []string, r Runner) (Matrix, bool) {
	var run []string
	for _, t := range job {
		if slices.Contains(names, t.Component) {
			run = append(run, t.on(r).String())
		}
	}
	if len(run) == 0 {
		return Matrix{}, false
	}
	all := make([]string, len(job))
	for i, t := range job {
		all[i] = t.String()
	}
	m := matrix("go "+strings.Join(all, " "), "Go "+strings.Join(run, " "), r, "just gates "+strings.Join(run, " "))
	m.UserNS = true
	return m, true
}

func (cfg Config) sharedBuild(names []string, r Runner) (Matrix, bool) {
	var shared []string
	for _, name := range names {
		c := cfg.Components[name]
		if len(c.vmChecks(r)) == 0 && len(cfg.suites(c, r)) > 0 {
			shared = append(shared, name)
		}
	}
	if len(shared) == 0 {
		return Matrix{}, false
	}
	return build(shared, nil, r), true
}

func build(components, vms []string, r Runner) Matrix {
	return matrix("build "+strings.Join(components, " "), strings.Join(components, " "), r, "just build-stage "+strings.Join(slices.Concat(components, vms), " "))
}

func matrix(id, name string, r Runner, run string) Matrix {
	return Matrix{
		ID:     slug(id + "-" + string(r)),
		Name:   fmt.Sprintf("%s (%s)", name, r),
		Runner: runnerLabels[r],
		Run:    run,
	}
}

func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}
