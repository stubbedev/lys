package viewer

import (
	"slices"

	"github.com/stubbedev/lys/desktop/dochelper"
	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/lib/media/demux"
)

var pictureSubtypes = map[string][]string{
	"image/x-portable-anymap": {"image/x-portable-bitmap", "image/x-portable-graymap", "image/x-portable-pixmap"},
}

var containerTypes = map[demux.Format][]string{
	demux.MP4:        {"video/mp4"},
	demux.Matroska:   {"video/matroska", "video/x-matroska"},
	demux.WebM:       {"video/webm"},
	demux.Ogg:        nil,
	demux.NativeFLAC: nil,
	demux.MPEGAudio:  nil,
}

func MimeTypes() []string {
	var types []string
	for _, t := range preview.PictureTypes() {
		types = append(types, t)
		types = append(types, pictureSubtypes[t]...)
	}
	for _, claimed := range containerTypes {
		types = append(types, claimed...)
	}
	types = append(types, dochelper.Types()...)
	slices.Sort(types)
	return slices.Compact(types)
}

type opens struct {
	db    *mime.Database
	types []string
}

func newOpens(db *mime.Database) opens { return opens{db: db, types: MimeTypes()} }

func (o opens) name(path string) bool { return o.isA(path, o.types) }

func (o opens) document(path string) bool { return o.isA(path, dochelper.Types()) }

func (o opens) isA(path string, types []string) bool {
	t := o.db.TypeByName(path)
	return slices.ContainsFunc(types, func(claimed string) bool { return o.db.IsA(t, claimed) })
}
