package preview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/stubbedev/lys/lib/exif"
	"github.com/stubbedev/lys/ui/media"
	"github.com/stubbedev/memory-go"

	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/thumbnail"
)

// What a preview reads: a text file's first TextBytes, a folder's
// first folderWalkEntries entries counting down its tree, read
// folderReadBatch at a time.
const (
	TextBytes         = 64 << 10
	folderWalkEntries = 100_000
	folderReadBatch   = 256
)

// PictureBudget is the bytes of decoded pixels a host's previews hold
// by default: one 64-megapixel picture.
const PictureBudget = 256 << 20

// y4mMagic starts a YUV4MPEG2 stream.
const y4mMagic = "YUV4MPEG2 "

// Pictures holds the pictures previews decode: their pixels, a JPEG's
// off the Go heap, within one budget of bytes until the view showing
// them lets them go. A picture past the whole budget is not decoded.
type Pictures struct {
	mem    memory.Shared
	budget *thumbnail.Budget
}

// NewPictures holds pictures within total bytes.
func NewPictures(total int64) *Pictures {
	return &Pictures{budget: thumbnail.NewBudget(total)}
}

// Held is the bytes of the pictures shown or decoding.
func (p *Pictures) Held() int64 { return p.budget.Held() }

// Sources are what loading reads files through: the MIME database
// that types them, the thumbnailer that makes a document's thumbnail
// (nil makes none, so such a file shows its icon), and the pictures'
// budget.
type Sources struct {
	MIME       *mime.Database
	Thumbnails *thumbnail.Thumbnailer
	Pictures   *Pictures
}

// Request is a file to load, and the box in pixels its thumbnail is
// made to fit.
type Request struct {
	Path string
	Fit  image.Point
}

// Info is what is known of a file besides its contents: its MIME type
// and the type's description ("" when the database has none), whether
// it is a folder, its size and when it was modified.
type Info struct {
	Type        string
	Description string
	Dir         bool
	Size        int64
	Modified    time.Time
}

// Failure is why a file shows a message in place of its contents.
type Failure uint8

// The failures: a file that cannot be read, one that does not decode,
// a picture past the budget, and a video in a format or codec lys does
// not play.
const (
	Unreadable Failure = iota + 1
	Undecodable
	TooLarge
	Unsupported
)

// Content is a loaded preview of one file: its Info and what it shows
// as, by its renderer. A view frees what it holds once nothing shows it
// (discard).
type Content struct {
	info     Info
	renderer Renderer
	picture  picture
	text     string
	video    media.Source
	folder   folderStats
	failure  Failure
	err      error
}

// Renderer is what the content shows as.
func (c Content) Renderer() Renderer { return c.renderer }

// picture is a decoded image, stored in its file's orientation, and the
// source's pixel size as shown (zero for a thumbnail, whose source size
// is not known); free lets its pixels and budget go.
type picture struct {
	img    image.Image
	orient exif.Orientation
	size   image.Point
	free   func()
}

// folderStats is a folder's count of items and the bytes of the
// regular files under it; a more flag says the walk stopped short (at
// its cap, or at a subfolder it could not read), so the number is a
// floor.
type folderStats struct {
	items     int
	itemsMore bool
	bytes     int64
	bytesMore bool
}

func failed(info Info, f Failure, err error) Content {
	return Content{info: info, renderer: Failed, failure: f, err: err}
}

// Load finds out what to show of a file, off the UI loop, and gives up
// with a Failed content once ctx is done. A folder counts what it
// holds; a picture decodes whole within the budget; a video opens its
// demuxer; text reads its first TextBytes. A video without a picture,
// text that does not read as text, and every other type show their
// thumbnail fitted into the request's box, else their icon.
func Load(ctx context.Context, src Sources, req Request) Content {
	info, err := stat(src.MIME, req.Path)
	if err != nil {
		return failed(info, Unreadable, err)
	}
	switch RendererFor(src.MIME, info.Type) {
	case Folder:
		stats, err := walkFolder(ctx, req.Path, folderWalkEntries)
		if err != nil {
			return failed(info, Unreadable, err)
		}
		return Content{info: info, renderer: Folder, folder: stats}
	case Image:
		return loadPicture(ctx, src.Pictures, req.Path, info)
	case Video:
		if c, ok := loadVideo(req.Path, info); ok {
			return c
		}
	case Text:
		if c, ok := loadText(req.Path, info); ok {
			return c
		}
	}
	return loadThumbnail(ctx, src.Thumbnails, req, info)
}

// stat is what the file is: its type by name and contents, a folder's
// inode/directory.
func stat(db *mime.Database, path string) (Info, error) {
	st, err := os.Stat(path)
	if err != nil {
		return Info{Type: db.TypeByName(path)}, err
	}
	info := Info{Dir: st.IsDir(), Size: st.Size(), Modified: st.ModTime()}
	if info.Type, err = db.TypeOfFile(path); err != nil {
		return info, err
	}
	info.Description = db.Comment(info.Type)
	return info, nil
}

// loadPicture decodes the picture at path whole once its pixels' bytes
// are free in the budget, in the orientation its file records.
func loadPicture(ctx context.Context, p *Pictures, path string, info Info) Content {
	need, err := imagedecode.DecodeBytes(path)
	if err != nil {
		return failed(info, Undecodable, err)
	}
	if need > p.budget.Total() {
		return failed(info, TooLarge, fmt.Errorf("preview: %s decodes to %d bytes, past the %d the budget holds", path, need, p.budget.Total()))
	}
	release, err := p.budget.Acquire(ctx, need)
	if err != nil {
		return failed(info, Unreadable, err)
	}
	dec, err := imagedecode.OpenOffHeap(&p.mem, path)
	if err != nil {
		release()
		return failed(info, Undecodable, err)
	}
	orient := Orientation(path)
	w, h := dec.Image.Bounds().Dx(), dec.Image.Bounds().Dy()
	w, h = orient.Size(w, h)
	free := func() {
		if err := dec.Free(); err != nil {
			log.Printf("preview: freeing the picture of %s: %v", path, err)
		}
		release()
	}
	return Content{info: info, renderer: Image, picture: picture{img: dec.Image, orient: orient, size: image.Pt(w, h), free: free}}
}

// Orientation is the orientation path's EXIF block records, as a
// picture shows; a block that does not parse is logged, and the
// picture shows as stored.
func Orientation(path string) exif.Orientation {
	f, err := os.Open(path) //nolint:gosec // the file being previewed
	if err != nil {
		log.Printf("preview: reading the orientation of %s: %v", path, err)
		return exif.TopLeft
	}
	defer func() { _ = f.Close() }()
	o, err := exif.Read(f)
	if err != nil {
		log.Printf("preview: the orientation of %s: %v", path, err)
		return exif.TopLeft
	}
	return o
}

// loadVideo opens the video at path: a YUV4MPEG2 stream decoded on the
// CPU, or an MP4 or Matroska file demuxed for the GPU to decode. An
// audio file without a picture is not one (false); a video lys cannot
// open is Unsupported.
func loadVideo(path string, info Info) (Content, bool) {
	src, err := openVideo(path)
	if err == nil {
		return Content{info: info, renderer: Video, video: src}, true
	}
	if !strings.HasPrefix(info.Type, "video/") {
		return Content{}, false
	}
	return failed(info, Unsupported, err), true
}

func openVideo(path string) (media.Source, error) {
	f, err := os.Open(path) //nolint:gosec // the file being previewed
	if err != nil {
		return nil, err
	}
	head := make([]byte, len(y4mMagic))
	if _, err := io.ReadFull(f, head); err == nil && string(head) == y4mMagic {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, errors.Join(fmt.Errorf("preview: rewinding %s: %w", path, err), f.Close())
		}
		src, err := media.NewY4M(f)
		if err != nil {
			return nil, errors.Join(err, f.Close())
		}
		return src, nil
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("preview: closing %s: %w", path, err)
	}
	return media.OpenFile(path)
}

// loadText reads the first TextBytes of path as text, false when they
// do not read as text.
func loadText(path string, info Info) (Content, bool) {
	head, err := readHead(path, TextBytes)
	if err != nil {
		return failed(info, Unreadable, err), true
	}
	text, ok := textOf(head, int64(len(head)) < info.Size)
	if !ok {
		return Content{}, false
	}
	return Content{info: info, renderer: Text, text: text}, true
}

// readHead reads the first n bytes of path, and no more.
func readHead(path string, n int) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // the file being previewed
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, n)
	read, err := io.ReadFull(f, buf)
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		err = nil
	}
	if err != nil {
		return nil, fmt.Errorf("preview: reading %s: %w", path, err)
	}
	return buf[:read], nil
}

// textOf is data as text when it reads as text: something, no NUL, and
// valid UTF-8 but for a character cut at the end of what was read.
func textOf(data []byte, cut bool) (string, bool) {
	if len(data) == 0 || bytes.IndexByte(data, 0) >= 0 {
		return "", false
	}
	if cut {
		for range utf8.UTFMax {
			if utf8.Valid(data) {
				break
			}
			data = data[:len(data)-1]
		}
	}
	if !utf8.Valid(data) {
		return "", false
	}
	return string(data), true
}

// loadThumbnail is the file's thumbnail fitted into the request's box,
// else its icon: there is no thumbnailer, nothing makes one, or the
// source is past what is thumbnailed.
func loadThumbnail(ctx context.Context, th *thumbnail.Thumbnailer, req Request, info Info) Content {
	if th == nil {
		return Content{info: info, renderer: Icon}
	}
	raster, _, err := th.Fit(ctx, req.Path, info.Type, req.Fit.X, req.Fit.Y)
	switch {
	case errors.Is(err, thumbnail.ErrNone), errors.Is(err, thumbnail.ErrTooLarge):
		return Content{info: info, renderer: Icon}
	case err != nil:
		return failed(info, Undecodable, err)
	}
	free := func() {
		if err := th.Free(raster); err != nil {
			log.Printf("preview: freeing the thumbnail of %s: %v", req.Path, err)
		}
	}
	return Content{info: info, renderer: Thumbnail, picture: picture{img: raster.Image, orient: exif.TopLeft, free: free}}
}

// walkFolder counts dir's items and the bytes of the regular files
// under it, breadth first, so the items are counted before anything
// below them, stopping at limit entries and once ctx is done. Links are
// not followed. A subfolder that cannot be read makes the bytes a
// floor; dir itself not reading is the error.
func walkFolder(ctx context.Context, dir string, limit int) (folderStats, error) {
	w := folderWalk{ctx: ctx, limit: limit, queue: []string{dir}}
	for top := true; len(w.queue) > 0 && !w.stopped; top = false {
		d := w.queue[0]
		w.queue = w.queue[1:]
		err := w.read(d, top)
		if err == nil {
			continue
		}
		if top || ctx.Err() != nil {
			return w.stats, err
		}
		w.stats.bytesMore = true
	}
	return w.stats, nil
}

// folderWalk is a walk under way: what it found, the folders it has
// still to read, and the entries it has seen against its limit.
type folderWalk struct {
	ctx     context.Context
	limit   int
	seen    int
	stopped bool
	queue   []string
	stats   folderStats
}

// read reads the folder d, top when it is the folder walked, queuing
// its subfolders.
func (w *folderWalk) read(d string, top bool) error {
	f, err := os.Open(d) //nolint:gosec // a folder being previewed
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	for {
		entries, err := f.ReadDir(folderReadBatch)
		for _, e := range entries {
			if err := w.ctx.Err(); err != nil {
				return err
			}
			if w.seen == w.limit {
				w.stats.itemsMore = w.stats.itemsMore || top
				w.stats.bytesMore, w.stopped = true, true
				return nil
			}
			w.seen++
			if top {
				w.stats.items++
			}
			w.count(d, e)
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("preview: reading %s: %w", d, err)
		}
	}
}

// count adds the entry e of the folder d: a subfolder to read, a
// regular file's bytes.
func (w *folderWalk) count(d string, e fs.DirEntry) {
	switch {
	case e.IsDir():
		w.queue = append(w.queue, filepath.Join(d, e.Name()))
	case e.Type().IsRegular():
		fi, err := e.Info()
		if err != nil {
			w.stats.bytesMore = true
			return
		}
		w.stats.bytes += fi.Size()
	}
}
