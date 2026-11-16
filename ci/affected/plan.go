package main

import (
	"errors"
	"fmt"
	"maps"
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

type Job struct {
	Name    string   `json:"name"`
	Runners []Runner `json:"runners"`
	Run     string   `json:"run"`
}

type Component struct {
	Paths      []string `json:"paths"`
	Depends    []string `json:"depends"`
	AffectsAll bool     `json:"affects-all"`
	Package    bool     `json:"package"`
	Gates      []string `json:"gates"`
	Race       bool     `json:"race"`
	Jobs       []Job    `json:"jobs"`
	VMs        []string `json:"vms"`
	FreeDisk   bool     `json:"free-disk"`
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

type Build struct {
	Arch       Runner `json:"arch"`
	Runner     string `json:"runner"`
	Components string `json:"components"`
	VMs        string `json:"vms"`
}

type Plan struct {
	Components []string `json:"components"`
	Builds     []Build  `json:"builds"`
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
	}
	return errors.Join(errs...)
}

func affected(components map[string]Component, imports map[string][]string, changed []string) []string {
	hit := map[string]bool{}
	for _, file := range changed {
		for name, c := range components {
			if strings.HasPrefix(file, name+"/") || slices.ContainsFunc(c.Paths, func(p string) bool { return matches(p, file) }) {
				if c.AffectsAll {
					return slices.Sorted(maps.Keys(components))
				}
				hit[name] = true
			}
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

func matches(path, file string) bool {
	if strings.HasSuffix(path, "/") {
		return strings.HasPrefix(file, path)
	}
	return file == path
}

func plan(components map[string]Component, names []string) Plan {
	p := Plan{Components: names, Builds: []Build{}, Jobs: []Matrix{}, Packages: []string{}}
	var gated, raced, vms []string
	for _, name := range names {
		c := components[name]
		if c.Package {
			p.Packages = append(p.Packages, name)
		}
		if len(c.Gates) > 0 {
			gated = append(gated, name)
		}
		if c.Race {
			raced = append(raced, name)
		}
	}
	for _, r := range runners {
		if len(gated) > 0 {
			p.Jobs = append(p.Jobs, userNS(matrix("go", "Go gates", r, "just go-gates "+strings.Join(gated, " "))))
		}
		if r == X86_64 && len(raced) > 0 {
			p.Jobs = append(p.Jobs, userNS(matrix("race", "race", r, "just race-gates "+strings.Join(raced, " "))))
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
			vms = append(vms, vm)
		}
	}
	for _, r := range runners {
		packaged := r == X86_64 && len(p.Packages) > 0
		if !packaged && !slices.ContainsFunc(p.Jobs, func(m Matrix) bool { return m.Runner == runnerLabels[r] }) {
			continue
		}
		b := Build{Arch: r, Runner: runnerLabels[r], Components: strings.Join(names, " ")}
		if r == vmRunner {
			b.VMs = strings.Join(vms, " ")
		}
		p.Builds = append(p.Builds, b)
	}
	return p
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
