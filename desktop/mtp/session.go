// Package mtp browses phones and cameras plugged in over USB, speaking
// MTP and PTP itself through usbfs: Devices and Watch find them, Open
// starts a Session that lists, reads, writes and deletes their files.
package mtp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"

	"github.com/stubbedev/lys/lib/intconv"

	"github.com/stubbedev/lys/desktop/mtp/ptp"
	"github.com/stubbedev/lys/desktop/mtp/usb"
)

const (
	transferTimeout = 30 * time.Second
	closeTimeout    = 5 * time.Second
	maxPartial      = 1 << 20
)

// ErrLocked is a phone that keeps its files from the host until its
// screen is unlocked and access to its data allowed.
var ErrLocked = errors.New("mtp: the device is locked or has not allowed access to its files")

// Session is one open session with a device. Its calls run one at a
// time; Event waits beside them.
type Session struct {
	conn *ptp.Conn
}

// Open claims d's interface and opens a session with it. Dates the
// device gives without a zone are read as local time, the zone a phone
// keeps its clock in.
func Open(ctx context.Context, d usb.Device) (*Session, error) {
	t, err := usb.Open(d)
	if err != nil {
		return nil, err
	}
	return NewSession(ctx, t, time.Local)
}

// NewSession opens a session over t, which the Session then owns, with
// dates without a zone read as wall clock in loc.
func NewSession(ctx context.Context, t ptp.Transport, loc *time.Location) (*Session, error) {
	c, err := ptp.Open(ctx, t, loc, transferTimeout)
	if err != nil {
		return nil, locked(err)
	}
	return &Session{conn: c}, nil
}

func locked(err error) error {
	var re *ptp.ResponseError
	if errors.As(err, &re) && (re.Code == ptp.RespDeviceBusy || re.Code == ptp.RespAccessDenied) {
		return fmt.Errorf("%w: %w", ErrLocked, err)
	}
	return err
}

// Info is the DeviceInfo the device gave when the session opened.
func (s *Session) Info() ptp.DeviceInfo { return s.conn.Info() }

// Close ends the session and releases the device.
func (s *Session) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
	defer cancel()
	return s.conn.Close(ctx)
}

func (s *Session) mtp() bool {
	info := s.conn.Info()
	return info.VendorExtensionID == 6 || strings.Contains(info.VendorExtensionDesc, "microsoft.com")
}

// Storage is one store on the device: internal memory, a card.
type Storage struct {
	ID          ptp.StorageID
	Description string
	Label       string
	Capacity    uint64
	Free        uint64
	ReadOnly    bool
}

// Storages are the device's stores. An MTP device offering none is
// locked: Android offers its stores only once unlocked with access
// allowed, and sends ptp.EventStoreAdded then. A camera offering none
// has no card.
func (s *Session) Storages(ctx context.Context) ([]Storage, error) {
	ids, err := s.conn.StorageIDs(ctx)
	if err != nil {
		return nil, locked(err)
	}
	if len(ids) == 0 && s.mtp() {
		return nil, fmt.Errorf("%w: it offers no storage", ErrLocked)
	}
	out := make([]Storage, 0, len(ids))
	for _, id := range ids {
		info, err := s.conn.StorageInfo(ctx, id)
		if err != nil {
			return nil, locked(err)
		}
		out = append(out, Storage{
			ID:          id,
			Description: info.Description,
			Label:       info.VolumeLabel,
			Capacity:    info.MaxCapacity,
			Free:        info.FreeSpace,
			ReadOnly:    info.Access != ptp.ReadWrite,
		})
	}
	return out, nil
}

// Object is a file or folder on the device.
type Object struct {
	Handle  ptp.ObjectHandle
	Storage ptp.StorageID
	// Parent is ptp.Root for an object at the top of its store.
	Parent ptp.ObjectHandle
	Name   string
	Size   int64
	// ModTime is the modification date, or the capture date of a
	// device that gives only that; zero when it gives neither.
	ModTime time.Time
	Dir     bool
}

// List is the objects in parent on storage, ptp.Root for the top of the
// store, in the device's order. One deleted while the list is read is
// left out.
func (s *Session) List(ctx context.Context, storage ptp.StorageID, parent ptp.ObjectHandle) ([]Object, error) {
	handles, err := s.conn.ObjectHandles(ctx, storage, parent)
	if err != nil {
		return nil, err
	}
	out := make([]Object, 0, len(handles))
	for _, h := range handles {
		o, err := s.Stat(ctx, h)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

// Stat describes the object h.
func (s *Session) Stat(ctx context.Context, h ptp.ObjectHandle) (Object, error) {
	info, err := s.conn.ObjectInfo(ctx, h)
	if err != nil {
		return Object{}, err
	}
	o := Object{
		Handle:  h,
		Storage: info.StorageID,
		Parent:  info.Parent,
		Name:    info.Filename,
		Size:    int64(info.CompressedSize),
		ModTime: info.ModificationDate,
		Dir:     info.Format == ptp.FormatAssociation,
	}
	if o.ModTime.IsZero() {
		o.ModTime = info.CaptureDate
	}
	if o.Dir || info.CompressedSize != ptp.SizeUnknown {
		return o, nil
	}
	if !s.conn.Info().Supports(ptp.OpGetObjectPropValue) {
		return Object{}, fmt.Errorf("mtp: %q is 4 GiB or more and the device gives no exact size: %w", o.Name, errors.ErrUnsupported)
	}
	b, err := s.conn.ObjectPropValue(ctx, h, ptp.PropObjectSize)
	if err != nil {
		return Object{}, err
	}
	size, err := ptp.ParseUint64(b)
	if err != nil {
		return Object{}, err
	}
	if o.Size, err = intconv.Exact[int64](size); err != nil {
		return Object{}, fmt.Errorf("mtp: size of %q: %w", o.Name, err)
	}
	return o, nil
}

// Download streams the content of the object h into w.
func (s *Session) Download(ctx context.Context, h ptp.ObjectHandle, w io.Writer) error {
	return s.conn.Object(ctx, h, w)
}

// Reader reads an object at any offset, each read its own transaction,
// so it holds the session only while one runs.
type Reader struct {
	s    *Session
	ctx  context.Context
	h    ptp.ObjectHandle
	size int64
	off  int64
}

// OpenReader is a Reader over the object h whose reads run under ctx.
// It fails with errors.ErrUnsupported on a device that cannot read part
// of an object; Download reads those.
func (s *Session) OpenReader(ctx context.Context, h ptp.ObjectHandle) (*Reader, error) {
	if !s.conn.Partial() {
		return nil, fmt.Errorf("mtp: read object %d at an offset: %w", h, errors.ErrUnsupported)
	}
	o, err := s.Stat(ctx, h)
	if err != nil {
		return nil, err
	}
	if o.Dir {
		return nil, fmt.Errorf("mtp: read %q: it is a folder", o.Name)
	}
	return &Reader{s: s, ctx: ctx, h: h, size: o.Size}, nil
}

// Size is the object's size when the Reader opened.
func (r *Reader) Size() int64 { return r.size }

// ReadAt reads len(p) bytes at off, fewer only at the end with io.EOF.
func (r *Reader) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("mtp: read object %d at negative offset %d", r.h, off)
	}
	n := 0
	for n < len(p) && off+int64(n) < r.size {
		want := int(min(int64(len(p)-n), r.size-off-int64(n), maxPartial))
		m, err := r.s.conn.PartialObject(r.ctx, r.h, off+int64(n), p[n:n+want])
		n += m
		if err != nil {
			return n, err
		}
		if m < want {
			return n, fmt.Errorf("mtp: object %d ended at %d, before its size %d: %w", r.h, off+int64(n), r.size, io.ErrUnexpectedEOF)
		}
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// Read reads from the current offset.
func (r *Reader) Read(p []byte) (int, error) {
	n, err := r.ReadAt(p, r.off)
	r.off += int64(n)
	if errors.Is(err, io.EOF) && n > 0 {
		return n, nil
	}
	return n, err
}

// Seek sets the offset of the next Read.
func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += r.off
	case io.SeekEnd:
		offset += r.size
	default:
		return 0, fmt.Errorf("mtp: seek with whence %d", whence)
	}
	if offset < 0 {
		return 0, fmt.Errorf("mtp: seek to negative offset %d", offset)
	}
	r.off = offset
	return offset, nil
}

// Upload creates a file named name in parent on storage, ptp.Root for
// the top of the store, from exactly size bytes of content. A content
// shorter than size cancels the transfer and leaves no file behind.
func (s *Session) Upload(ctx context.Context, storage ptp.StorageID, parent ptp.ObjectHandle, name string, size int64, modTime time.Time, content io.Reader) (Object, error) {
	if content == nil {
		return Object{}, fmt.Errorf("mtp: upload %q without content", name)
	}
	info := ptp.ObjectInfo{Format: ptp.FormatUndefined, Filename: name, ModificationDate: modTime}
	h, err := s.conn.SendObject(ctx, storage, parent, info, size, content)
	if err != nil {
		return Object{}, fmt.Errorf("mtp: upload %q: %w", name, err)
	}
	return s.Stat(ctx, h)
}

// Mkdir creates a folder named name in parent on storage, ptp.Root for
// the top of the store.
func (s *Session) Mkdir(ctx context.Context, storage ptp.StorageID, parent ptp.ObjectHandle, name string) (Object, error) {
	info := ptp.ObjectInfo{Format: ptp.FormatAssociation, AssociationType: ptp.AssociationGenericFolder, Filename: name}
	h, err := s.conn.SendObject(ctx, storage, parent, info, 0, nil)
	if err != nil {
		return Object{}, fmt.Errorf("mtp: create folder %q: %w", name, err)
	}
	return s.Stat(ctx, h)
}

// Delete deletes the object h, and everything in it if it is a folder.
func (s *Session) Delete(ctx context.Context, h ptp.ObjectHandle) error {
	return s.conn.DeleteObject(ctx, h)
}

// Rename renames the object h to name. It fails with
// errors.ErrUnsupported on a device without MTP's object properties.
func (s *Session) Rename(ctx context.Context, h ptp.ObjectHandle, name string) error {
	if !s.conn.Info().Supports(ptp.OpSetObjectPropValue) {
		return fmt.Errorf("mtp: rename object %d: %w", h, errors.ErrUnsupported)
	}
	value, err := ptp.AppendString(nil, name)
	if err != nil {
		return err
	}
	return s.conn.SetObjectPropValue(ctx, h, ptp.PropObjectFileName, value)
}

// Move moves the object h into parent on storage, ptp.Root for the top
// of the store.
func (s *Session) Move(ctx context.Context, h ptp.ObjectHandle, storage ptp.StorageID, parent ptp.ObjectHandle) error {
	return s.conn.MoveObject(ctx, h, storage, parent)
}

// Event waits for the next event the device sends: objects and stores
// added and removed.
func (s *Session) Event(ctx context.Context) (ptp.Event, error) {
	return s.conn.Event(ctx)
}
