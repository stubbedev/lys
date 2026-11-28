package preview

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/lib/exif"
	"github.com/stubbedev/lys/lib/icons"
	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/desktop/mime"
)

// What the richer renderers read: an SVG drawing of at most
// icons.FileLimit bytes, a font of at most FontBytes, a table's first
// tableColumns columns, and an archive's first ArchiveEntries entries
// within its first ArchiveBytes bytes, which a compressed tar is read
// through to list.
const (
	FontBytes      = 32 << 20
	ArchiveEntries = 10_000
	ArchiveBytes   = 1 << 30
	tableColumns   = 64
)

// table is rows of cells under a header, a CSV file's or an archive's
// listing, and whether more rows were left out (more).
type table struct {
	header []string
	rows   [][]string
	more   bool
}

// archiveFormat is how an archive is stored.
type archiveFormat uint8

const (
	noArchive archiveFormat = iota
	zipArchive
	tarArchive
	gzipTar
	bzip2Tar
)

// archiveTypes are the archive types listed, by format: these types
// exactly, as the types classed under a zip (an office document, an
// EPUB) are documents of their own.
var archiveTypes = map[string]archiveFormat{
	"application/zip":                    zipArchive,
	"application/java-archive":           zipArchive,
	"application/x-tar":                  tarArchive,
	"application/x-compressed-tar":       gzipTar,
	"application/x-bzip2-compressed-tar": bzip2Tar,
}

// archiveFormatOf is the format of an archive of mimeType, its alias
// resolved, noArchive for another type.
func archiveFormatOf(db *mime.Database, mimeType string) archiveFormat {
	return archiveTypes[db.Lineage(mimeType)[0]]
}

// loadSVG draws the SVG drawing at path at its aspect ratio to fill the
// request's box, however small the drawing says it is, within the
// pictures' budget; false when it is not a drawing lys draws, for the
// caller to show its source.
func loadSVG(ctx context.Context, p *Pictures, req Request, info Info) (Content, bool) {
	if info.Size > icons.FileLimit {
		return Content{}, false
	}
	data, err := os.ReadFile(req.Path)
	if err != nil {
		return failed(info, Unreadable, fmt.Errorf("preview: reading %s: %w", req.Path, err)), true
	}
	w, h, err := svgSize(data)
	if err != nil {
		return Content{}, false
	}
	scale := min(float64(req.Fit.X)/float64(w), float64(req.Fit.Y)/float64(h))
	box := image.Pt(max(1, int(math.Round(float64(w)*scale))), max(1, int(math.Round(float64(h)*scale))))
	c := p.hold(ctx, info, req.Path, int64(box.X)*int64(box.Y)*4, func() exif.Orientation { return exif.TopLeft }, func() (imagedecode.Decoded, error) {
		drawn, err := icons.DecodeImage(data, req.Path, box.X, box.Y)
		if err != nil {
			return imagedecode.Decoded{}, err
		}
		return imagedecode.Decoded{Image: drawn.Image()}, nil
	})
	if c.renderer == Failed && c.failure == Undecodable {
		return Content{}, false
	}
	c.picture.size = image.Pt(w, h)
	return c, true
}

// svgSize is the size an SVG drawing says it is, in its own units
// rounded up: its viewBox's, else its width and height in pixels.
func svgSize(data []byte) (w, h int, err error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	for {
		tok, err := d.Token()
		if err != nil {
			return 0, 0, fmt.Errorf("preview: an SVG drawing without its svg element: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "svg" {
			return 0, 0, fmt.Errorf("preview: an SVG drawing starting with %q", start.Name.Local)
		}
		var vw, vh float64
		for _, a := range start.Attr {
			switch a.Name.Local {
			case "viewBox":
				if f := strings.FieldsFunc(a.Value, func(r rune) bool { return r == ' ' || r == ',' }); len(f) == 4 {
					vw, _ = strconv.ParseFloat(f[2], 64)
					vh, _ = strconv.ParseFloat(f[3], 64)
				}
			case "width":
				if vw == 0 {
					vw, _ = strconv.ParseFloat(strings.TrimSuffix(a.Value, "px"), 64)
				}
			case "height":
				if vh == 0 {
					vh, _ = strconv.ParseFloat(strings.TrimSuffix(a.Value, "px"), 64)
				}
			}
		}
		if vw <= 0 || vh <= 0 || vw > math.MaxInt32 || vh > math.MaxInt32 {
			return 0, 0, fmt.Errorf("preview: an SVG drawing of %vx%v", vw, vh)
		}
		return int(math.Ceil(vw)), int(math.Ceil(vh)), nil
	}
}

// readText reads the first TextBytes of path as text: false when they
// do not read as text, and whether the file goes on past them.
func readText(path string, info Info) (text string, cut, ok bool, err error) {
	head, err := readHead(path, TextBytes)
	if err != nil {
		return "", false, false, err
	}
	cut = int64(len(head)) < info.Size
	text, ok = textOf(head, cut)
	return text, cut, ok, nil
}

// loadMarkdown lays out the Markdown at path, false when it does not
// read as text.
func loadMarkdown(path string, info Info) (Content, bool) {
	text, _, ok, err := readText(path, info)
	switch {
	case err != nil:
		return failed(info, Unreadable, err), true
	case !ok:
		return Content{}, false
	}
	return Content{info: info, renderer: Markdown, markdown: markdownBlocks(text)}, true
}

// loadTable reads the CSV or TSV table at path, its first row the
// header, as far as its first TextBytes, a row they cut short left out;
// false when they do not read as a table (a stray quote, say), for the
// caller to show them as text.
func loadTable(db *mime.Database, path string, info Info) (Content, bool) {
	text, cut, ok, err := readText(path, info)
	switch {
	case err != nil:
		return failed(info, Unreadable, err), true
	case !ok:
		return Content{}, false
	}
	r := csv.NewReader(strings.NewReader(text))
	r.FieldsPerRecord = -1
	if db.IsA(info.Type, tsvType) {
		r.Comma = '\t'
	}
	var t table
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if !cut || len(t.rows) == 0 {
				return Content{}, false
			}
			t.more = true
			break
		}
		rec = rec[:min(len(rec), tableColumns)]
		if t.header == nil {
			t.header = rec
			continue
		}
		t.rows = append(t.rows, rec)
	}
	if t.header == nil {
		return Content{}, false
	}
	if cut && !t.more {
		t.more = true
		if len(t.rows) > 0 && !strings.HasSuffix(text, "\n") {
			t.rows = t.rows[:len(t.rows)-1]
		}
	}
	return Content{info: info, renderer: Table, table: t}, true
}

// formatJSON is text indented as JSON, when it is a whole JSON value.
func formatJSON(text string) (string, bool) {
	var b bytes.Buffer
	if err := json.Indent(&b, []byte(text), "", "  "); err != nil {
		return "", false
	}
	return b.String(), true
}

// loadFont reads the font at path for its specimen; false when it is
// past FontBytes or does not parse.
func loadFont(path string, info Info) (Content, bool) {
	if info.Size > FontBytes {
		return Content{}, false
	}
	data, err := os.ReadFile(path) //nolint:gosec // the font being previewed
	if err != nil {
		return failed(info, Unreadable, fmt.Errorf("preview: reading %s: %w", path, err)), true
	}
	face, err := render.LoadFont(data)
	if err != nil {
		return Content{}, false
	}
	return Content{info: info, renderer: Font, font: face}, true
}

// loadArchive lists the archive at path: its first ArchiveEntries
// entries, each its name, size and modification time, within its first
// ArchiveBytes bytes, giving up once ctx is done. An archive that does
// not read is Undecodable.
func loadArchive(ctx context.Context, path string, f archiveFormat, info Info) Content {
	var entries []archiveEntry
	var more bool
	var err error
	if f == zipArchive {
		entries, more, err = zipEntries(ctx, path)
	} else {
		entries, more, err = tarEntries(ctx, path, f)
	}
	if err != nil {
		if ctx.Err() != nil {
			return failed(info, Unreadable, ctx.Err())
		}
		return failed(info, Undecodable, fmt.Errorf("preview: listing %s: %w", path, err))
	}
	t := table{header: []string{i18n.T("preview-archive-name"), i18n.T("preview-archive-size"), i18n.T("preview-archive-modified")}, more: more}
	st := folderStats{items: len(entries), itemsMore: more, bytesMore: more}
	t.rows = make([][]string, len(entries))
	for i, e := range entries {
		size := ""
		if !e.dir {
			size = byteSize(e.size)
			st.bytes += e.size
		}
		t.rows[i] = []string{e.name, size, modified(e.modified)}
	}
	return Content{info: info, renderer: Archive, table: t, folder: st}
}

// archiveEntry is one entry an archive lists.
type archiveEntry struct {
	name     string
	dir      bool
	size     int64
	modified time.Time
}

func zipEntries(ctx context.Context, path string) ([]archiveEntry, bool, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = r.Close() }()
	entries := make([]archiveEntry, 0, min(len(r.File), ArchiveEntries))
	for _, f := range r.File[:min(len(r.File), ArchiveEntries)] {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		entries = append(entries, archiveEntry{name: f.Name, dir: f.FileInfo().IsDir(), size: int64(min(f.UncompressedSize64, math.MaxInt64)), modified: f.Modified})
	}
	return entries, len(r.File) > ArchiveEntries, nil
}

func tarEntries(ctx context.Context, path string, f archiveFormat) ([]archiveEntry, bool, error) {
	file, err := os.Open(path) //nolint:gosec // the archive being previewed
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = file.Close() }()
	limited := &io.LimitedReader{R: file, N: ArchiveBytes}
	var stream io.Reader = bufio.NewReader(limited)
	switch f {
	case gzipTar:
		gz, err := gzip.NewReader(stream)
		if err != nil {
			return nil, false, err
		}
		defer func() { _ = gz.Close() }()
		stream = gz
	case bzip2Tar:
		stream = bzip2.NewReader(stream)
	}
	tr := tar.NewReader(stream)
	var entries []archiveEntry
	for len(entries) < ArchiveEntries {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		h, err := tr.Next()
		switch {
		case errors.Is(err, io.EOF):
			return entries, false, nil
		case err != nil && limited.N == 0 && len(entries) > 0:
			return entries, true, nil
		case err != nil:
			return nil, false, err
		}
		entries = append(entries, archiveEntry{name: h.Name, dir: h.Typeflag == tar.TypeDir, size: h.Size, modified: h.ModTime})
	}
	_, err = tr.Next()
	return entries, !errors.Is(err, io.EOF), nil
}
