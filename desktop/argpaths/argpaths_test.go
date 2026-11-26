package argpaths

import (
	"slices"
	"strings"
	"testing"
)

func TestParseReadsPathsAndFileURIs(t *testing.T) {
	got, err := Parse([]string{"a.png", "./q/../r.png", "../b/c.txt", "/x//y/", "file:///tmp/a%20b.pdf", "file://localhost/srv/d", "file:/e"}, "/home/u/pics")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/home/u/pics/a.png", "/home/u/pics/r.png", "/home/u/b/c.txt", "/x/y", "/tmp/a b.pdf", "/srv/d", "/e"}
	if !slices.Equal(got, want) {
		t.Errorf("Parse = %v, want %v", got, want)
	}
	if got, err := Parse(nil, "/"); err != nil || len(got) != 0 {
		t.Errorf("Parse of no argument = %v, %v", got, err)
	}
}

func TestParseRefusesWhatNamesNoLocalFile(t *testing.T) {
	for _, arg := range []string{"", "https://example.com/a.png", "file://host/a", "sftp://host/a", "file:"} {
		got, err := Parse([]string{"ok.txt", arg}, "/tmp")
		if err == nil {
			t.Errorf("Parse(%q) = %v, want an error", arg, got)
			continue
		}
		if arg != "" && !strings.Contains(err.Error(), arg) {
			t.Errorf("the error for %q does not name it: %v", arg, err)
		}
	}
}
