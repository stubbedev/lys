package argpaths

import (
	"slices"
	"testing"
)

func TestParseReadsPathsAndFileURIs(t *testing.T) {
	got, err := Parse([]string{"a.png", "../b/c.txt", "/x//y/", "file:///tmp/a%20b.pdf", "file://localhost/srv/d", "file:/e"}, "/home/u/pics")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/home/u/pics/a.png", "/home/u/b/c.txt", "/x/y", "/tmp/a b.pdf", "/srv/d", "/e"}
	if !slices.Equal(got, want) {
		t.Errorf("Parse = %v, want %v", got, want)
	}
	if got, err := Parse(nil, "/"); err != nil || len(got) != 0 {
		t.Errorf("Parse of no argument = %v, %v", got, err)
	}
}

func TestParseRefusesWhatNamesNoLocalFile(t *testing.T) {
	for _, arg := range []string{"", "https://example.com/a.png", "file://host/a", "sftp://host/a", "file:"} {
		if got, err := Parse([]string{"ok.txt", arg}, "/tmp"); err == nil {
			t.Errorf("Parse(%q) = %v, want an error", arg, got)
		}
	}
}
