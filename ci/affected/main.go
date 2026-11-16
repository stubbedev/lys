package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/stubbedev/lys/lib/srcroot"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "affected:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("affected", flag.ContinueOnError)
	base := flags.String("base", "", "the commit the change is measured from")
	all := flags.Bool("all", false, "plan every component")
	github := flags.Bool("github", false, "write the plan to $GITHUB_OUTPUT")
	if err := flags.Parse(args); err != nil {
		return err
	}
	root, module, err := srcroot.Module(".")
	if err != nil {
		return err
	}
	components, err := load(root + "/ci/components.json")
	if err != nil {
		return err
	}
	names, err := choose(root, module, components, *base, *all)
	if err != nil {
		return err
	}
	p := plan(components, names)
	if *github {
		return writeGitHub(p)
	}
	return writeText(stdout, p)
}

func load(path string) (map[string]Component, error) {
	data, err := os.ReadFile(path) //nolint:gosec // ci/components.json under the module root
	if err != nil {
		return nil, fmt.Errorf("read the components: %w", err)
	}
	var components map[string]Component
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&components); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := validate(components); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return components, nil
}

func choose(root, module string, components map[string]Component, base string, all bool) ([]string, error) {
	if all || base == "" || strings.Trim(base, "0") == "" {
		return affected(components, nil, []string{"go.mod"}, nil), nil
	}
	changed, err := gitLines(root, "diff", "--name-only", "--no-renames", base+"...HEAD")
	if err != nil {
		return nil, err
	}
	imports, err := goImports(root, module)
	if err != nil {
		return nil, err
	}
	goPackages, err := goPackagesChangedSince(root, components, base, changed)
	if err != nil {
		return nil, err
	}
	return affected(components, imports, changed, goPackages), nil
}

func goPackagesChangedSince(root string, components map[string]Component, base string, changed []string) ([]string, error) {
	var dirs []string
	var mergeBase string
	for _, name := range slices.Sorted(maps.Keys(components)) {
		path := components[name].GoPackages
		if path == "" || !slices.Contains(changed, path) {
			continue
		}
		if mergeBase == "" {
			out, err := git(root, "merge-base", base, "HEAD")
			if err != nil {
				return nil, err
			}
			mergeBase = strings.TrimSpace(string(out))
		}
		old, err := git(root, "show", mergeBase+":"+path)
		if err != nil {
			return nil, err
		}
		current, err := git(root, "show", "HEAD:"+path)
		if err != nil {
			return nil, err
		}
		changedDirs, err := changedGoPackages(path, old, current)
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, changedDirs...)
	}
	return dirs, nil
}

func gitLines(root string, args ...string) ([]string, error) {
	out, err := git(root, args...)
	return strings.Fields(string(out)), err
}

func git(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...) //nolint:gosec // git reads of a base the workflow names
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, bytes.TrimSpace(stderr.Bytes()))
	}
	return out, nil
}

func goImports(root, module string) (map[string][]string, error) {
	cmd := exec.Command("go", "list", "-test", "-deps=false", "-f", "{{.ImportPath}} {{join .Deps \" \"}}", "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list the module's imports: %w", err)
	}
	return importsOf(module, string(out)), nil
}

func importsOf(module, list string) map[string][]string {
	imports := map[string][]string{}
	for line := range strings.Lines(list) {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		owner := componentOf(module, strings.TrimSuffix(fields[0], ".test"))
		if owner == "" {
			continue
		}
		for _, dep := range fields[1:] {
			if c := componentOf(module, dep); c != "" && c != owner && !slices.Contains(imports[owner], c) {
				imports[owner] = append(imports[owner], c)
			}
		}
	}
	return imports
}

func componentOf(module, importPath string) string {
	rest, ok := strings.CutPrefix(importPath, module+"/")
	if !ok {
		return ""
	}
	name, _, _ := strings.Cut(componentOfDir(rest), " ")
	return name
}

func writeText(w io.Writer, p Plan) error {
	if _, err := fmt.Fprintf(w, "components: %s\npackages: %s\n", strings.Join(p.Components, " "), strings.Join(p.Packages, " ")); err != nil {
		return err
	}
	for _, j := range slices.Concat(p.Builds, p.Jobs) {
		if _, err := fmt.Fprintf(w, "  %s: %s\n", j.Name, j.Run); err != nil {
			return err
		}
	}
	return nil
}

func writeGitHub(p Plan) error {
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		return errors.New("GITHUB_OUTPUT is not set")
	}
	var out bytes.Buffer
	for _, entry := range []struct {
		key string
		v   any
	}{{"components", p.Components}, {"builds", p.Builds}, {"jobs", p.Jobs}, {"packages", p.Packages}} {
		data, err := json.Marshal(entry.v)
		if err != nil {
			return fmt.Errorf("encode %s: %w", entry.key, err)
		}
		fmt.Fprintf(&out, "%s=%s\n", entry.key, data)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0) //nolint:gosec // the file GitHub names for step outputs
	if err != nil {
		return fmt.Errorf("open GITHUB_OUTPUT: %w", err)
	}
	_, err = f.Write(out.Bytes())
	return errors.Join(err, f.Close())
}
