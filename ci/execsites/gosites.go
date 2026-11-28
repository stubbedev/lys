package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
)

const dynamic = "dynamic"

const componentsFile = "ci/components.json"

var processStarts = map[string]map[string]int{
	"os/exec":               {"Command": 0, "CommandContext": 1, "LookPath": 0},
	"os":                    {"StartProcess": 0},
	"syscall":               {"Exec": 0, "ForkExec": 0, "StartProcess": 0},
	"golang.org/x/sys/unix": {"Exec": 0},
}

const execPackage, cmdType, cmdPathField = "os/exec", "Cmd", "Path"

var targetArchitectures = []string{"amd64", "arm64"}

type position struct {
	file string
	line int
}

func (p position) String() string {
	return fmt.Sprintf("%s:%d", p.file, p.line)
}

type site[K comparable] struct {
	at   position
	what string
	key  K
}

type goPackageGraph map[string]struct {
	Imports []string `json:"imports"`
}

// goPackagesPath is the Go package graph the one component of
// componentsFile that names one names.
func goPackagesPath(fsys fs.FS) (string, error) {
	data, err := fs.ReadFile(fsys, componentsFile)
	if err != nil {
		return "", fmt.Errorf("read the components: %w", err)
	}
	var cfg struct {
		Components map[string]struct {
			GoPackages string `json:"go-packages"`
		} `json:"components"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse %s: %w", componentsFile, err)
	}
	var paths []string
	for _, c := range cfg.Components {
		if c.GoPackages != "" {
			paths = append(paths, c.GoPackages)
		}
	}
	slices.Sort(paths)
	if paths = slices.Compact(paths); len(paths) != 1 {
		return "", fmt.Errorf("%s names %d Go package graphs %q, want one", componentsFile, len(paths), paths)
	}
	return paths[0], nil
}

func readGoPackages(fsys fs.FS) (string, goPackageGraph, error) {
	path, err := goPackagesPath(fsys)
	if err != nil {
		return "", nil, err
	}
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return "", nil, fmt.Errorf("read the Go packages: %w", err)
	}
	var graph goPackageGraph
	if err := json.Unmarshal(data, &graph); err != nil {
		return "", nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return path, graph, nil
}

type goTree struct {
	fsys      fs.FS
	module    string
	graphPath string
	graph     goPackageGraph
	contexts  []build.Context
	fset      *token.FileSet
	packages  map[string]*goPackage
}

type goPackage struct {
	dir    string
	name   string
	files  []*goFile
	consts map[string]constDef
}

type goFile struct {
	path    string
	ast     *ast.File
	imports map[string]string
}

type constDef struct {
	value ast.Expr
	file  *goFile
	pkg   *goPackage
}

func newGoTree(fsys fs.FS, module string) (*goTree, error) {
	graphPath, graph, err := readGoPackages(fsys)
	if err != nil {
		return nil, err
	}
	t := &goTree{fsys: fsys, module: module, graphPath: graphPath, graph: graph, fset: token.NewFileSet(), packages: map[string]*goPackage{}}
	for _, arch := range targetArchitectures {
		ctx := build.Default
		ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled = "linux", arch, false
		ctx.BuildTags, ctx.ToolTags = nil, nil
		ctx.JoinPath = path.Join
		ctx.OpenFile = func(name string) (io.ReadCloser, error) { return fsys.Open(name) }
		t.contexts = append(t.contexts, ctx)
	}
	return t, nil
}

func (t *goTree) mains(patterns []string) ([]string, error) {
	var mains []string
	var errs []error
	for _, pattern := range patterns {
		prefix, isPattern := strings.CutSuffix(pattern, "/...")
		if !isPattern {
			pkg, err := t.load(pattern)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if pkg.name != "main" {
				errs = append(errs, fmt.Errorf("subPackages names %s, which is no main package built for linux", pattern))
				continue
			}
			mains = append(mains, pattern)
			continue
		}
		var matched []string
		for dir := range t.graph {
			if dir == prefix || strings.HasPrefix(dir, prefix+"/") {
				matched = append(matched, dir)
			}
		}
		if len(matched) == 0 {
			errs = append(errs, fmt.Errorf("subPackages names %s, which matches no package of %s", pattern, t.graphPath))
		}
		for _, dir := range matched {
			pkg, err := t.load(dir)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if pkg.name == "main" {
				mains = append(mains, dir)
			}
		}
	}
	slices.Sort(mains)
	return slices.Compact(mains), errors.Join(errs...)
}

func (t *goTree) closure(roots []string) ([]string, error) {
	seen := map[string]bool{}
	queue := slices.Clone(roots)
	for len(queue) > 0 {
		dir := queue[0]
		queue = queue[1:]
		if seen[dir] {
			continue
		}
		p, ok := t.graph[dir]
		if !ok {
			return nil, fmt.Errorf("%s names no package %s: run just go-packages", t.graphPath, dir)
		}
		seen[dir] = true
		queue = append(queue, p.Imports...)
	}
	dirs := make([]string, 0, len(seen))
	for dir := range seen {
		dirs = append(dirs, dir)
	}
	slices.Sort(dirs)
	return dirs, nil
}

func (t *goTree) load(dir string) (*goPackage, error) {
	if pkg, ok := t.packages[dir]; ok {
		return pkg, nil
	}
	if _, ok := t.graph[dir]; !ok {
		return nil, fmt.Errorf("%s names no package %s: run just go-packages", t.graphPath, dir)
	}
	entries, err := fs.ReadDir(t.fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("list the Go package %s: %w", dir, err)
	}
	pkg := &goPackage{dir: dir, consts: map[string]constDef{}}
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		built, err := t.builds(dir, name)
		if err != nil {
			return nil, err
		}
		if !built {
			continue
		}
		file, err := t.parse(path.Join(dir, name))
		if err != nil {
			return nil, err
		}
		if pkg.name == "" {
			pkg.name = file.ast.Name.Name
		}
		pkg.files = append(pkg.files, file)
		for _, decl := range file.ast.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, ident := range vs.Names {
					if i < len(vs.Values) {
						pkg.consts[ident.Name] = constDef{value: vs.Values[i], file: file, pkg: pkg}
					}
				}
			}
		}
	}
	t.packages[dir] = pkg
	return pkg, nil
}

func (t *goTree) builds(dir, name string) (bool, error) {
	for _, ctx := range t.contexts {
		ok, err := ctx.MatchFile(dir, name)
		if err != nil {
			return false, fmt.Errorf("match %s/%s against linux/%s: %w", dir, name, ctx.GOARCH, err)
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func (t *goTree) parse(file string) (*goFile, error) {
	src, err := fs.ReadFile(t.fsys, file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}
	f, err := parser.ParseFile(t.fset, file, src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}
	imports := map[string]string{}
	for _, spec := range f.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("%s: import %s: %w", file, spec.Path.Value, err)
		}
		name := path.Base(imported)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "." {
			if _, ok := processStarts[imported]; ok {
				return nil, fmt.Errorf("%s: a dot import of %s hides its process starts from this check", t.fset.Position(spec.Pos()), imported)
			}
		}
		imports[name] = imported
	}
	return &goFile{path: file, ast: f, imports: imports}, nil
}

func (t *goTree) sites(dirs []string) ([]site[goKey], error) {
	var sites []site[goKey]
	for _, dir := range dirs {
		pkg, err := t.load(dir)
		if err != nil {
			return nil, err
		}
		for _, file := range pkg.files {
			sites = append(sites, t.fileSites(pkg, file)...)
		}
	}
	return sites, nil
}

type scope struct {
	name string
	node ast.Node
}

func scopes(f *ast.File) []scope {
	var out []scope
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			out = append(out, scope{funcName(d), d})
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					out = append(out, scope{vs.Names[0].Name, vs})
				}
			}
		}
	}
	return out
}

func (t *goTree) fileSites(pkg *goPackage, file *goFile) []site[goKey] {
	var sites []site[goKey]
	for _, sc := range scopes(file.ast) {
		handled := map[*ast.SelectorExpr]bool{}
		add := func(node ast.Node, what, program string) {
			sites = append(sites, site[goKey]{
				at:   position{file.path, t.fset.Position(node.Pos()).Line},
				what: what,
				key:  goKey{pkg.dir, sc.name, program},
			})
		}
		ast.Inspect(sc.node, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				sel, imported, name, ok := file.target(n.Fun)
				if !ok {
					return true
				}
				handled[sel] = true
				program := dynamic
				if arg := processStarts[imported][name]; arg < len(n.Args) && (!n.Ellipsis.IsValid() || arg < len(n.Args)-1) {
					if v, ok := t.resolve(n.Args[arg], file, pkg, nil); ok {
						program = v
					}
				}
				add(n, imported+"."+name, program)
			case *ast.CompositeLit:
				if file.isCmdType(n.Type) {
					add(n, execPackage+"."+cmdType+"{}", t.cmdPath(n, file, pkg))
				}
			case *ast.SelectorExpr:
				if _, imported, name, ok := file.target(n); ok && !handled[n] {
					add(n, imported+"."+name+" as a value", dynamic)
				}
			}
			return true
		})
	}
	return sites
}

func (t *goTree) cmdPath(lit *ast.CompositeLit, file *goFile, pkg *goPackage) string {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := kv.Key.(*ast.Ident); !ok || key.Name != cmdPathField {
			continue
		}
		if v, ok := t.resolve(kv.Value, file, pkg, nil); ok {
			return v
		}
	}
	return dynamic
}

func (f *goFile) importOf(expr ast.Expr) (*ast.SelectorExpr, string, bool) {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return nil, "", false
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok || x.Obj != nil {
		return nil, "", false
	}
	imported, ok := f.imports[x.Name]
	return sel, imported, ok
}

func (f *goFile) target(expr ast.Expr) (*ast.SelectorExpr, string, string, bool) {
	sel, imported, ok := f.importOf(expr)
	if !ok {
		return nil, "", "", false
	}
	if _, ok := processStarts[imported][sel.Sel.Name]; !ok {
		return nil, "", "", false
	}
	return sel, imported, sel.Sel.Name, true
}

func (f *goFile) isCmdType(expr ast.Expr) bool {
	sel, imported, ok := f.importOf(expr)
	return ok && imported == execPackage && sel.Sel.Name == cmdType
}

func (t *goTree) resolve(expr ast.Expr, file *goFile, pkg *goPackage, seen map[ast.Expr]bool) (string, bool) {
	if seen[expr] {
		return "", false
	}
	if seen == nil {
		seen = map[ast.Expr]bool{}
	}
	seen[expr] = true
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		v, err := strconv.Unquote(e.Value)
		return v, err == nil
	case *ast.ParenExpr:
		return t.resolve(e.X, file, pkg, seen)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		x, ok := t.resolve(e.X, file, pkg, seen)
		if !ok {
			return "", false
		}
		y, ok := t.resolve(e.Y, file, pkg, seen)
		return x + y, ok
	case *ast.Ident:
		if e.Obj != nil {
			return t.resolveObject(e, file, pkg, seen)
		}
		def, ok := pkg.consts[e.Name]
		if !ok {
			return "", false
		}
		return t.resolve(def.value, def.file, def.pkg, seen)
	case *ast.SelectorExpr:
		_, imported, ok := file.importOf(e)
		if !ok {
			return "", false
		}
		dir, inModule := strings.CutPrefix(imported, t.module+"/")
		if !inModule {
			return "", false
		}
		other, err := t.load(dir)
		if err != nil {
			return "", false
		}
		def, ok := other.consts[e.Sel.Name]
		if !ok {
			return "", false
		}
		return t.resolve(def.value, def.file, def.pkg, seen)
	default:
		return "", false
	}
}

func (t *goTree) resolveObject(ident *ast.Ident, file *goFile, pkg *goPackage, seen map[ast.Expr]bool) (string, bool) {
	if ident.Obj.Kind != ast.Con {
		return "", false
	}
	spec, ok := ident.Obj.Decl.(*ast.ValueSpec)
	if !ok {
		return "", false
	}
	for i, name := range spec.Names {
		if name.Name == ident.Name && i < len(spec.Values) {
			return t.resolve(spec.Values[i], file, pkg, seen)
		}
	}
	return "", false
}

func funcName(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return d.Name.Name
	}
	return receiverName(d.Recv.List[0].Type) + "." + d.Name.Name
}

func receiverName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return receiverName(e.X)
	case *ast.IndexExpr:
		return receiverName(e.X)
	case *ast.IndexListExpr:
		return receiverName(e.X)
	case *ast.Ident:
		return e.Name
	default:
		return fmt.Sprintf("%T", expr)
	}
}
