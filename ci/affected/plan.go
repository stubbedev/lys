package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
)

type Runner string

const (
	X86_64  Runner = "x86_64"
	AArch64 Runner = "aarch64"
)

var runners = []Runner{X86_64, AArch64}

var runnerLabels = map[Runner]string{X86_64: "ubuntu-latest", AArch64: "ubuntu-24.04-arm"}

const vmRunner = X86_64

const shardedGate = "test"

type Job struct {
	Name    string   `json:"name"`
	Runners []Runner `json:"runners"`
	Run     string   `json:"run"`
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
	VMs        []string   `json:"vms"`
	FreeDisk   bool       `json:"free-disk"`
	Shards     [][]string `json:"shards"`
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

type Plan struct {
	Components []string `json:"components"`
	Builds     []Matrix `json:"builds"`
	Jobs       []Matrix `json:"jobs"`
	Packages   []string `json:"packages"`
}

func validate(components map[string]Component) error {
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(components)) {
		c := components[name]
		for _, dep := range c.Depends {
			if _, ok := components[dep]; !ok {
				errs = append(errs, fmt.Errorf("%s depends on %q, which is no component", name, dep))
			}
		}
		for _, j := range c.Jobs {
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
		for _, vm := range c.VMs {
			if !strings.HasPrefix(vm, name+"-") {
				errs = append(errs, fmt.Errorf("%s runs VM check %q, which is not %s's", name, vm, name))
			}
		}
		if c.FreeDisk && len(c.VMs) == 0 {
			errs = append(errs, fmt.Errorf("%s frees disk for VM checks it does not have", name))
		}
		errs = append(errs, validateShards(name, c)...)
	}
	return errors.Join(errs...)
}

func validateShards(name string, c Component) []error {
	if len(c.Shards) == 0 {
		return nil
	}
	var errs []error
	if !slices.Contains(c.Gates, shardedGate) && !c.Race {
		errs = append(errs, fmt.Errorf("%s has shards but neither a %s gate nor a race suite to split", name, shardedGate))
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

func plan(components map[string]Component, names []string) Plan {
	p := Plan{Components: names, Builds: []Matrix{}, Jobs: []Matrix{}, Packages: []string{}}
	for _, name := range names {
		if components[name].Package {
			p.Packages = append(p.Packages, name)
		}
	}
	for _, r := range runners {
		for _, name := range names {
			c := components[name]
			if len(c.Gates) > 0 {
				p.Jobs = append(p.Jobs, gateJobs("go", "Go gates", "go-gates", name, c, slices.Contains(c.Gates, shardedGate), r)...)
			}
			if r == X86_64 && c.Race {
				p.Jobs = append(p.Jobs, gateJobs("race", "race", "race-gates", name, c, true, r)...)
			}
		}
	}
	for _, name := range names {
		c := components[name]
		for _, j := range c.Jobs {
			for _, r := range j.Runners {
				p.Jobs = append(p.Jobs, matrix(name+"-"+j.Name, name+" / "+j.Name, r, j.Run))
			}
		}
		for _, vm := range c.VMs {
			m := matrix(vm, vm, vmRunner, "just vm-checks "+vm)
			m.KVM, m.FreeDisk = true, c.FreeDisk
			p.Jobs = append(p.Jobs, m)
		}
	}
	for _, r := range runners {
		for _, name := range names {
			if b, ok := build(name, components[name], r); ok {
				p.Builds = append(p.Builds, b)
			}
		}
	}
	return p
}

func build(name string, c Component, r Runner) (Matrix, bool) {
	var vms []string
	if r == vmRunner {
		vms = c.VMs
	}
	if len(c.Gates) == 0 && len(vms) == 0 {
		return Matrix{}, false
	}
	m := matrix("build-"+name, name, r, strings.Join(append([]string{"just build-stage", name}, vms...), " "))
	m.FreeDisk = len(vms) > 0
	return m, true
}

func gateJobs(id, title, recipe, name string, c Component, sharded bool, r Runner) []Matrix {
	if !sharded || len(c.Shards) == 0 {
		return []Matrix{userNS(matrix(id+"-"+name, title+" "+name, r, "just "+recipe+" "+name))}
	}
	parts := len(c.Shards) + 1
	jobs := make([]Matrix, 0, parts)
	for part := 1; part <= parts; part++ {
		target := fmt.Sprintf("%s/%d", name, part)
		jobs = append(jobs, userNS(matrix(id+"-"+target, fmt.Sprintf("%s %s %d/%d", title, name, part, parts), r, "just "+recipe+" "+target)))
	}
	return jobs
}

func userNS(m Matrix) Matrix {
	m.UserNS = true
	return m
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
