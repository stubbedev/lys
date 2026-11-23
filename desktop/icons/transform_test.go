package icons

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/desktop/icons/usvg"
)

// TestToSymbolicMatchesGoldens holds ToSymbolic and usvg.Parse to the
// goldens of every SVG in testdata/symbolic (see testdata/README.md):
// the same bytes out, or no icon (NONE), and the same files rejected by
// the parser with the same error. SYMBOLIC_CORPUS adds a directory laid
// out the same way.
func TestToSymbolicMatchesGoldens(t *testing.T) {
	dirs := []string{filepath.Join("testdata", "symbolic")}
	if extra := os.Getenv("SYMBOLIC_CORPUS"); extra != "" {
		dirs = append(dirs, extra)
	}
	for _, dir := range dirs {
		files, err := filepath.Glob(filepath.Join(dir, "*.svg"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no corpus in %s: %v", dir, err)
		}
		for _, file := range files {
			t.Run(filepath.Base(file), func(t *testing.T) {
				src := readFile(t, file)
				golden := readFile(t, file+".golden")
				got, ok := ToSymbolic(src)
				switch {
				case golden == "NONE" && ok:
					t.Errorf("got an icon where the golden has none:\n%s", got)
				case golden != "NONE" && !ok:
					t.Error("no icon where the golden has one")
				case ok && got != golden:
					t.Errorf("output differs from the golden:\n%s", firstDifference(got, golden))
				}
				gotParse := "OK"
				if _, err := usvg.Parse(src); err != nil {
					gotParse = "ERR: " + err.Error()
				}
				if want := readFile(t, file+".parse"); gotParse != want {
					t.Errorf("parse = %q, want %q", gotParse, want)
				}
			})
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// firstDifference shows the first differing line with context around
// the first differing byte.
func firstDifference(got, want string) string {
	gl, wl := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := range min(len(gl), len(wl)) {
		if gl[i] == wl[i] {
			continue
		}
		g, w := gl[i], wl[i]
		j := 0
		for j < len(g) && j < len(w) && g[j] == w[j] {
			j++
		}
		lo := max(0, j-40)
		return fmt.Sprintf("line %d:\n got  ...%s\n want ...%s", i+1, g[lo:min(len(g), j+40)], w[lo:min(len(w), j+40)])
	}
	return fmt.Sprintf("%d lines, want %d", len(gl), len(wl))
}

func TestTheStrokeSourceDrawsWhatTheOutlinesDraw(t *testing.T) {
	const size = 64
	goldens, err := filepath.Glob(filepath.Join("testdata", "symbolic", "*.svg.golden"))
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, golden := range goldens {
		icon := readFile(t, golden)
		open, end := strings.Index(icon, "<metadata>"), strings.Index(icon, "</metadata>")
		if open < 0 {
			if strings.Contains(icon, "stroke='rgb(0,0,0)'") {
				t.Errorf("%s: a stroke without its metadata source", golden)
			}
			continue
		}
		checked++
		outlines, err := render.LoadSVG([]byte(icon), size, size)
		if err != nil {
			t.Fatalf("%s: outlines: %v", golden, err)
		}
		source, err := render.LoadSVG([]byte(icon[open+len("<metadata>"):end]), size, size)
		if err != nil {
			t.Fatalf("%s: stroke source: %v", golden, err)
		}
		total := 0
		for i, p := range outlines.Image().Pix {
			d := int(p) - int(source.Image().Pix[i])
			total += max(d, -d)
		}
		if mean := float64(total) / float64(len(outlines.Image().Pix)); mean > 1 {
			t.Errorf("%s: the stroke source and the outlines differ by %.2f per channel on average", golden, mean)
		}
	}
	if checked == 0 {
		t.Fatal("premise: no golden carries a stroke source")
	}
}
