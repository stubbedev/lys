package configdocs

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCommittedPagesAreCurrent(t *testing.T) {
	dir := t.TempDir()
	if err := Generate(dir); err != nil {
		t.Fatal(err)
	}
	n := 0
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		n++
		rel, _ := filepath.Rel(dir, path)
		got, _ := os.ReadFile(path)
		want, err := os.ReadFile(filepath.Join("../../../docs/shell/config", rel))
		if err != nil {
			t.Errorf("%s: not in docs/shell/config", rel)
			return nil
		}
		if string(got) != string(want) {
			t.Errorf("%s differs from docs/shell/config:\n%s", rel, firstDiff(string(want), string(got)))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n < 50 {
		t.Errorf("generated %d pages", n)
	}
}

// firstDiff shows the first differing line with its neighbours.
func firstDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := range max(len(w), len(g)) {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			return "line " + strconv.Itoa(i+1) + "\n  want: " + wl + "\n  got:  " + gl
		}
	}
	return "(trailing difference)"
}
