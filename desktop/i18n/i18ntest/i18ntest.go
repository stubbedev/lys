// Package i18ntest holds the checks that keep an error's own text off a
// person's screen: a scan of Fluent sources for messages taking one,
// and a scan of Go sources for code turning one into text. Each app's
// tests run them over its own catalogues and surfaces.
package i18ntest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	ftlMessage    = regexp.MustCompile(`^(-?[a-zA-Z][a-zA-Z0-9_-]*)\s*=`)
	rawReasonPart = regexp.MustCompile(`\{[^}]*\$(reason|error|err)\b`)
)

// RawReasons are the messages of src, a Fluent source, taking a
// $reason, $error or $err placeable, in order, but those sentences
// names: messages whose placeable is always a localized sentence,
// never an error's own text.
func RawReasons(src string, sentences ...string) []string {
	var found []string
	id := ""
	for line := range strings.SplitSeq(src, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if m := ftlMessage.FindStringSubmatch(line); m != nil {
			id = m[1]
		}
		if rawReasonPart.MatchString(line) && !slices.Contains(sentences, id) && !slices.Contains(found, id) {
			found = append(found, id)
		}
	}
	return found
}

// errorLike is a name an error goes by.
var errorLike = regexp.MustCompile(`(err|Err)$`)

// RawErrorsIn is where src, the Go file named name, turns an error into
// text: an Error call, or an error-named value formatted by fmt.Sprint,
// Sprintf or Sprintln. An Error method may wrap another error's text; a
// panic, a log line or fmt.Errorf may format one; a D-Bus error answers
// a program, not a person; and an Error call compared with == or !=
// yields no text.
func RawErrorsIn(name, src string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("i18ntest: parse %s: %w", name, err)
	}
	var found []string
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil && fn.Name.Name == "Error" {
			continue
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BinaryExpr:
				if x.Op == token.EQL || x.Op == token.NEQ {
					return !isErrorCall(x.X) && !isErrorCall(x.Y)
				}
			case *ast.CallExpr:
				switch callee := calleeName(x.Fun); {
				case callee == "panic", callee == "fmt.Errorf", strings.HasPrefix(callee, "log."),
					strings.HasPrefix(callee, "dbusx."), callee == "dbus.NewError":
					return false
				case isErrorCall(x):
					found = append(found, fset.Position(x.Pos()).String())
				case callee == "fmt.Sprint", callee == "fmt.Sprintf", callee == "fmt.Sprintln":
					if slices.ContainsFunc(x.Args, isErrorNamed) {
						found = append(found, fset.Position(x.Pos()).String())
					}
				}
			}
			return true
		})
	}
	return found, nil
}

// RawErrors is RawErrorsIn over every file of files, in order.
func RawErrors(files []string) ([]string, error) {
	var found []string
	for _, name := range files {
		src, err := os.ReadFile(name) //nolint:gosec // a source file of the repository a test names
		if err != nil {
			return nil, fmt.Errorf("i18ntest: read %s: %w", name, err)
		}
		at, err := RawErrorsIn(name, string(src))
		if err != nil {
			return nil, err
		}
		found = append(found, at...)
	}
	return found, nil
}

// isErrorCall says e is x.Error() for any receiver x.
func isErrorCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	return ok && len(call.Args) == 0 && calleeName(call.Fun) == ".Error"
}

// calleeName is a call's function as written: panic, fmt.Sprintf,
// dbusx.Failed, or .Error for a method of any receiver.
func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		if pkg, ok := f.X.(*ast.Ident); ok && slices.Contains([]string{"fmt", "log", "dbus", "dbusx"}, pkg.Name) {
			return pkg.Name + "." + f.Sel.Name
		}
		return "." + f.Sel.Name
	}
	return ""
}

func isErrorNamed(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.Ident:
		return errorLike.MatchString(v.Name)
	case *ast.SelectorExpr:
		return errorLike.MatchString(v.Sel.Name)
	}
	return false
}

// GoSources are the non-test Go files under roots but those except
// names, past testdata and the test doubles' packages (named for what
// they stand in for, ending in test), in walk order. Every name in
// except must be one of them, so an exemption cannot outlive its file.
func GoSources(roots []string, except ...string) ([]string, error) {
	var files []string
	exempt := map[string]bool{}
	for _, name := range except {
		exempt[filepath.Clean(name)] = false
	}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if name := d.Name(); p != root && (name == "testdata" || strings.HasSuffix(name, "test")) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			if _, ok := exempt[p]; ok {
				exempt[p] = true
				return nil
			}
			files = append(files, p)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("i18ntest: walk %s: %w", root, err)
		}
	}
	for _, name := range except {
		if !exempt[filepath.Clean(name)] {
			return nil, fmt.Errorf("i18ntest: the exempt %s is no Go source under %q", name, roots)
		}
	}
	return files, nil
}
