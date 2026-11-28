package vfs

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"time"
	"unsafe"

	"github.com/stubbedev/memory-go"
	"github.com/stubbedev/memory-go/bufpool"
	"golang.org/x/sys/unix"
)

// The listing's buffers: getdents64 reads into dirBufSize, about 3,000
// names a call, and the names are written nameChunk at a time.
const (
	dirBufSize = 128 << 10
	nameChunk  = 128 << 10
)

// infosStart is a listing's first capacity; it doubles from there,
// where append would grow a large listing by a quarter at a time.
const infosStart = 512

// linux_dirent64: d_ino, d_off, then d_reclen, d_type and the name.
const (
	direntReclen = 16
	direntType   = 18
	direntName   = 19
)

// LocalFS is the local disk. A folder is listed through one directory
// fd, getdents64 into a pooled off-heap buffer, and every entry stated
// relative to that fd straight from the name in the buffer: no path is
// walked and no string is made per entry, the names being written into
// shared chunks.
type LocalFS struct {
	mem  memory.Shared
	bufs *bufpool.Pool
}

// NewLocal is the local disk's filesystem.
func NewLocal() *LocalFS {
	l := &LocalFS{}
	bufs, err := bufpool.New(&l.mem, 4*dirBufSize)
	if err != nil {
		panic(fmt.Sprintf("vfs: a dirent pool retaining %d bytes: %v", 4*dirBufSize, err))
	}
	l.bufs = bufs
	return l
}

// DirCounts are the system calls a listing made.
type DirCounts struct {
	Getdents, Stats int
}

// ReadDir lists dir; an entry that cannot be stated (a dangling link)
// is listed with nothing known of it.
func (l *LocalFS) ReadDir(_ context.Context, dir string) ([]Info, error) {
	infos, _, err := l.ReadDirCounted(dir)
	return infos, err
}

// ReadDirCounted is ReadDir with the system calls it made.
func (l *LocalFS) ReadDirCounted(dir string) ([]Info, DirCounts, error) {
	var counts DirCounts
	var names nameArena
	var infos []Info
	var st unix.Stat_t
	n, err := l.scan(dir, func(fd int, name []byte, cname *byte, link bool) bool {
		counts.Stats++
		info := Info{Name: names.add(name)}
		if statAt(fd, cname, &st) == nil {
			info = infoOf(info.Name, &st)
		}
		info.Link = link
		if len(infos) == cap(infos) {
			infos = slices.Grow(infos, max(len(infos), infosStart))
		}
		infos = append(infos, info)
		return true
	})
	counts.Getdents = n
	if err != nil {
		return nil, counts, err
	}
	return infos, counts, nil
}

// scan opens dir and hands visit each entry but . and ..: the
// directory fd, the name, the name NUL-terminated in the dirent buffer,
// which stays valid only during the call, and whether it is a link. A
// false from visit stops the scan. It reports the getdents64 calls
// made.
func (l *LocalFS) scan(dir string, visit func(fd int, name []byte, cname *byte, link bool) bool) (int, error) {
	fd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return 0, &fs.PathError{Op: "open", Path: dir, Err: err}
	}
	defer func() { _ = unix.Close(fd) }()
	buf, err := l.bufs.Get(dirBufSize)
	if err != nil {
		return 0, fmt.Errorf("reading %s: a %d byte buffer: %w", dir, dirBufSize, err)
	}
	defer func() { _ = l.bufs.Put(buf) }()
	b := buf.Bytes()
	calls := 0
	for {
		n, err := unix.Getdents(fd, b)
		calls++
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return calls, &fs.PathError{Op: "getdents", Path: dir, Err: err}
		}
		if n == 0 {
			return calls, nil
		}
		for off := 0; off < n; {
			reclen := int(binary.NativeEndian.Uint16(b[off+direntReclen:]))
			name := b[off+direntName : off+reclen]
			name = name[:bytes.IndexByte(name, 0)]
			at, link := off+direntName, b[off+direntType] == unix.DT_LNK
			off += reclen
			if string(name) == "." || string(name) == ".." {
				continue
			}
			if !visit(fd, name, &b[at], link) {
				return calls, nil
			}
		}
	}
}

// statAt stats the NUL-terminated name at name relative to the
// directory fd, following a link as os.Stat does: newfstatat without a
// path conversion, so a listing allocates nothing per entry.
func statAt(dirfd int, name *byte, st *unix.Stat_t) error {
	_, _, errno := unix.Syscall6(unix.SYS_NEWFSTATAT, uintptr(dirfd), uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(st)), 0, 0, 0) //nolint:gosec // the name lives in the off-heap dirent buffer, the stat on the stack
	if errno != 0 {
		return errno
	}
	return nil
}

// nameArena writes a listing's names into shared chunks on the Go heap
// and hands out strings over them: a chunk is only ever appended to,
// and the strings keep it alive for as long as any holds one.
type nameArena struct {
	chunk []byte
}

func (a *nameArena) add(name []byte) string {
	if len(name) == 0 {
		return ""
	}
	if cap(a.chunk)-len(a.chunk) < len(name) {
		a.chunk = make([]byte, 0, max(nameChunk, len(name)))
	}
	start := len(a.chunk)
	a.chunk = append(a.chunk, name...)
	return unsafe.String(&a.chunk[start], len(name)) //nolint:gosec // the chunk is never written below its length again
}

// infoOf is the entry name as st describes it.
func infoOf(name string, st *unix.Stat_t) Info {
	return Info{
		Name: name, IsDir: st.Mode&unix.S_IFMT == unix.S_IFDIR,
		Size: st.Size, Modified: time.Unix(st.Mtim.Sec, st.Mtim.Nsec),
	}
}

// Stat states path, following a link; a link that leads nowhere is an
// entry with nothing known of it.
func (l *LocalFS) Stat(_ context.Context, p string) (Info, error) {
	var st unix.Stat_t
	name := baseName(p)
	if err := unix.Lstat(p, &st); err != nil {
		return Info{}, &fs.PathError{Op: "stat", Path: p, Err: err}
	}
	if st.Mode&unix.S_IFMT != unix.S_IFLNK {
		return infoOf(name, &st), nil
	}
	if err := unix.Stat(p, &st); err != nil {
		return Info{Name: name, Link: true}, nil
	}
	info := infoOf(name, &st)
	info.Link = true
	return info, nil
}

// baseName is the last element of a local path.
func baseName(p string) string {
	for i := len(p) - 1; i > 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}

// Open opens path for reading, as an *os.File: a reader at any offset.
func (l *LocalFS) Open(_ context.Context, p string) (File, error) {
	return os.Open(p) //nolint:gosec // a file the user browses to
}

// Create makes path, which must not exist yet.
func (l *LocalFS) Create(_ context.Context, p string) (io.WriteCloser, error) {
	return os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666) //nolint:gosec // a file the user asked for, as the umask allows
}

// Mkdir makes the folder path.
func (l *LocalFS) Mkdir(_ context.Context, p string) error {
	return os.Mkdir(p, 0o777) //nolint:gosec // a folder the user asked for, as the umask allows
}

// Rename renames from to to, never over something there.
func (l *LocalFS) Rename(_ context.Context, from, to string) error {
	if err := unix.Renameat2(unix.AT_FDCWD, from, unix.AT_FDCWD, to, unix.RENAME_NOREPLACE); err != nil {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: err}
	}
	return nil
}

// Remove removes the file, link or empty folder path.
func (l *LocalFS) Remove(_ context.Context, p string) error {
	return os.Remove(p)
}
