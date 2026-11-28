// Package preview is Quick Look: one view showing a file in the
// richest form lys renders, its renderer chosen by the file's MIME
// type. A picture (an SVG drawing included) shows zoomable, a video or
// a sound plays with its controls, text and code show highlighted (JSON
// formatted), Markdown shows rendered, a CSV or TSV table as a table,
// an archive lists what it holds, a font shows a specimen, a folder
// counts its items and their size, a document shows its pages where the
// host reads documents, and anything else shows its sandboxed
// thumbnail, else its type's icon. A file that cannot be shown says
// why. Every surface that previews files (the file chooser, the
// launcher, lys-viewer) shows this one view.
package preview

import (
	"slices"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/desktop/mime"
)

// Renderer is what a view shows a file as.
type Renderer uint8

// The renderers. Empty is a view showing nothing and Loading one
// finding out what to show; Document is a document's pages, which a
// view asks for (Config.Pages); Markdown is Markdown laid out, Table a
// CSV or TSV file's rows, Archive the files an archive holds and Font a
// font's specimen; Icon is the last fallback, and Failed a file that
// cannot be shown, which says why.
const (
	Empty Renderer = iota
	Loading
	Image
	Video
	Text
	Folder
	Document
	Markdown
	Table
	Archive
	Font
	Thumbnail
	Icon
	Failed
)

func (r Renderer) String() string {
	switch r {
	case Empty:
		return "empty"
	case Loading:
		return "loading"
	case Image:
		return "image"
	case Video:
		return "video"
	case Text:
		return "text"
	case Folder:
		return "folder"
	case Document:
		return "document"
	case Markdown:
		return "markdown"
	case Table:
		return "table"
	case Archive:
		return "archive"
	case Font:
		return "font"
	case Thumbnail:
		return "thumbnail"
	case Icon:
		return "icon"
	case Failed:
		return "failed"
	}
	return "renderer(" + strconv.Itoa(int(r)) + ")"
}

// pictureTypes are the image types imagedecode decodes.
var pictureTypes = imagedecode.Types()

// PictureTypes are the image types a view shows as a picture, each
// with the subtypes the MIME database classes under it.
func PictureTypes() []string { return slices.Clone(pictureTypes) }

// The types the richer renderers show, each with the subtypes the MIME
// database classes under it but for the archives, whose subtypes (an
// office document, an EPUB) are documents of their own.
const (
	svgType      = "image/svg+xml"
	markdownType = "text/markdown"
	csvType      = "text/csv"
	tsvType      = "text/tab-separated-values"
	jsonType     = "application/json"
)

// fontTypes are the font formats a specimen shows.
var fontTypes = []string{"font/ttf", "font/otf", "font/woff", "font/woff2"}

// RendererFor is the renderer a file of mimeType is shown with first:
// Image for a picture imagedecode decodes and for an SVG drawing, Video
// for a video and for an audio file that may carry one, Markdown for
// Markdown, Table for CSV and TSV, Archive for a zip, 7z or tar archive
// (gzip, bzip2, xz, LZMA or zstd compressed), Font for a font, Text for
// other text, Folder for a directory and Thumbnail for everything else.
// A video
// without a picture to show, text that does not read as text and a
// font or archive that does not read fall back to Thumbnail, a drawing
// or a table that does not read to Text, and a file without a thumbnail
// to Icon, once loading finds out.
func RendererFor(db *mime.Database, mimeType string) Renderer {
	switch {
	case mimeType == mime.Directory:
		return Folder
	case isPicture(db, mimeType), db.IsA(mimeType, svgType):
		return Image
	case strings.HasPrefix(mimeType, "video/"), strings.HasPrefix(mimeType, "audio/"):
		return Video
	case db.IsA(mimeType, markdownType):
		return Markdown
	case db.IsA(mimeType, csvType), db.IsA(mimeType, tsvType):
		return Table
	case archiveFormatOf(db, mimeType) != noArchive:
		return Archive
	case slices.ContainsFunc(fontTypes, func(t string) bool { return db.IsA(mimeType, t) }):
		return Font
	case db.IsA(mimeType, mime.PlainText):
		return Text
	}
	return Thumbnail
}

func isPicture(db *mime.Database, mimeType string) bool {
	for _, t := range pictureTypes {
		if db.IsA(mimeType, t) {
			return true
		}
	}
	return false
}
