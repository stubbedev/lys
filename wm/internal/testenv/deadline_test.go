package testenv

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

type deadlineCall struct {
	pkg, fn string
	arg     int
}

var deadlineCalls = []deadlineCall{
	{"time", "After", 0},
	{"time", "NewTimer", 0},
	{"time", "AfterFunc", 0},
	{"context", "WithTimeout", 1},
	{"net", "DialTimeout", 2},
}

type fixedDeadline struct {
	at    token.Position
	value time.Duration
}

func (d fixedDeadline) String() string {
	return fmt.Sprintf("%s:%d: gives up after a fixed %v", d.at.Filename, d.at.Line, d.value)
}

type sourcePackage struct {
	files, tests []*ast.File
}

func TestNoTestGivesUpAtAFixedDeadline(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()
	packages := map[string]*sourcePackage{}
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return skipDir(root, path, d.Name())
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		file, err := parser.ParseFile(fset, rel, src, parser.SkipObjectResolution)
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		dir := filepath.Dir(rel)
		pkg, seen := packages[dir]
		if !seen {
			pkg = &sourcePackage{}
			packages[dir] = pkg
			dirs = append(dirs, dir)
		}
		pkg.files = append(pkg.files, file)
		if isTestCode(path, file) {
			pkg.tests = append(pkg.tests, file)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the module at %s: %v", root, err)
	}
	for _, dir := range dirs {
		for _, d := range packages[dir].fixedDeadlines(fset) {
			t.Errorf("%v; wait on the condition, or give up at testenv.HangLimit", d)
		}
	}
}

func TestFixedDeadlinesAreFoundAndConditionWaitsAreNot(t *testing.T) {
	const test = `package p

import (
	"context"
	"net"
	"time"

	"github.com/stubbedev/lys/wm/internal/testenv"
)

const settle = 10 * time.Second

func f(ctx context.Context, d time.Duration, guard, never interface{ Arm(time.Duration) error }) {
	<-time.After(5 * time.Second)
	_ = time.Now().Add(settle)
	_ = time.Now().Add(2 * settle / 4)
	_, _ = context.WithTimeout(ctx, 20*time.Second)
	_, _ = net.DialTimeout("unix", "x", 3*time.Second)
	_ = time.NewTimer(1500 * time.Millisecond)
	_ = guard.Arm(settle)
	<-time.After(ioTimeout / 2)
	<-time.After(testenv.HangLimit)
	<-time.After(30 * time.Second)
	_, _ = context.WithTimeout(ctx, time.Minute)
	_ = never.Arm(time.Hour)
	<-time.After(50 * time.Millisecond)
	<-time.After(d)
	_ = time.Now().Add(d * 2)
	_ = time.Now().Add(time.Second / 2)
	_ = later.Add(5 * time.Second)
}
`
	const lib = `package p

import "time"

const ioTimeout = 5 * time.Second
`
	fset := token.NewFileSet()
	var pkg sourcePackage
	for name, src := range map[string]string{"p_test.go": test, "p.go": lib} {
		file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		pkg.files = append(pkg.files, file)
		if name == "p_test.go" {
			pkg.tests = append(pkg.tests, file)
		}
	}
	var got []string
	for _, d := range pkg.fixedDeadlines(fset) {
		got = append(got, fmt.Sprintf("%s:%d %v", d.at.Filename, d.at.Line, d.value))
	}
	want := []string{"p_test.go:14 5s", "p_test.go:15 10s", "p_test.go:16 5s", "p_test.go:17 20s", "p_test.go:18 3s", "p_test.go:19 1.5s", "p_test.go:20 10s", "p_test.go:21 2.5s"}
	if !slices.Equal(got, want) {
		t.Fatalf("found %q, want %q", got, want)
	}
}

func isTestCode(path string, file *ast.File) bool {
	if strings.HasSuffix(path, "_test.go") {
		return true
	}
	if file.Name.Name == "main" {
		return false
	}
	for _, spec := range file.Imports {
		if path, err := strconv.Unquote(spec.Path.Value); err == nil && path == "testing" {
			return true
		}
	}
	return false
}

func (p *sourcePackage) fixedDeadlines(fset *token.FileSet) []fixedDeadline {
	consts := map[string]ast.Expr{}
	for _, file := range p.files {
		collectConsts(file, consts)
	}
	var found []fixedDeadline
	for _, file := range p.tests {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			arg, ok := deadlineArg(call)
			if !ok {
				return true
			}
			v, ok := durationOf(arg, consts, map[string]bool{})
			if ok && v >= time.Second && v < HangLimit {
				found = append(found, fixedDeadline{at: fset.Position(arg.Pos()), value: v})
			}
			return true
		})
	}
	return found
}

func collectConsts(file *ast.File, consts map[string]ast.Expr) {
	ast.Inspect(file, func(n ast.Node) bool {
		decl, ok := n.(*ast.GenDecl)
		if !ok || decl.Tok != token.CONST {
			return true
		}
		for _, spec := range decl.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Values) != len(vs.Names) {
				continue
			}
			for i, name := range vs.Names {
				consts[name.Name] = vs.Values[i]
			}
		}
		return true
	})
}

func deadlineArg(call *ast.CallExpr) (ast.Expr, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	if sel.Sel.Name == "Add" && len(call.Args) == 1 && isCall(sel.X, "time", "Now") {
		return call.Args[0], true
	}
	if sel.Sel.Name == "Arm" && len(call.Args) == 1 {
		return call.Args[0], true
	}
	for _, d := range deadlineCalls {
		if isQualified(sel, d.pkg, d.fn) && len(call.Args) > d.arg {
			return call.Args[d.arg], true
		}
	}
	return nil, false
}

func isCall(e ast.Expr, pkg, fn string) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && isQualified(sel, pkg, fn)
}

func isQualified(sel *ast.SelectorExpr, pkg, name string) bool {
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == pkg && sel.Sel.Name == name
}

var timeUnits = map[string]time.Duration{
	"Nanosecond":  time.Nanosecond,
	"Microsecond": time.Microsecond,
	"Millisecond": time.Millisecond,
	"Second":      time.Second,
	"Minute":      time.Minute,
	"Hour":        time.Hour,
}

func durationOf(e ast.Expr, consts map[string]ast.Expr, resolving map[string]bool) (time.Duration, bool) {
	v, ok := constantOf(e, consts, resolving)
	if !ok {
		return 0, false
	}
	n, exact := constant.Int64Val(constant.ToInt(v))
	return time.Duration(n), exact
}

func constantOf(e ast.Expr, consts map[string]ast.Expr, resolving map[string]bool) (constant.Value, bool) {
	switch e := e.(type) {
	case *ast.BasicLit:
		v := constant.MakeFromLiteral(e.Value, e.Kind, 0)
		return v, v.Kind() != constant.Unknown
	case *ast.ParenExpr:
		return constantOf(e.X, consts, resolving)
	case *ast.SelectorExpr:
		unit, ok := timeUnits[e.Sel.Name]
		if !ok || !isQualified(e, "time", e.Sel.Name) {
			return nil, false
		}
		return constant.MakeInt64(int64(unit)), true
	case *ast.Ident:
		value, ok := consts[e.Name]
		if !ok || resolving[e.Name] {
			return nil, false
		}
		resolving[e.Name] = true
		defer delete(resolving, e.Name)
		return constantOf(value, consts, resolving)
	case *ast.BinaryExpr:
		x, ok := constantOf(e.X, consts, resolving)
		if !ok {
			return nil, false
		}
		y, ok := constantOf(e.Y, consts, resolving)
		if !ok {
			return nil, false
		}
		switch e.Op {
		case token.ADD, token.SUB, token.MUL:
			return constant.BinaryOp(x, e.Op, y), true
		case token.QUO:
			if constant.Sign(y) == 0 {
				return nil, false
			}
			if x.Kind() == constant.Int && y.Kind() == constant.Int {
				return constant.BinaryOp(x, token.QUO_ASSIGN, y), true
			}
			return constant.BinaryOp(x, token.QUO, y), true
		}
	}
	return nil, false
}
