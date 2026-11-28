package apptheme

import (
	"errors"
	"slices"
	"testing"
)

func TestAnAppsIconsJoinTheSearchPathFromItsShareDirectory(t *testing.T) {
	t.Parallel()
	f := &fakeIconSearch{paths: []string{"/usr/share/icons"}}
	shipIcons("lys-files", func() (string, error) { return "/opt/lys/bin/lys-files", nil }, f.search())
	if want := []string{"/usr/share/icons", "/opt/lys/share/lys-files/icons"}; !slices.Equal(f.paths, want) {
		t.Errorf("the search path is %q, want %q", f.paths, want)
	}
}

func TestAnAppThatCannotFindItselfKeepsTheSearchPath(t *testing.T) {
	t.Parallel()
	f := &fakeIconSearch{paths: []string{"/usr/share/icons"}}
	shipIcons("lys-files", func() (string, error) { return "", errors.New("no /proc") }, f.search())
	if want := []string{"/usr/share/icons"}; !slices.Equal(f.paths, want) {
		t.Errorf("the search path is %q, want %q", f.paths, want)
	}
}
