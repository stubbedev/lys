package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

type unitScope string

const (
	scopeWhole    unitScope = "whole"
	scopePackages unitScope = "packages"
	scopeRest     unitScope = "rest"
	scopeRun      unitScope = "run"
	scopeSkip     unitScope = "skip"
)

// gateUnit is one process the gates recipe starts: recipe of target's
// component and part, over the packages or tests its scope selects.
type gateUnit struct {
	target  Target
	recipe  string
	scope   unitScope
	pattern string
	list    []string
}

// String is the line the gates recipe reads: the target, the recipe,
// the scope, the -run or -skip pattern and the packages, "-" standing
// for an empty pattern or list.
func (u gateUnit) String() string {
	pattern, list := u.pattern, strings.Join(u.list, " ")
	if pattern == "" {
		pattern = "-"
	}
	if list == "" {
		list = "-"
	}
	return strings.Join([]string{u.target.String(), u.recipe, string(u.scope), pattern, list}, " ")
}

// gateUnits is every process the gates recipe starts on r for specs. A
// target without a part on a component with shards runs as every part
// of it, as CI's Go jobs split it, so a local run of a component tests
// no package in one process that CI tests in several.
func (cfg Config) gateUnits(specs []Target, r Runner) ([]gateUnit, error) {
	var units []gateUnit
	seen := map[string]bool{}
	for _, spec := range specs {
		c, ok := cfg.Components[spec.Component]
		if !ok {
			return nil, fmt.Errorf("%s is no component of ci/components.json", spec.Component)
		}
		n := max(cfg.parts(c, GoGates, r), cfg.parts(c, Race, r))
		if spec.Part > 0 && len(c.Shards) == 0 {
			return nil, fmt.Errorf("%s is no part of %s, which has no shards", spec, spec.Component)
		}
		if spec.Part > n {
			return nil, fmt.Errorf("%s is no part of %s, which has parts 1 to %d on %s", spec, spec.Component, n, r)
		}
		suites := []Suite{GoGates, Race}
		if spec.Suite != "" {
			suites = []Suite{spec.Suite}
		}
		parts := []int{spec.Part}
		if spec.Part == 0 && n > 0 {
			parts = make([]int, n)
			for i := range parts {
				parts[i] = i + 1
			}
		}
		count := len(units)
		for _, part := range parts {
			for _, s := range suites {
				for _, recipe := range cfg.partRecipes(c, s, r, part) {
					t := Target{Component: spec.Component, Part: part}
					task := t.String() + " " + recipe
					if seen[task] {
						return nil, fmt.Errorf("%s runs %s a second time", spec, task)
					}
					seen[task] = true
					units = append(units, cfg.split(c, t, recipe)...)
				}
			}
		}
		if len(units) == count {
			names := make([]string, len(suites))
			for i, s := range suites {
				names[i] = string(s)
			}
			return nil, fmt.Errorf("%s runs nothing, since %s has no %s to run on %s", spec, spec.Component, strings.Join(names, " "), r)
		}
	}
	return units, nil
}

// partRecipes is the recipes of c's suite s that part runs on r, part 0
// being the whole suite.
func (cfg Config) partRecipes(c Component, s Suite, r Runner, part int) []string {
	if s == Race {
		if c.Race && part <= cfg.parts(c, Race, r) {
			return []string{"race"}
		}
		return nil
	}
	if part == 0 {
		return cfg.gates(c, r)
	}
	return cfg.partGates(c, r, part)
}

// split is the processes recipe of part t of c runs in: one for the
// whole recipe, unless t is a part and the recipe one shards split,
// whose part k tests shard k's packages and each package it selects
// tests of by name with -run, and whose part after the shards tests
// every package no shard names, which the gates recipe lists, and each
// package a shard selects tests of with every selected test skipped.
func (cfg Config) split(c Component, t Target, recipe string) []gateUnit {
	whole := gateUnit{target: t, recipe: recipe, scope: scopeWhole}
	if t.Part == 0 || recipe != "race" && !slices.Contains(cfg.ShardedGates, recipe) {
		return []gateUnit{whole}
	}
	if t.Part <= len(c.Shards) {
		var units []gateUnit
		var pkgs []string
		for _, e := range c.Shards[t.Part-1] {
			if len(e.Tests) == 0 {
				pkgs = append(pkgs, e.Package)
				continue
			}
			units = append(units, gateUnit{target: t, recipe: recipe, scope: scopeRun, pattern: testsPattern(e.Tests), list: []string{e.Package}})
		}
		if len(pkgs) > 0 {
			units = append([]gateUnit{{target: t, recipe: recipe, scope: scopePackages, list: pkgs}}, units...)
		}
		return units
	}
	var named []string
	selected := map[string][]string{}
	for _, shard := range c.Shards {
		for _, e := range shard {
			named = append(named, e.Package)
			if len(e.Tests) > 0 {
				selected[e.Package] = append(selected[e.Package], e.Tests...)
			}
		}
	}
	slices.Sort(named)
	units := []gateUnit{{target: t, recipe: recipe, scope: scopeRest, list: slices.Compact(named)}}
	for _, pkg := range slices.Sorted(maps.Keys(selected)) {
		units = append(units, gateUnit{target: t, recipe: recipe, scope: scopeSkip, pattern: testsPattern(selected[pkg]), list: []string{pkg}})
	}
	return units
}

func testsPattern(tests []string) string {
	return "^(" + strings.Join(tests, "|") + ")$"
}
