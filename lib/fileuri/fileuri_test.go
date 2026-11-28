package fileuri

import "testing"

func TestPath(t *testing.T) {
	for uri, want := range map[string]string{
		"file:///home/u/bg.png":    "/home/u/bg.png",
		"file://localhost/srv/a":   "/srv/a",
		"file://LocalHost/srv/a":   "/srv/a",
		"file:/srv/a":              "/srv/a",
		"file:///srv/a?x=1#frag":   "/srv/a",
		"file:///a%2":              "/a%2",
		"file:///a%zz":             "/a%zz",
		"file:///%E2%9C%93%ff.png": "/✓�.png",
		"file:///trailing%41":      "/trailingA",
		"file:///My%20Walls/a%23b": "/My Walls/a#b",
	} {
		if got, ok := Path(uri); !ok || got != want {
			t.Errorf("Path(%q) = %q, %v; want %q", uri, got, ok, want)
		}
	}
	for _, uri := range []string{"https://x/y", "file://host", "/plain", "file://otherhost/etc/passwd", "file:relative", "file://", "file:///a%00b", "File:///a"} {
		if _, ok := Path(uri); ok {
			t.Errorf("%q read as a path", uri)
		}
	}
}

func TestPathOf(t *testing.T) {
	for s, want := range map[string]string{
		"file:///My%20Pictures/a.png": "/My Pictures/a.png",
		"/My%20Pictures/a.png":        "/My%20Pictures/a.png",
		"img/a.png":                   "img/a.png",
	} {
		if got, ok := PathOf(s); !ok || got != want {
			t.Errorf("PathOf(%q) = %q, %v; want %q", s, got, ok, want)
		}
	}
	for _, s := range []string{"", "file://otherhost/a.png", "file:a.png"} {
		if got, ok := PathOf(s); ok {
			t.Errorf("PathOf(%q) = %q, want no path", s, got)
		}
	}
}

func TestEncodeRoundTrips(t *testing.T) {
	if got := FromPath("/home/u/Shot 1#.png"); got != "file:///home/u/Shot%201%23.png" {
		t.Errorf("FromPath = %q", got)
	}
	if got := Encode("Line & stuff~", ""); got != "Line%20%26%20stuff~" {
		t.Errorf("Encode = %q", got)
	}
	for _, s := range []string{"a b/c#d%e", "✓"} {
		if got := DecodeString(Encode(s, "")); got != s {
			t.Errorf("round trip %q = %q", s, got)
		}
	}
}

func TestFromPathEscapesLikeGlib(t *testing.T) {
	if got := FromPath("/tmp/a b/ü#?.png"); got != "file:///tmp/a%20b/%C3%BC%23%3F.png" {
		t.Errorf("got %s", got)
	}
	if got := FromPath("/x/!$&'()*+,-.:=@_~"); got != "file:///x/!$&'()*+,-.:=@_~" {
		t.Errorf("safe set escaped: %s", got)
	}
}
