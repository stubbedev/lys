package launcher

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/shlex"
	"github.com/stubbedev/lys/desktop/thumbnail"
)

// Thumbnails for row icons and file previews (rofi -preview-cmd and
// thumbnail://), rofi-thumbnails(5)'s contract made by package
// thumbnail:
//
//   - a row asks for one by prefixing its icon with thumbnail://;
//   - the image is the freedesktop cache's, so a thumbnail a file
//     manager made is reused and one made here shows up there;
//   - without -preview-cmd an image is decoded and another file is
//     thumbnailed by the XDG .thumbnailer for its mimetype;
//   - with -preview-cmd that command makes it ({input} {output} {size});
//   - every command runs in the thumbnailers' sandbox, which reads the
//     input and writes the output, both under paths of its own;
//   - a file nothing can thumbnail keeps its mimetype icon.
//
// Only the spec's normal size (128px) is asked for: a launcher row is
// about 40px tall.

// ThumbnailScheme is the icon prefix that asks for a thumbnail.
const ThumbnailScheme = "thumbnail://"

// previewCmdName is -preview-cmd in errors and under the cache's fail/.
const previewCmdName = "preview-cmd"

// Thumbnailer makes the normal thumbnails of files through files, with
// -preview-cmd when one is set, finding a file's type in types; a nil
// files is a -preview-cmd that does not parse, which makes none.
type Thumbnailer struct {
	files *thumbnail.Thumbnailer
	types *mime.Database
}

// NewThumbnailer makes thumbnails with files, or with previewCmd in its
// sandbox, workers and budget when it is set; a blank one is no
// command, so it never shadows the system thumbnailers, and one that
// does not parse is logged and makes nothing. types finds the type
// whose installed thumbnailer a file runs.
func NewThumbnailer(files *thumbnail.Thumbnailer, types *mime.Database, previewCmd string) Thumbnailer {
	if strings.TrimSpace(previewCmd) == "" {
		return Thumbnailer{files: files, types: types}
	}
	words, ok := shlex.Split(previewCmd)
	if !ok || len(words) == 0 {
		log.Printf("launcher: -preview-cmd %q does not parse as a command", previewCmd)
		return Thumbnailer{}
	}
	return Thumbnailer{types: types, files: files.WithCommand(previewCmdName, func(input, output string, size int) []string {
		return previewArgv(words, input, output, size)
	})}
}

// previewArgv is -preview-cmd's words with its placeholders filled,
// each word one argument: a file name is attacker-controlled in every
// mode that lists a directory, so it is never given to a shell.
func previewArgv(words []string, input, output string, size int) []string {
	vars := Values(previewVars(input, output, size))
	argv := make([]string, len(words))
	for i, w := range words {
		argv[i] = Render(w, vars)
	}
	return argv
}

func previewVars(input, output string, size int) map[string]string {
	return map[string]string{"input": input, "output": output, "size": strconv.Itoa(size)}
}

// Generate is the cache file holding file's thumbnail, made when the
// cache holds none made from the file as it is now. A file nothing can
// thumbnail, or too large to, is a normal outcome (false): the caller
// keeps the fallback; a thumbnail that could not be made is logged.
func (t Thumbnailer) Generate(ctx context.Context, file string) (string, bool) {
	if t.files == nil {
		return "", false
	}
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
