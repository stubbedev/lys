// Package preview is Quick Look: one view showing a file in the
// richest form lys renders, its renderer chosen by the file's MIME
// type. A picture shows zoomable, a video plays with its controls,
// text and code show highlighted, a folder counts its items and their
// size, and anything else shows its sandboxed thumbnail, else its type's
// icon. A file that cannot be shown says why. Every surface that
// previews files (the file chooser, the launcher, lys-viewer) shows
// this one view.
package preview

import (
	"slices"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/mime"
)

// Renderer is what a view shows a file as.
type Renderer uint8

// The renderers. Empty is a view showing nothing and Loading one
// finding out what to show; Icon is the last fallback, and Failed a
// file that cannot be shown, which says why.
const (
	Empty Renderer = iota
	Loading
	Image
	Video
	Text
	Folder
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
	case Thumbnail:
		return "thumbnail"
	case Icon:
		return "icon"
	case Failed:
		return "failed"
	}
	return "renderer(" + strconv.Itoa(int(r)) + ")"
}

// pictureTypes are the image types imagedecode decodes, with their
// subtypes.
var pictureTypes = []string{
	"image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp", "image/tiff",
	"image/x-tga", "image/x-portable-anymap", "image/x-farbfeld",
}

// PictureTypes are the image types a view shows as a picture, each
// with the subtypes the MIME database classes under it.
func PictureTypes() []string { return slices.Clone(pictureTypes) }

// RendererFor is the renderer a file of mimeType is shown with first:
// Image for a picture imagedecode decodes, Video for a video and for an
// audio file that may carry one, Text for text, Folder for a directory
// and Thumbnail for everything else. A video without a picture to show
// and text that does not read as text fall back to Thumbnail, and a
// file without a thumbnail to Icon, once loading finds out.
func RendererFor(db *mime.Database, mimeType string) Renderer {
	switch {
	case mimeType == mime.Directory:
		return Folder
	case isPicture(db, mimeType):
		return Image
	case strings.HasPrefix(mimeType, "video/"), strings.HasPrefix(mimeType, "audio/"):
		return Video
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
