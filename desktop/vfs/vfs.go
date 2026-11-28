// Package vfs is the virtual filesystem the file browser and the file
// operations work through: the local disk is one filesystem, each
// connected network location or device another, addressed by a
// location string, a local path or scheme://[user@]host[:port]/path.
// Every filesystem lists a folder, states an entry, reads a file,
// creates one, makes a folder, renames and removes, each under a
// context and failing with the errors of io/fs and this package.
package vfs

import (
	"context"
	"io"
	"time"
)

// Info is what a listing or a stat knows of one entry. A link is
// what it leads to, Link set; a link that leads nowhere is an entry
// with nothing known of it but that.
type Info struct {
	Name     string
	IsDir    bool
	Link     bool
	Size     int64
	Modified time.Time
}

// FS is one filesystem. Its paths are absolute and slash-separated,
// "/" its root. A missing entry fails with fs.ErrNotExist, a taken name
// with fs.ErrExist, a refusal with fs.ErrPermission, and what the
// filesystem cannot do with ErrUnsupported, each wrapped in an
// *fs.PathError naming the path.
type FS interface {
	// ReadDir lists the folder dir, in no particular order.
	ReadDir(ctx context.Context, dir string) ([]Info, error)
	// Stat states path.
	Stat(ctx context.Context, path string) (Info, error)
	// Open opens the file path for reading. A file that can be read at
	// any offset is an io.ReaderAt and an io.Seeker too.
	Open(ctx context.Context, path string) (File, error)
	// Create makes the file path, which must not exist yet, for writing:
	// it is whole once Close returns no error.
	Create(ctx context.Context, path string) (io.WriteCloser, error)
	// Mkdir makes the folder path, which must not exist yet.
	Mkdir(ctx context.Context, path string) error
	// Rename gives from the name to, never over something already
	// there.
	Rename(ctx context.Context, from, to string) error
	// Remove removes the file, or the empty folder, path.
	Remove(ctx context.Context, path string) error
}

// File is a file open for reading.
type File interface {
	io.Reader
	io.Closer
}

// Conn is a filesystem behind a connection: a network location or a
// device, closed when it is disconnected.
type Conn interface {
	FS
	Close() error
}
