// Package mtpfs is a phone or a camera plugged in over USB, through
// desktop/mtp's MTP and PTP session, behind vfs.FS: the root lists the
// device's stores, each a folder holding its objects, found by name
// from the top of their store.
package mtpfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/mtp"
	"github.com/stubbedev/lys/desktop/mtp/ptp"
	"github.com/stubbedev/lys/desktop/mtp/usb"
	"github.com/stubbedev/lys/desktop/vfs"
)

// session is what a device answers: desktop/mtp's Session.
type session interface {
	Storages(ctx context.Context) ([]mtp.Storage, error)
	List(ctx context.Context, storage ptp.StorageID, parent ptp.ObjectHandle) ([]mtp.Object, error)
	OpenReader(ctx context.Context, h ptp.ObjectHandle) (*mtp.Reader, error)
	Download(ctx context.Context, h ptp.ObjectHandle, w io.Writer) error
	Upload(ctx context.Context, storage ptp.StorageID, parent ptp.ObjectHandle, name string, size int64, modTime time.Time, content io.Reader) (mtp.Object, error)
	Mkdir(ctx context.Context, storage ptp.StorageID, parent ptp.ObjectHandle, name string) (mtp.Object, error)
	Delete(ctx context.Context, h ptp.ObjectHandle) error
	Rename(ctx context.Context, h ptp.ObjectHandle, name string) error
	Move(ctx context.Context, h ptp.ObjectHandle, storage ptp.StorageID, parent ptp.ObjectHandle) error
	Close() error
}

// New is the filesystem of the device s has a session with, which it
// then owns. A device whose stores stay closed (a locked phone) is
// vfs.ErrLocked.
func New(ctx context.Context, s *mtp.Session) (vfs.Conn, error) { return newConn(ctx, s) }

func newConn(ctx context.Context, s session) (*conn, error) {
	if _, err := s.Storages(ctx); err != nil {
		return nil, errors.Join(lockedError(err), s.Close())
	}
	return &conn{s: s}, nil
}

// lockedError is err, typed vfs.ErrLocked too when the device is locked.
func lockedError(err error) error {
	if errors.Is(err, mtp.ErrLocked) {
		return fmt.Errorf("%w: %w", vfs.ErrLocked, err)
	}
	return err
}

// conn is a device's session.
type conn struct{ s session }

// Close ends the session and releases the device.
func (c *conn) Close() error { return c.s.Close() }

// storeName is how a store shows as a folder of the root: its
// description, else its label, else its number.
func storeName(st mtp.Storage) string {
	name := strings.ReplaceAll(cmpOr(cmpOr(st.Description, st.Label), "Storage "+strconv.FormatUint(uint64(st.ID), 16)), "/", "-")
	return name
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// place is where a path is on the device: its store, and its object,
// ptp.Root for the store itself.
type place struct {
	storage ptp.StorageID
	obj     mtp.Object
	store   bool
}

// find is the place p names, walked by name from its store's top.
func (c *conn) find(ctx context.Context, op, p string) (place, error) {
	parts := strings.Split(strings.Trim(path.Clean(p), "/"), "/")
	if parts[0] == "" {
		return place{}, &fs.PathError{Op: op, Path: p, Err: vfs.ErrUnsupported}
	}
	stores, err := c.s.Storages(ctx)
	if err != nil {
		return place{}, &fs.PathError{Op: op, Path: p, Err: err}
	}
	pl := place{store: true}
	found := false
	for _, st := range stores {
		if storeName(st) == parts[0] {
			pl.storage, found = st.ID, true
			break
		}
	}
	if !found {
		return place{}, &fs.PathError{Op: op, Path: p, Err: fs.ErrNotExist}
	}
	parent := ptp.Root
	for _, name := range parts[1:] {
		objs, err := c.s.List(ctx, pl.storage, parent)
		if err != nil {
			return place{}, &fs.PathError{Op: op, Path: p, Err: err}
		}
		found = false
		for _, o := range objs {
			if o.Name == name {
				pl.obj, pl.store, found = o, false, true
				parent = o.Handle
				break
			}
		}
		if !found {
			return place{}, &fs.PathError{Op: op, Path: p, Err: fs.ErrNotExist}
		}
	}
	return pl, nil
}

// handle is the place's object handle, ptp.Root for a store.
func (pl place) handle() ptp.ObjectHandle {
	if pl.store {
		return ptp.Root
	}
	return pl.obj.Handle
}

func (pl place) dir() bool { return pl.store || pl.obj.Dir }

func infoOf(o mtp.Object) vfs.Info {
	return vfs.Info{Name: o.Name, IsDir: o.Dir, Size: o.Size, Modified: o.ModTime}
}

// ReadDir implements vfs.FS: the root lists the stores.
func (c *conn) ReadDir(ctx context.Context, dir string) ([]vfs.Info, error) {
	if path.Clean(dir) == "/" {
		stores, err := c.s.Storages(ctx)
		if err != nil {
			return nil, &fs.PathError{Op: "readdir", Path: dir, Err: err}
		}
		infos := make([]vfs.Info, len(stores))
		for i, st := range stores {
			infos[i] = vfs.Info{Name: storeName(st), IsDir: true}
		}
		return infos, nil
	}
	pl, err := c.find(ctx, "readdir", dir)
	if err != nil {
		return nil, err
	}
	if !pl.dir() {
		return nil, &fs.PathError{Op: "readdir", Path: dir, Err: errors.New("not a folder")}
	}
	objs, err := c.s.List(ctx, pl.storage, pl.handle())
	if err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: dir, Err: err}
	}
	infos := make([]vfs.Info, len(objs))
	for i, o := range objs {
		infos[i] = infoOf(o)
	}
	return infos, nil
}

// Stat implements vfs.FS.
func (c *conn) Stat(ctx context.Context, p string) (vfs.Info, error) {
	if path.Clean(p) == "/" {
		return vfs.Info{Name: "/", IsDir: true}, nil
	}
	pl, err := c.find(ctx, "stat", p)
	if err != nil {
		return vfs.Info{}, err
	}
	if pl.store {
		return vfs.Info{Name: path.Base(p), IsDir: true}, nil
	}
	return infoOf(pl.obj), nil
}

// Open implements vfs.FS: a reader at any offset on a device that reads
// part of an object, else the object downloaded as it is read.
func (c *conn) Open(ctx context.Context, p string) (vfs.File, error) {
	pl, err := c.find(ctx, "open", p)
	if err != nil {
		return nil, err
	}
	if pl.dir() {
		return nil, &fs.PathError{Op: "open", Path: p, Err: errors.New("a folder")}
	}
	r, err := c.s.OpenReader(ctx, pl.obj.Handle)
	if err == nil {
		return reader{r}, nil
	}
	if !errors.Is(err, errors.ErrUnsupported) {
		return nil, &fs.PathError{Op: "open", Path: p, Err: err}
	}
	pr, pw := io.Pipe()
	go func() { _ = pw.CloseWithError(c.s.Download(ctx, pl.obj.Handle, pw)) }()
	return pr, nil
}

type reader struct{ *mtp.Reader }

func (reader) Close() error { return nil }

// split is the place of p's folder and p's name, once nothing is at p.
func (c *conn) split(ctx context.Context, op, p string) (place, string, error) {
	parent, err := c.find(ctx, op, path.Dir(p))
	if err != nil {
		return place{}, "", err
	}
	if !parent.dir() {
		return place{}, "", &fs.PathError{Op: op, Path: p, Err: fs.ErrNotExist}
	}
	if _, err := c.find(ctx, op, p); err == nil {
		return place{}, "", &fs.PathError{Op: op, Path: p, Err: fs.ErrExist}
	}
	return parent, path.Base(p), nil
}

// Create implements vfs.FS: what is written gathers in a temporary
// file, which Close uploads, MTP sending a file whole with its size.
func (c *conn) Create(ctx context.Context, p string) (io.WriteCloser, error) {
	parent, name, err := c.split(ctx, "create", p)
	if err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp("", "lys-mtp-")
	if err != nil {
		return nil, &fs.PathError{Op: "create", Path: p, Err: err}
	}
	return &upload{c: c, ctx: ctx, p: p, parent: parent, name: name, tmp: tmp}, nil
}

type upload struct {
	c      *conn
	ctx    context.Context
	p      string
	parent place
	name   string
	tmp    *os.File
	size   int64
}

func (u *upload) Write(b []byte) (int, error) {
	n, err := u.tmp.Write(b)
	u.size += int64(n)
	return n, err
}

// Close sends the file and drops the temporary one.
func (u *upload) Close() error {
	defer func() {
		_ = u.tmp.Close()
		_ = os.Remove(u.tmp.Name())
	}()
	if _, err := u.tmp.Seek(0, io.SeekStart); err != nil {
		return &fs.PathError{Op: "create", Path: u.p, Err: err}
	}
	_, err := u.c.s.Upload(u.ctx, u.parent.storage, u.parent.handle(), u.name, u.size, time.Now(), u.tmp)
	if err != nil {
		return &fs.PathError{Op: "create", Path: u.p, Err: err}
	}
	return nil
}

// Mkdir implements vfs.FS.
func (c *conn) Mkdir(ctx context.Context, p string) error {
	parent, name, err := c.split(ctx, "mkdir", p)
	if err != nil {
		return err
	}
	if _, err := c.s.Mkdir(ctx, parent.storage, parent.handle(), name); err != nil {
		return &fs.PathError{Op: "mkdir", Path: p, Err: err}
	}
	return nil
}

// Rename implements vfs.FS: a move to another folder, then a rename to
// another name.
func (c *conn) Rename(ctx context.Context, from, to string) error {
	pl, err := c.find(ctx, "rename", from)
	if err != nil {
		return err
	}
	if pl.store {
		return &fs.PathError{Op: "rename", Path: from, Err: vfs.ErrUnsupported}
	}
	parent, name, err := c.split(ctx, "rename", to)
	if err != nil {
		return err
	}
	if path.Dir(path.Clean(from)) != path.Dir(path.Clean(to)) {
		if err := c.s.Move(ctx, pl.obj.Handle, parent.storage, parent.handle()); err != nil {
			return &fs.PathError{Op: "rename", Path: from, Err: err}
		}
	}
	if name != pl.obj.Name {
		if err := c.s.Rename(ctx, pl.obj.Handle, name); err != nil {
			return &fs.PathError{Op: "rename", Path: from, Err: err}
		}
	}
	return nil
}

// Remove implements vfs.FS: a folder only once it is empty, as a
// delete would remove all it holds.
func (c *conn) Remove(ctx context.Context, p string) error {
	pl, err := c.find(ctx, "remove", p)
	if err != nil {
		return err
	}
	if pl.store {
		return &fs.PathError{Op: "remove", Path: p, Err: vfs.ErrUnsupported}
	}
	if pl.obj.Dir {
		held, err := c.s.List(ctx, pl.storage, pl.obj.Handle)
		if err != nil {
			return &fs.PathError{Op: "remove", Path: p, Err: err}
		}
		if len(held) > 0 {
			return &fs.PathError{Op: "remove", Path: p, Err: errors.New("directory not empty")}
		}
	}
	if err := c.s.Delete(ctx, pl.obj.Handle); err != nil {
		return &fs.PathError{Op: "remove", Path: p, Err: err}
	}
	return nil
}

// Device is a phone or a camera plugged in.
type Device struct {
	usb usb.Device
}

// Server is the device as a location's server: mtp:// and its USB
// port's sysfs name.
func (d Device) Server() vfs.Server { return vfs.Server{Scheme: vfs.MTP, Host: d.usb.Name} }

// Label is the device's name: its product, after its maker unless the
// product names it, else its USB ids.
func (d Device) Label() string {
	maker, _, _ := strings.Cut(strings.TrimSpace(d.usb.Manufacturer), " ")
	switch {
	case d.usb.Product == "":
		return fmt.Sprintf("%04x:%04x", d.usb.VendorID, d.usb.ProductID)
	case maker == "" || strings.Contains(strings.ToLower(d.usb.Product), strings.ToLower(maker)):
		return d.usb.Product
	}
	return maker + " " + d.usb.Product
}

// Camera reports a camera (PTP), not a phone or a player (MTP).
func (d Device) Camera() bool { return d.usb.Protocol == usb.PTP }

// Devices are the phones and cameras plugged in now.
func Devices(ctx context.Context) ([]Device, error) {
	found, err := mtp.Devices(ctx)
	out := make([]Device, len(found))
	for i, d := range found {
		out[i] = Device{usb: d}
	}
	return out, err
}

// Watch calls changed each time a device is plugged in or out, until
// ctx ends.
func Watch(ctx context.Context, changed func()) error {
	return mtp.Watch(ctx, func([]usb.Device, error) { changed() })
}

// Open opens a session with d, its filesystem; a locked phone is
// vfs.ErrLocked.
func Open(ctx context.Context, d Device) (vfs.Conn, error) {
	s, err := mtp.Open(ctx, d.usb)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", d.Label(), lockedError(err))
	}
	return New(ctx, s)
}
