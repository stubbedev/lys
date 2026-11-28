package xdg

import "testing"

func TestExpandHome(t *testing.T) {
	for path, want := range map[string]string{
		"~":            "/home/me",
		"~/":           "/home/me/",
		"~/docs/a.txt": "/home/me/docs/a.txt",
		"~/docs/":      "/home/me/docs/",
		"~foo":         "~foo",
		"~foo/bar":     "~foo/bar",
		"/abs/~/x":     "/abs/~/x",
		"rel":          "rel",
		"":             "",
	} {
		if got := ExpandHome(path, "/home/me"); got != want {
			t.Errorf("ExpandHome(%q) = %q, want %q", path, got, want)
		}
	}
	if got := ExpandHome("~/x", "/home/me/"); got != "/home/me/x" {
		t.Errorf("a home with a trailing slash expanded ~/x to %q", got)
	}
	if got := ExpandHome("~/x", "/"); got != "/x" {
		t.Errorf("home / expanded ~/x to %q", got)
	}
	if got := ExpandHome("~/x", ""); got != "~/x" {
		t.Errorf("no home expanded ~/x to %q", got)
	}
}

func TestCollapseHome(t *testing.T) {
	for path, want := range map[string]string{
		"/home/me":        "~",
		"/home/me/":       "~/",
		"/home/me/x/y":    "~/x/y",
		"/home/me2/x":     "/home/me2/x",
		"/home/mex":       "/home/mex",
		"/srv/home/me/x":  "/srv/home/me/x",
		"/home":           "/home",
		"relative/me/abc": "relative/me/abc",
	} {
		if got := CollapseHome(path, "/home/me"); got != want {
			t.Errorf("CollapseHome(%q) = %q, want %q", path, got, want)
		}
	}
	if got := CollapseHome("/home/me/x", "/home/me/"); got != "~/x" {
		t.Errorf("a home with a trailing slash collapsed /home/me/x to %q", got)
	}
	for _, home := range []string{"", "/"} {
		if got := CollapseHome("/x", home); got != "/x" {
			t.Errorf("home %q collapsed /x to %q", home, got)
		}
	}
}

func TestExpandAndCollapseRoundTrip(t *testing.T) {
	for _, path := range []string{"~", "~/a", "~/a/b/"} {
		if got := CollapseHome(ExpandHome(path, "/home/me"), "/home/me"); got != path {
			t.Errorf("%q round-tripped to %q", path, got)
		}
	}
}
