package settings

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// wantClosedFailure requires s, whose face reads shown, to show summary
// with the error, holding errPart, behind its closed disclosure and
// nowhere in what it reads.
func wantClosedFailure(t *testing.T, what string, s *stateParts, shown, summary, errPart string) {
	t.Helper()
	if s.kind == stateNone || shown != summary || strings.Contains(shown, errPart) || !strings.Contains(s.detail.Text(), errPart) || s.detail.Visible() || !s.reveal.button.Visible() {
		t.Errorf("%s shows %v %q over the detail %q (shown %v), want %q with the error holding %q behind the closed disclosure", what, s.kind, shown, s.detail.Text(), s.detail.Visible(), summary, errPart)
	}
}

// stateFile is the one file whose code may turn an error into text: the
// states' detail.
const stateFile = "cardstate.go"

// errorLike is a name an error goes by.
var errorLike = regexp.MustCompile(`(err|Err)$`)

// rawErrorsIn is where src, the file named name, turns an error into
// text: an Error call, or an error-named value formatted by fmt.Sprint,
// Sprintf or Sprintln. An Error method may wrap another error's text,
// and a panic, a log line or fmt.Errorf may format one.
func rawErrorsIn(name, src string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var found []string
	report := func(n ast.Node) {
		found = append(found, fset.Position(n.Pos()).String())
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil && fn.Name.Name == "Error" {
			continue
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch callee := calleeName(call.Fun); {
			case callee == "panic", callee == "fmt.Errorf", strings.HasPrefix(callee, "log."):
				return false
			case strings.HasSuffix(callee, ".Error") && len(call.Args) == 0:
				report(call)
			case callee == "fmt.Sprint", callee == "fmt.Sprintf", callee == "fmt.Sprintln":
				if slices.ContainsFunc(call.Args, isErrorNamed) {
					report(call)
				}
			}
			return true
		})
	}
	return found, nil
}

// calleeName is a call's function as written: panic, fmt.Sprintf, or
// x.Error for any receiver x.
func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		if pkg, ok := f.X.(*ast.Ident); ok && (pkg.Name == "fmt" || pkg.Name == "log") {
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

// No error's own text reaches a label, a title, a tooltip or a message
// outside a state's detail, where it waits behind a disclosure under a
// summary in the app's words.
func TestNoErrorTextReachesTheUIOutsideAStatesDetail(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") || name == stateFile {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		found, err := rawErrorsIn(name, string(src))
		if err != nil {
			t.Fatal(err)
		}
		for _, at := range found {
			t.Errorf("%s: an error's text reaches the UI; show a summary with the error as a state's detail", at)
		}
	}
}

func TestTheErrorTextScanCatchesEachWayAnErrorBecomesText(t *testing.T) {
	const bad = `package settings

func a(l *widget.Label, err error) { l.SetText(err.Error()) }

func b(err error) string { return locale.T().Get("x", i18n.Str("reason", err.Error())) }

func c(l *widget.Label, s state) { l.SetTooltip(s.err.Error()) }

func d(err error) string { return fmt.Sprintf("failed: %v", err) }

func e(p probe) string { return fmt.Sprint(p.lastErr) }

func f(err error) string { return strings.TrimSpace(err.Error()) }
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
	if want := []string{"3", "5", "7", "9", "11", "13"}; !slices.Equal(lines, want) {
		t.Errorf("the scan finds %q, want %q", lines, want)
	}

	const good = `package settings

type wrapped struct{ inner error }

func (w wrapped) Error() string { return "wrapped: " + w.inner.Error() }

func a(err error) error { return fmt.Errorf("read: %w", err) }

func b(err error) { log.Printf("read: %v", err) }

func c(err error) { panic(fmt.Sprintf("read: %v", err)) }

func d(s state, err error) { s.Error(locale.T().Get("x"), err, nil) }

func e(n int) string { return fmt.Sprint(n) }
`
	if found, err := rawErrorsIn("good.go", good); err != nil || len(found) != 0 {
		t.Errorf("the scan flags %q (%v) in code keeping errors out of the UI", found, err)
	}
}
