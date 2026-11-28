package fileops

import (
	"strings"
	"testing"
)

func TestPlanTransfer(t *testing.T) {
	file := func(dev uint64, size int64) node {
		return node{name: "f", kind: regular, size: size, vol: volume{dev: dev}}
	}
	folder := node{name: "b", kind: directory, vol: volume{dev: 2}, children: []node{{name: "c", kind: regular, size: 5, vol: volume{dev: 2}}}}
	dirs := map[string]listing{"/dst": {vol: volume{dev: 1}, names: map[string]bool{"x": true}}}
	reqs := []request{
		{src: "/src/a", node: file(1, 10), target: "/dst/a"},
		{src: "/src/b", node: folder, target: "/dst/b"},
		{src: "/src/c", node: file(1, 1), target: "/dst/x"},
		{src: "/other/a", node: file(1, 3), target: "/dst/a"},
	}

	moved, err := planTransfer(moving, reqs, dirs)
	if err != nil {
		t.Fatal(err)
	}
	type view struct {
		src, dst                      string
		rename, conflict, targetIsDir bool
	}
	want := []view{
		{src: "/src/a", dst: "/dst/a", rename: true},
		{src: "/src/b", dst: "/dst/b"},
		{src: "/src/c", dst: "/dst/x", rename: true, conflict: true, targetIsDir: true},
		{src: "/other/a", dst: "/dst/a", rename: true, conflict: true},
	}
	if len(moved.items) != len(want) {
		t.Fatalf("a move plans %d items, want %d", len(moved.items), len(want))
	}
	for i, it := range moved.items {
		if got := (view{it.src, it.dst, it.rename, it.conflict, it.targetIsDir}); got != want[i] {
			t.Errorf("move item %d = %+v, want %+v", i, got, want[i])
		}
	}
	for i, it := range moved.items {
		if it.replace {
			t.Errorf("move item %d replaces before any conflict is answered", i)
		}
	}
	if moved.files != 5 || moved.bytes != 5 {
		t.Errorf("a move weighs %d files, %d bytes; want 5 files and only the copied folder's 5 bytes", moved.files, moved.bytes)
	}
	if len(dirs["/dst"].names) != 1 {
		t.Errorf("planning changed the listing it was given: %v", dirs["/dst"].names)
	}

	copied, err := planTransfer(copying, reqs, dirs)
	if err != nil {
		t.Fatal(err)
	}
	for i, it := range copied.items {
		if it.rename {
			t.Errorf("copy item %d is a rename", i)
		}
	}
	if copied.bytes != 19 {
		t.Errorf("a copy weighs %d bytes, want 19", copied.bytes)
	}
}

func TestPlanTransferOntoItself(t *testing.T) {
	dirs := map[string]listing{"/d": {vol: volume{dev: 1}, names: map[string]bool{"a.txt": false, "a (2).txt": false}}}
	reqs := []request{{src: "/d/a.txt", node: node{name: "a.txt", kind: regular, vol: volume{dev: 1}}, target: "/d/a.txt"}}
	p, err := planTransfer(copying, reqs, dirs)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.items) != 1 || p.items[0].dst != "/d/a (3).txt" || p.items[0].conflict {
		t.Errorf("copying a file onto itself plans %+v, want a copy beside it under a free name", p.items)
	}
	p, err = planTransfer(moving, reqs, dirs)
	if err != nil || len(p.items) != 0 {
		t.Errorf("moving a file onto itself plans %+v, %v; want nothing to do", p.items, err)
	}
}

func TestPlanTransferRefuses(t *testing.T) {
	folder := node{name: "b", kind: directory, vol: volume{dev: 1}}
	for name, req := range map[string]request{
		"into itself": {src: "/src/b", node: folder, target: "/src/b/sub/b"},
		"unlisted":    {src: "/src/b", node: folder, target: "/elsewhere/b"},
	} {
		_, err := planTransfer(copying, []request{req}, map[string]listing{"/src/b/sub": {vol: volume{dev: 1}}})
		if err == nil || !strings.Contains(err.Error(), "/src/b") {
			t.Errorf("%s: err = %v, want an error naming the source", name, err)
		}
	}
}

func TestResolve(t *testing.T) {
	taken := map[string]bool{"a.txt": false, "a (2).txt": false}
	it := item{src: "/s/a.txt", dst: "/d/a.txt", node: node{kind: regular}, conflict: true}
	if _, ok, err := resolve(it, Skip, taken); ok || err != nil {
		t.Errorf("skip = %v, %v; want the item dropped", ok, err)
	}
	if got, ok, err := resolve(it, Replace, taken); !ok || err != nil || !got.replace || got.dst != it.dst {
		t.Errorf("replace = %+v, %v, %v", got, ok, err)
	}
	if got, ok, err := resolve(it, KeepBoth, taken); !ok || err != nil || got.replace || got.conflict || got.dst != "/d/a (3).txt" {
		t.Errorf("keep both = %+v, %v, %v", got, ok, err)
	}
	if _, _, err := resolve(item{src: "/d/b/b", dst: "/d/b", node: node{kind: directory}}, Replace, taken); err == nil {
		t.Error("replacing the folder the source lies in was allowed")
	}
	if _, _, err := resolve(it, Action(0), taken); err == nil {
		t.Error("an unknown answer was taken")
	}
}

func TestFreeName(t *testing.T) {
	taken := map[string]bool{"x (2).txt": false, "Photos (2)": true}
	for _, c := range []struct {
		name  string
		isDir bool
		want  string
	}{
		{"a.txt", false, "a (2).txt"},
		{"x.txt", false, "x (3).txt"},
		{"x (2).txt", false, "x (3).txt"},
		{"archive.tar.gz", false, "archive.tar (2).gz"},
		{".bashrc", false, ".bashrc (2)"},
		{"Photos", true, "Photos (3)"},
		{"v1.2", true, "v1.2 (2)"},
		{"odd (x)", false, "odd (x) (2)"},
	} {
		if got := freeName(c.name, c.isDir, taken); got != c.want {
			t.Errorf("freeName(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
