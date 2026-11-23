package i18ntest

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestTheReasonScanFindsEachMessageTakingAnError(t *testing.T) {
	const src = `# { $reason } in a comment is no placeable
app-plain = Saved
app-bad-reason = Cannot read it: { $reason }
app-bad-error = Failed ({ $error })
app-bad-err =
    Two lines,
    the second { $err }
app-attr = Fine
    .tooltip = Why: { $reason }
app-sentence = Suspends only: { $reason }
app-reasoning = { $reasons } and { $errors } are other names
app-twice = { $reason } and again { $reason }
`
	if got, want := RawReasons(src, "app-sentence"), []string{"app-bad-reason", "app-bad-error", "app-bad-err", "app-attr", "app-twice"}; !slices.Equal(got, want) {
		t.Errorf("the scan finds %q, want %q", got, want)
	}
	if got := RawReasons(src); !slices.Contains(got, "app-sentence") {
		t.Errorf("without its sentence named, the scan finds %q", got)
	}
	if got := RawReasons("app-plain = Saved\napp-count = { $count } items\n"); len(got) != 0 {
		t.Errorf("the scan flags %q in messages taking no error", got)
	}
}

func lines(found []string) []string {
	var out []string
	for _, at := range found {
		_, rest, _ := strings.Cut(at, ":")
		line, _, _ := strings.Cut(rest, ":")
		out = append(out, line)
	}
	return out
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
	found, err := RawErrorsIn("bad.go", bad)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := lines(found), []string{"3", "5", "7", "9", "11", "13", "15", "17"}; !slices.Equal(got, want) {
		t.Errorf("the scan finds %q, want %q", got, want)
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
	if found, err := RawErrorsIn("good.go", good); err != nil || len(found) != 0 {
		t.Errorf("the scan flags %q (%v) in code keeping errors off the screen", found, err)
	}
	if _, err := RawErrorsIn("broken.go", "package bar\nfunc {"); err == nil || !strings.Contains(err.Error(), "broken.go") {
		t.Errorf("a file that does not parse: %v, want its parse error", err)
	}
}

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGoSourcesSkipTestsDoublesAndTheExempt(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"a.go":                  "package a\n",
		"a_test.go":             "package a\n",
		"notes.txt":             "",
		"alert.go":              "package a\n",
		"sub/b.go":              "package sub\n",
		"sub/testdata/c.go":     "package c\n",
		"sub/subtest/d.go":      "package subtest\n",
		"other/e.go":            "package other\n",
		"other/e_amd64.go":      "package other\n",
		"other/fixture_test.go": "package other\n",
	})
	files, err := GoSources([]string{root}, filepath.Join(root, "alert.go"))
	if err != nil {
		t.Fatal(err)
	}
	var rel []string
	for _, f := range files {
		r, err := filepath.Rel(root, f)
		if err != nil {
			t.Fatal(err)
		}
		rel = append(rel, filepath.ToSlash(r))
	}
	if want := []string{"a.go", "other/e.go", "other/e_amd64.go", "sub/b.go"}; !slices.Equal(rel, want) {
		t.Errorf("sources %q, want %q", rel, want)
	}

	if _, err := GoSources([]string{root}, filepath.Join(root, "gone.go")); err == nil || !strings.Contains(err.Error(), "gone.go") {
		t.Errorf("an exempt file that is gone: %v, want an error naming it", err)
	}
	if _, err := GoSources([]string{filepath.Join(root, "missing")}); err == nil {
		t.Error("a root that is gone walks without an error")
	}
}

func TestRawErrorsReadEachFile(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"a.go": "package a\n\nfunc a(err error) string { return err.Error() }\n",
		"b.go": "package a\n\nfunc b(err error) error { return fmt.Errorf(\"b: %w\", err) }\n",
	})
	found, err := RawErrors([]string{filepath.Join(root, "a.go"), filepath.Join(root, "b.go")})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "a.go") + ":3:"; len(found) != 1 || !strings.HasPrefix(found[0], want) {
		t.Errorf("found %q, want one at %s", found, want)
	}
	if _, err := RawErrors([]string{filepath.Join(root, "gone.go")}); err == nil || !strings.Contains(err.Error(), "gone.go") {
		t.Errorf("a file that is gone: %v, want an error naming it", err)
	}
}
