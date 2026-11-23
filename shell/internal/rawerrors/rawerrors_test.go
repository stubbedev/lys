// Package rawerrors_test holds the check that no error's own text
// reaches the shell's screens outside an alert's description.
package rawerrors_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// uiRoots are the trees whose code puts text on the shell's screens:
// the surfaces, and the launcher's providers, whose outcomes the
// launcher shows as they come.
var uiRoots = []string{"../../shell", "../../service/launcher"}

// detailFile is the one file whose code may turn an error into text:
// the bar's failure alert and page, which show it as a description
// under a summary in the shell's words.
const detailFile = "../../shell/bar/failurealert.go"

// errorLike is a name an error goes by.
var errorLike = regexp.MustCompile(`(err|Err)$`)

// rawErrorsIn is where src, the file named name, turns an error into
// text: an Error call, or an error-named value formatted by fmt.Sprint,
// Sprintf or Sprintln. An Error method may wrap another error's text; a
// panic, a log line or fmt.Errorf may format one; a D-Bus error answers
// a program, not a person; and an Error call compared with == or !=
// yields no text.
func rawErrorsIn(name, src string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
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

// uiSources are the non-test Go files under uiRoots but detailFile,
// past testdata and the test doubles' packages (named for what they
// stand in for, ending in test).
func uiSources(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, root := range uiRoots {
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
			if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") && p != filepath.Clean(detailFile) {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// No error's own text reaches a label, a title, a toast, a notification
// or a message outside the bar's failure alert, where it is the
// description under a summary in the shell's words.
func TestNoErrorTextReachesTheShellsScreensOutsideAFailureAlert(t *testing.T) {
	files := uiSources(t)
	if !slices.Contains(files, filepath.Clean("../../shell/bar/dock.go")) || !slices.Contains(files, filepath.Clean("../../service/launcher/search/env.go")) {
		t.Fatalf("the scan reads %d files without the bar's or the launcher's", len(files))
	}
	if _, err := os.Stat(detailFile); err != nil {
		t.Fatalf("the failure alert's file: %v", err)
	}
	for _, name := range files {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		found, err := rawErrorsIn(name, string(src))
		if err != nil {
			t.Fatal(err)
		}
		for _, at := range found {
			t.Errorf("%s: an error's text reaches the screen; show a summary, with the error as a failure alert's description or in the log", at)
		}
	}
}

func TestTheErrorTextScanCatchesEachWayAnErrorBecomesText(t *testing.T) {
	const bad = `package bar

func a(l *widget.Label, err error) { l.SetText(err.Error()) }

func b(err error) string { return i18n.T("x", i18n.Str("error", err.Error())) }

func c(toast func(string, string), s state) { toast(s.err.Error(), "") }

func d(err error) string { return fmt.Sprintf("failed: %v", err) }

func e(p probe) string { return fmt.Sprint(p.lastErr) }

func f(err error) string { return strings.TrimSpace(err.Error()) }

func g(err error) Outcome { return Stay{Message: err.Error()} }

func h(err error) bool { return len(err.Error()) == 0 }
`
	found, err := rawErrorsIn("bad.go", bad)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, at := range found {
		_, rest, _ := strings.Cut(at, ":")
		line, _, _ := strings.Cut(rest, ":")
		lines = append(lines, line)
	}
	if want := []string{"3", "5", "7", "9", "11", "13", "15", "17"}; !slices.Equal(lines, want) {
		t.Errorf("the scan finds %q, want %q", lines, want)
	}

	const good = `package bar

type wrapped struct{ inner error }

func (w wrapped) Error() string { return "wrapped: " + w.inner.Error() }

func a(err error) error { return fmt.Errorf("read: %w", err) }

func b(err error) { log.Printf("read: %v", err) }

func c(err error) { panic(fmt.Sprintf("read: %v", err)) }

func d(a *widget.Alert, err error) { reportWrite(a, "x", i18n.T("x"), err) }

func e(n int) string { return fmt.Sprint(n) }

func f(err error) *dbus.Error { return dbusx.InvalidArgs(err.Error()) }

func g(err error) *dbus.Error { return dbus.NewError("x", []any{err.Error()}) }

func h(a, b error) bool { return a.Error() == b.Error() }
`
	if found, err := rawErrorsIn("good.go", good); err != nil || len(found) != 0 {
		t.Errorf("the scan flags %q (%v) in code keeping errors off the screen", found, err)
	}
}
