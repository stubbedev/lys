package main

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Runner is a GitHub-hosted runner architecture a job runs on.
type Runner string

const (
	X86_64  Runner = "x86_64"
	AArch64 Runner = "aarch64"
)

var runnerLabels = map[Runner]string{X86_64: "ubuntu-latest", AArch64: "ubuntu-24.04-arm"}

// Job is one CI job of a component, run once per runner.
type Job struct {
	Name     string   `json:"name"`
	Runners  []Runner `json:"runners"`
	Run      string   `json:"run"`
	KVM      bool     `json:"kvm"`
	UserNS   bool     `json:"userns"`
	FreeDisk bool     `json:"free-disk"`
	Timeout  int      `json:"timeout"`
}

// Component is one entry of ci/components.json.
type Component struct {
	// Paths outside the component's directory whose change affects it;
	// a path ending in / is a directory.
	Paths []string `json:"paths"`
	// Depends names the components its checks run beyond what its Go code
	// imports: a VM test that boots another component's package.
	Depends []string `json:"depends"`
	// AffectsAll marks the repository's shared files: a change to them
	// affects every component.
	AffectsAll bool  `json:"affects-all"`
	Package    bool  `json:"package"`
	Jobs       []Job `json:"jobs"`
}

// Matrix is one entry of the CI job matrix.
type Matrix struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Runner   string `json:"runner"`
	Run      string `json:"run"`
	KVM      bool   `json:"kvm"`
	UserNS   bool   `json:"userns"`
	FreeDisk bool   `json:"free-disk"`
	Timeout  int    `json:"timeout"`
}

// Plan is what CI runs for a change.
type Plan struct {
	Components []string `json:"components"`
	Jobs       []Matrix `json:"jobs"`
	Packages   []string `json:"packages"`
}

const defaultTimeout = 45

func validate(components map[string]Component) error {
	var errs []error
	for name, c := range components {
		for _, dep := range c.Depends {
			if _, ok := components[dep]; !ok {
				errs = append(errs, fmt.Errorf("%s depends on %q, which is no component", name, dep))
			}
		}
		for _, j := range c.Jobs {
			if j.Name == "" || j.Run == "" || len(j.Runners) == 0 {
				errs = append(errs, fmt.Errorf("%s has a job without a name, a command or a runner", name))
			}
			for _, r := range j.Runners {
				if _, ok := runnerLabels[r]; !ok {
					errs = append(errs, fmt.Errorf("%s / %s runs on %q, which is no runner", name, j.Name, r))
				}
			}
		}
	}
	return errors.Join(errs...)
}

// affected is every component a change to the changed files touches:
// those the files are in or name, and every component depending on one of
// them through its Go imports (imports maps a component to the components
// its packages import) or its declared Depends.
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
	p := Plan{Components: names, Jobs: []Matrix{}, Packages: []string{}}
	for _, name := range names {
		c := components[name]
		if c.Package {
			p.Packages = append(p.Packages, name)
		}
		for _, j := range c.Jobs {
			for _, r := range j.Runners {
				p.Jobs = append(p.Jobs, Matrix{
					ID:       slug(name + "-" + j.Name + "-" + string(r)),
					Name:     fmt.Sprintf("%s / %s (%s)", name, j.Name, r),
					Runner:   runnerLabels[r],
					Run:      j.Run,
					KVM:      j.KVM,
					UserNS:   j.UserNS,
					FreeDisk: j.FreeDisk,
					Timeout:  cmp.Or(j.Timeout, defaultTimeout),
				})
			}
		}
	}
	return p
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
