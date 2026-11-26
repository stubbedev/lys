package launcher

import (
	"context"
	"errors"
	"log"

	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/thumbnail"
)

// Thumbnails for row icons and file previews (rofi's thumbnail://),
// rofi-thumbnails(5)'s contract made by package thumbnail:
//
//   - a row asks for one by prefixing its icon with thumbnail://;
//   - the image is the freedesktop cache's, so a thumbnail a file
//     manager made is reused and one made here shows up there;
//   - lys makes it with its own decoders, a picture, a document or a
//     video's frame; rofi's -preview-cmd, a command making it, is
//     refused, as lys runs no thumbnail command;
//   - a file nothing can thumbnail keeps its mimetype icon.
//
// Only the spec's normal size (128px) is asked for: a launcher row is
// about 40px tall.

// ThumbnailScheme is the icon prefix that asks for a thumbnail.
const ThumbnailScheme = "thumbnail://"

// Thumbnailer makes the normal thumbnails of files through files,
// finding a file's type in types.
type Thumbnailer struct {
	files *thumbnail.Thumbnailer
	types *mime.Database
}

// NewThumbnailer makes thumbnails with files, finding a file's type in
// types.
func NewThumbnailer(files *thumbnail.Thumbnailer, types *mime.Database) Thumbnailer {
	return Thumbnailer{files: files, types: types}
}

// Generate is the cache file holding file's thumbnail, made when the
// cache holds none made from the file as it is now. A file nothing can
// thumbnail, or too large to, is a normal outcome (false): the caller
// keeps the fallback; a thumbnail that could not be made is logged.
func (t Thumbnailer) Generate(ctx context.Context, file string) (string, bool) {
	made, err := t.files.File(ctx, file, t.types.TypeByName(file), thumbnail.NormalPx)
	if err != nil {
		if !errors.Is(err, thumbnail.ErrNone) && !errors.Is(err, thumbnail.ErrTooLarge) && ctx.Err() == nil {
			log.Printf("launcher: the thumbnail of %s: %v", file, err)
		}
		return "", false
	}
	return made, true
}

// MimeIcon is the icon-theme name for a file with no thumbnail: its
// mimetype's file glyph.
func MimeIcon(file string) string {
	db := mime.System()
	return db.FileIcon(db.TypeByName(file))
}
