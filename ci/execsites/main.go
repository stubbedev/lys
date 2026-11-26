package main

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"

	"github.com/stubbedev/lys/lib/srcroot"
)

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "execsites:", err)
		os.Exit(1)
	}
}

func run(args []string, stderr io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: execsites <allowlist>")
	}
	name := args[0]
	data, err := os.ReadFile(name) //nolint:gosec // the allowlist the recipe names
	if err != nil {
		return fmt.Errorf("read the allowlist: %w", err)
	}
	list, err := parseAllowlist(name, data)
	if err != nil {
		return err
	}
	root, module, err := srcroot.Module(".")
	if err != nil {
		return err
	}
	found, err := scanTree(os.DirFS(root), module)
	if err != nil {
		return err
	}
	problems := check(name, list, found)
	for _, p := range problems {
		fmt.Fprintln(stderr, p)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d process starts or entries disagree with %s", len(problems), name)
	}
	return nil
}

type tree struct {
	goSites        []site[goKey]
	packagingSites []site[packagingKey]
}

func scanTree(fsys fs.FS, module string) (tree, error) {
	nix, err := runtimeNix(fsys)
	if err != nil {
		return tree{}, err
	}
	patterns, err := subPackages(nix)
	if err != nil {
		return tree{}, err
	}
	goTree, err := newGoTree(fsys, module)
	if err != nil {
		return tree{}, err
	}
	mains, err := goTree.mains(patterns)
	if err != nil {
		return tree{}, err
	}
	if len(mains) == 0 {
		return tree{}, errors.New("no subPackages of the flake's packages name a main package")
	}
	dirs, err := goTree.closure(mains)
	if err != nil {
		return tree{}, err
	}
	goSites, err := goTree.sites(dirs)
	if err != nil {
		return tree{}, err
	}
	lys := map[string]bool{}
	for _, dir := range mains {
		lys[path.Base(dir)] = true
	}
	pkgSites, err := packagingSites(fsys, nix, lys)
	if err != nil {
		return tree{}, err
	}
	return tree{goSites: goSites, packagingSites: pkgSites}, nil
}

type problem struct {
	at      position
	message string
}

func (p problem) String() string {
	return p.at.String() + ": " + p.message
}

func check(name string, list allowlist, found tree) []problem {
	problems := slices.Concat(
		match(name, "go", list.goEntries, found.goSites),
		match(name, "packaging", list.packagingEntries, found.packagingSites),
	)
	slices.SortFunc(problems, func(a, b problem) int {
		return cmp.Or(cmp.Compare(a.at.file, b.at.file), cmp.Compare(a.at.line, b.at.line), cmp.Compare(a.message, b.message))
	})
	return problems
}

func match[K interface {
	comparable
	fmt.Stringer
}](name, section string, entries []entry[K], sites []site[K]) []problem {
	used := make(map[K]bool, len(entries))
	for _, e := range entries {
		used[e.key] = false
	}
	var problems []problem
	for _, s := range sites {
		if _, ok := used[s.key]; !ok {
			problems = append(problems, problem{s.at, fmt.Sprintf("%s (%v) has no %s entry in %s", s.what, s.key, section, name)})
			continue
		}
		used[s.key] = true
	}
	for _, e := range entries {
		if !used[e.key] {
			problems = append(problems, problem{position{name, e.line}, fmt.Sprintf("the %s entry (%v) matches nothing in the tree: delete it", section, e.key)})
		}
	}
	return problems
}
