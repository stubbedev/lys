package fileops

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"golang.org/x/sys/unix"
)

type kind uint8

const (
	regular kind = iota + 1
	directory
	symlink
)

// node is what a scan found at a path: a regular file, a folder with
// its children in name order, or a symbolic link and its target.
type node struct {
	name     string
	kind     kind
	perm     uint32
	size     int64
	mtime    unix.Timespec
	dev      uint64
	target   string
	children []node
}

func (n node) isDir() bool { return n.kind == directory }

// count is what n weighs in a job's progress: its files, itself
// included, and the bytes of the regular ones.
func (n node) count() (files, bytes int64) {
	files = 1
	if n.kind == regular {
		bytes = n.size
	}
	for _, c := range n.children {
		f, b := c.count()
		files += f
		bytes += b
	}
	return files, bytes
}

type inode struct{ dev, ino uint64 }

// scan reads the tree at path without following links, or following
// every one when follow is set; a link that leads back into a folder
// being scanned is an error, as is a device, socket or pipe.
func scan(path string, follow bool) (node, error) {
	return scanAt(path, follow, nil)
}

func scanAt(path string, follow bool, open []inode) (node, error) {
	var st unix.Stat_t
	stat := unix.Lstat
	if follow {
		stat = unix.Stat
	}
	if err := stat(path, &st); err != nil {
		return node{}, &os.PathError{Op: "stat", Path: path, Err: err}
	}
	n := node{
		name: filepath.Base(path), perm: st.Mode & 0o7777, size: st.Size,
		mtime: st.Mtim, dev: st.Dev,
	}
	switch st.Mode & unix.S_IFMT {
	case unix.S_IFREG:
		n.kind = regular
	case unix.S_IFLNK:
		target, err := os.Readlink(path)
		if err != nil {
			return node{}, err
		}
		n.kind, n.target = symlink, target
	case unix.S_IFDIR:
		n.kind = directory
		self := inode{st.Dev, st.Ino}
		if slices.Contains(open, self) {
			return node{}, fmt.Errorf("%s leads back into a folder that contains it", path)
		}
		inner := append(slices.Clip(open), self)
		des, err := os.ReadDir(path)
		if err != nil {
			return node{}, err
		}
		n.children = make([]node, 0, len(des))
		for _, de := range des {
			c, err := scanAt(filepath.Join(path, de.Name()), follow, inner)
			if err != nil {
				return node{}, err
			}
			n.children = append(n.children, c)
		}
	default:
		return node{}, fmt.Errorf("%s is not a file, a folder or a link", path)
	}
	return n, nil
}

// listing is a folder that things are put in: its filesystem and what
// it holds, each name mapped to whether it is a folder.
type listing struct {
	dev   uint64
	names map[string]bool
}

func list(dir string) (listing, error) {
	var st unix.Stat_t
	if err := unix.Stat(dir, &st); err != nil {
		return listing{}, &os.PathError{Op: "stat", Path: dir, Err: err}
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		return listing{}, fmt.Errorf("%s is not a folder", dir)
	}
	des, err := os.ReadDir(dir)
	if err != nil {
		return listing{}, err
	}
	l := listing{dev: st.Dev, names: make(map[string]bool, len(des))}
	for _, de := range des {
		l.names[de.Name()] = de.IsDir()
	}
	return l, nil
}
