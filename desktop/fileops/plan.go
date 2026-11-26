package fileops

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"strconv"
	"strings"
)

// transfer is what a plan does with its sources.
type transfer uint8

const (
	copying transfer = iota + 1
	moving
)

func (t transfer) verb() string {
	if t == moving {
		return "move"
	}
	return "copy"
}

// request asks for the tree scanned at src to end up at target.
type request struct {
	src    string
	node   node
	target string
}

// item is one source's step: renamed in place when rename is set,
// else copied (then removed for a move) to dst. conflict is set when
// dst names something already there, a folder when targetIsDir.
type item struct {
	src, dst    string
	node        node
	rename      bool
	conflict    bool
	targetIsDir bool
	replace     bool
}

// plan is what a transfer will do, item by item, what it weighs, and
// the names each destination folder holds once it is done.
type plan struct {
	op    transfer
	items []item
	files int64
	bytes int64
	taken map[string]map[string]bool
}

// planTransfer plans reqs against the destination folders' listings:
// a move within one filesystem is a rename, any other transfer a copy;
// a target already taken, by the folder or by an earlier request, is a
// conflict; copying a thing onto itself keeps both and moving it onto
// itself does nothing. A folder put inside itself is an error.
func planTransfer(op transfer, reqs []request, dirs map[string]listing) (plan, error) {
	p := plan{op: op, items: make([]item, 0, len(reqs)), taken: make(map[string]map[string]bool, len(dirs))}
	for dir, l := range dirs {
		names := make(map[string]bool, len(l.names)+len(reqs))
		maps.Copy(names, l.names)
		p.taken[dir] = names
	}
	for _, r := range reqs {
		src, dst := filepath.Clean(r.src), filepath.Clean(r.target)
		if r.node.isDir() && dst != src && within(dst, src) {
			return plan{}, fmt.Errorf("%s %s into itself", op.verb(), src)
		}
		dir, name := filepath.Dir(dst), filepath.Base(dst)
		l, ok := dirs[dir]
		if !ok {
			return plan{}, fmt.Errorf("%s %s: the folder %s was not listed", op.verb(), src, dir)
		}
		taken := p.taken[dir]
		it := item{src: src, dst: dst, node: r.node, rename: op == moving && l.dev == r.node.dev}
		switch isDir, exists := taken[name]; {
		case dst == src && op == moving:
			continue
		case dst == src:
			it.dst = filepath.Join(dir, freeName(name, r.node.isDir(), taken))
		case exists:
			it.conflict, it.targetIsDir = true, isDir
		}
		taken[filepath.Base(it.dst)] = r.node.isDir()
		files, bytes := r.node.count()
		p.files += files
		if !it.rename {
			p.bytes += bytes
		}
		p.items = append(p.items, it)
	}
	return p, nil
}

// within reports whether path is dir or lies under it.
func within(path, dir string) bool {
	return path == dir || strings.HasPrefix(path, strings.TrimSuffix(dir, "/")+"/")
}

// resolve applies a conflict's answer to its item: false when it is
// skipped, a free name beside the taken one to keep both, or the item
// replacing what is there. Replacing a folder the source lies in would
// destroy the source, and is an error.
func resolve(it item, a Action, taken map[string]bool) (item, bool, error) {
	switch a {
	case Skip:
		return item{}, false, nil
	case Replace:
		if within(it.src, it.dst) {
			return item{}, false, fmt.Errorf("replace %s with %s inside it", it.dst, it.src)
		}
		it.replace = true
		return it, true, nil
	case KeepBoth:
		name := freeName(filepath.Base(it.dst), it.node.isDir(), taken)
		it.dst, it.conflict, it.targetIsDir = filepath.Join(filepath.Dir(it.dst), name), false, false
		return it, true, nil
	}
	return item{}, false, errors.New("the conflict was answered with no known action")
}

// freeName is the first of "stem (2)ext", "stem (3)ext"… not taken,
// where a file's ext is its last dot suffix and a stem's own " (n)" is
// counted on from rather than nested.
func freeName(name string, isDir bool, taken map[string]bool) string {
	stem, ext := name, ""
	if i := strings.LastIndexByte(name, '.'); !isDir && i > 0 {
		stem, ext = name[:i], name[i:]
	}
	if open := strings.LastIndex(stem, " ("); open > 0 && strings.HasSuffix(stem, ")") {
		if _, err := strconv.ParseUint(stem[open+2:len(stem)-1], 10, 32); err == nil {
			stem = stem[:open]
		}
	}
	for n := 2; ; n++ {
		candidate := stem + " (" + strconv.Itoa(n) + ")" + ext
		if _, ok := taken[candidate]; !ok {
			return candidate
		}
	}
}
