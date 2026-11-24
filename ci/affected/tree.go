package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	testPackages = "LYS_TEST_PACKAGES"
	testRun      = "LYS_TEST_RUN"
	testSkip     = "LYS_TEST_SKIP"
)

var recipeLine = regexp.MustCompile(`(?m)^@?([a-z][a-z0-9_-]*)([^\n]*?):(?:[ \t]|$)`)

func checkTree(root string, cfg Config) error {
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(cfg.Components)) {
		c := cfg.Components[name]
		if len(c.Shards) == 0 {
			continue
		}
		errs = append(errs, checkShardPackages(filepath.Join(root, name), name, c)...)
		errs = append(errs, checkShardedRecipes(filepath.Join(root, name, "justfile"), name, c, cfg.ShardedGates)...)
	}
	return errors.Join(errs...)
}

func checkShardPackages(dir, name string, c Component) []error {
	var errs []error
	for i, shard := range c.Shards {
		for _, e := range shard {
			if len(e.Tests) == 0 {
				has, err := holdsTests(filepath.Join(dir, e.Package))
				if err != nil {
					errs = append(errs, fmt.Errorf("%s shard %d names %s: %w", name, i+1, e.Package, err))
				} else if !has {
					errs = append(errs, fmt.Errorf("%s shard %d names %s, which holds no test", name, i+1, e.Package))
				}
				continue
			}
			have, err := testsIn(filepath.Join(dir, e.Package))
			if err != nil {
				errs = append(errs, fmt.Errorf("%s shard %d selects tests of %s: %w", name, i+1, e.Package, err))
				continue
			}
			for _, test := range e.Tests {
				if !have[test] {
					errs = append(errs, fmt.Errorf("%s shard %d selects %s of %s, which no test file there declares", name, i+1, test, e.Package))
				}
			}
		}
	}
	return errs
}

func holdsTests(pattern string) (bool, error) {
	dir, tree := strings.CutSuffix(pattern, "/...")
	if !tree {
		tests, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
		return len(tests) > 0, err
	}
	found := false
	err := filepath.WalkDir(dir, func(path string, _ fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(path, "_test.go") {
			found = true
			return filepath.SkipAll
		}
		return err
	})
	return found, err
}

func testsIn(dir string) (map[string]bool, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s holds no test file", dir)
	}
	tests := map[string]bool{}
	fset := token.NewFileSet()
	for _, path := range files {
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && testName.MatchString(fn.Name.Name) {
				tests[fn.Name.Name] = true
			}
		}
	}
	return tests, nil
}

func checkShardedRecipes(path, name string, c Component, shardedGates []string) []error {
	data, err := os.ReadFile(path) //nolint:gosec // a component's justfile under the module root
	if err != nil {
		return []error{fmt.Errorf("%s has shards, but its justfile does not read: %w", name, err)}
	}
	want := []string{testPackages}
	if len(c.selectedPackages()) > 0 {
		want = append(want, testRun, testSkip)
	}
	var errs []error
	for _, recipe := range shardedRecipes(c, shardedGates) {
		body := recipeBody(string(data), recipe)
		for _, v := range want {
			if !strings.Contains(body, v) {
				errs = append(errs, fmt.Errorf("%s/justfile's %s recipe does not read $%s, so %s's shards would not split it", name, recipe, v, name))
			}
		}
		if strings.Contains(body, " ./...") {
			errs = append(errs, fmt.Errorf("%s/justfile's %s recipe tests ./... rather than $%s, so %s's shards would not split it", name, recipe, testPackages, name))
		}
	}
	return errs
}

func shardedRecipes(c Component, shardedGates []string) []string {
	var recipes []string
	for _, g := range shardedGates {
		if slices.Contains(c.Gates, g) {
			recipes = append(recipes, g)
		}
	}
	if c.Race {
		recipes = append(recipes, "race")
	}
	return recipes
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
