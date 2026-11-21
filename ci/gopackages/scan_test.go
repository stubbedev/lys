package main

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

const testModule = "example.com/m"

func testScan(t *testing.T, files map[string]string) (map[string]Package, error) {
	t.Helper()
	fsys := fstest.MapFS{}
	for name, src := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(src)}
	}
	return scan(fsys, testModule, slices.Sorted(maps.Keys(files)))
}

func mustScan(t *testing.T, files map[string]string) map[string]Package {
	t.Helper()
	packages, err := testScan(t, files)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	return packages
}

func TestAPackageListsItsModuleImportsAndWhatItsTestsAdd(t *testing.T) {
	got := mustScan(t, map[string]string{
		"lib/a/a.go":       "package a\nimport (\n\t\"fmt\"\n\t\"example.com/m/lib/b\"\n\t\"example.com/m/lib/c\"\n)\n",
		"lib/a/x_linux.go": "//go:build linux\n\npackage a\nimport _ \"example.com/m/lib/b\"\n",
		"lib/a/a_test.go":  "package a_test\nimport (\n\t\"testing\"\n\t\"example.com/m/lib/a\"\n\t\"example.com/m/lib/b\"\n\t\"example.com/m/lib/d\"\n)\n",
		"lib/b/b.go":       "package b\n",
		"lib/c/c.go":       "package c\nimport \"other.org/x\"\n",
		"lib/d/d.go":       "package d\n",
		"README.md":        "not Go\n",
	})
	want := map[string]Package{
		"lib/a": {Imports: []string{"lib/b", "lib/c"}, External: []string{"fmt"}, TestImports: []string{"lib/d"}, TestExternal: []string{"testing"}},
		"lib/b": {},
		"lib/c": {External: []string{"other.org/x"}},
		"lib/d": {},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scan = %+v, want %+v", got, want)
	}
}

func TestEmbedsNameTheLiteralPathTheirPatternsStartFrom(t *testing.T) {
	got := mustScan(t, map[string]string{
		"p/p.go": "package p\nimport \"embed\"\n\n" +
			"//go:embed default.yaml\nvar a string\n\n" +
			"//go:embed icons/scalable/*.svg \"quoted dir/x.txt\"\nvar b embed.FS\n\n" +
			"//go:embed all:*\nvar c embed.FS\n",
		"p/p_test.go":                 "package p\nimport _ \"embed\"\n\n//go:embed testdata/golden.png\nvar d []byte\n",
		"p/default.yaml":              "",
		"p/icons/scalable/one.svg":    "",
		"p/quoted dir/x.txt":          "",
		"p/testdata/golden.png":       "",
		"p/testdata/inner/ignored.go": "package ignored\nimport \"example.com/m/nowhere\"\n",
	})
	want := map[string]Package{"p": {
		External:   []string{"embed"},
		Embeds:     []string{"p", "p/default.yaml", "p/icons/scalable", "p/quoted dir/x.txt"},
		TestEmbeds: []string{"p/testdata/golden.png"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scan = %+v, want %+v", got, want)
	}
}

func TestDirectoriesAndFilesGoIgnoresAreSkipped(t *testing.T) {
	got := mustScan(t, map[string]string{
		"p/p.go":             "package p\n",
		"p/_draft.go":        "package p\nimport \"example.com/m/nowhere\"\n",
		"p/.hidden.go":       "package p\nimport \"example.com/m/nowhere\"\n",
		"p/testdata/x/x.go":  "package x\nimport \"example.com/m/nowhere\"\n",
		"_tools/t/t.go":      "package t\nimport \"example.com/m/nowhere\"\n",
		".github/g/g.go":     "package g\n",
		"vendor/v/v.go":      "package v\n",
		"p/inner/notgo.yaml": "",
	})
	if want := []string{"p"}; !slices.Equal(slices.Sorted(maps.Keys(got)), want) {
		t.Fatalf("packages = %v, want %v", slices.Sorted(maps.Keys(got)), want)
	}
}

func TestExternalImportsLeaveOutTheModuleAndCgo(t *testing.T) {
	got := mustScan(t, map[string]string{
		"a/a.go":      "package a\nimport (\n\t\"C\"\n\t\"example.com/m\"\n\t\"example.com/m/b\"\n\t\"example.com/mod/x\"\n\t\"os\"\n)\n",
		"a/a_test.go": "package a\nimport (\n\t\"os\"\n\t\"testing\"\n)\n",
		"b/b.go":      "package b\nimport \"os\"\n",
	})
	want := map[string]Package{
		"a": {Imports: []string{"b"}, External: []string{"example.com/mod/x", "os"}, TestExternal: []string{"testing"}},
		"b": {External: []string{"os"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scan = %+v, want %+v", got, want)
	}
}

func TestAnImportOfADirectoryWithoutGoFilesFails(t *testing.T) {
	_, err := testScan(t, map[string]string{
		"a/a.go":    "package a\nimport \"example.com/m/gone\"\n",
		"gone/x.md": "",
	})
	if err == nil || !strings.Contains(err.Error(), "a imports example.com/m/gone, which holds no Go files") {
		t.Fatalf("scan error = %v, want the missing package named", err)
	}
}

func TestAnEmbedPatternMatchingNothingFails(t *testing.T) {
	_, err := testScan(t, map[string]string{
		"a/a.go": "package a\nimport _ \"embed\"\n\n//go:embed shaders/*.spv\nvar s []byte\n",
	})
	if err == nil || !strings.Contains(err.Error(), `a/a.go: the embed pattern "shaders/*.spv" matches no tracked file under a/shaders`) {
		t.Fatalf("scan error = %v, want the empty pattern named", err)
	}
}

func TestAFileThatDoesNotParseFails(t *testing.T) {
	_, err := testScan(t, map[string]string{"a/a.go": "package a\nfunc {\n"})
	if err == nil || !strings.Contains(err.Error(), "parse a/a.go") {
		t.Fatalf("scan error = %v, want the file named", err)
	}
}

func TestAnEmbedDirectiveWithoutPatternsFails(t *testing.T) {
	_, err := testScan(t, map[string]string{"a/a.go": "package a\nimport _ \"embed\"\n\n//go:embed\nvar s []byte\n"})
	if err == nil || !strings.Contains(err.Error(), "//go:embed names no pattern") {
		t.Fatalf("scan error = %v, want the empty directive named", err)
	}
}

func TestTheEncodingIsStable(t *testing.T) {
	files := map[string]string{
		"b/b.go": "package b\nimport (\n\t\"example.com/m/a\"\n\t\"example.com/m/c\"\n)\n",
		"b/z.go": "package b\nimport \"example.com/m/a\"\n",
		"a/a.go": "package a\n",
		"c/c.go": "package c\n",
	}
	first, err := encode(mustScan(t, files))
	if err != nil {
		t.Fatal(err)
	}
	second, err := encode(mustScan(t, files))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"a\": {},\n  \"b\": {\n    \"imports\": [\n      \"a\",\n      \"c\"\n    ]\n  },\n  \"c\": {}\n}\n"
	if string(first) != want || string(second) != want {
		t.Fatalf("encode = %q then %q, want %q", first, second, want)
	}
}
