package preview

import (
	"image"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/lib/intconv"

	"github.com/stubbedev/lys/desktop/i18n"
)

// detailSeparator joins the parts of a detail line.
const detailSeparator = " · "

// detail is the line under a loaded file's name: a folder's kind, item
// count and size, a file's kind, an archive's item count or a table's
// rows, a picture's pixel size, the file's
// size, and when it was modified. What is not known is left out: a file
// that does not stat says its kind alone, and a picture held in memory
// has no time.
func detail(name string, c Content) string {
	info := c.info
	if c.renderer == Folder {
		return joinDetail(i18n.T("preview-kind-folder"), folderItems(c.folder), folderBytes(c.folder), modified(info.Modified))
	}
	kind := kindOf(name, info)
	if info.Modified.IsZero() && info.Size == 0 {
		return kind
	}
	items := ""
	switch c.renderer {
	case Archive:
		items = folderItems(c.folder)
	case Table:
		items = tableRows(c.table)
	}
	return joinDetail(kind, items, pixelSize(c.picture.size), byteSize(info.Size), modified(info.Modified))
}

// kindOf names a file's type: its MIME type's description, else its
// extension ("PNG file"), else File.
func kindOf(name string, info Info) string {
	switch {
	case info.Dir:
		return i18n.T("preview-kind-folder")
	case info.Description != "":
		return info.Description
	}
	if ext := strings.TrimPrefix(filepath.Ext(name), "."); ext != "" {
		return i18n.T("preview-kind-typed", i18n.Str("type", strings.ToUpper(ext)))
	}
	return i18n.T("preview-kind-file")
}

func folderItems(st folderStats) string {
	shown := i18n.Shell().Integer(int64(st.items))
	if st.itemsMore {
		return i18n.T("preview-folder-items-capped", i18n.Str("shown", shown))
	}
	return i18n.T("preview-folder-items", i18n.Int("count", int64(st.items)), i18n.Str("shown", shown))
}

func tableRows(t table) string {
	shown := i18n.Shell().Integer(int64(len(t.rows)))
	if t.more {
		return i18n.T("preview-table-rows-capped", i18n.Str("shown", shown))
	}
	return i18n.T("preview-table-rows", i18n.Int("count", int64(len(t.rows))), i18n.Str("shown", shown))
}

func folderBytes(st folderStats) string {
	size := byteSize(st.bytes)
	if st.bytesMore {
		return i18n.T("preview-size-at-least", i18n.Str("size", size))
	}
	return size
}

func pixelSize(size image.Point) string {
	if size == (image.Point{}) {
		return ""
	}
	return strconv.Itoa(size.X) + " × " + strconv.Itoa(size.Y)
}

func byteSize(n int64) string { return i18n.Shell().ByteSize(intconv.Clamp[uint64](n)) }

// modified is a modification time in local time, in the desktop
// locale's numeric date and time.
func modified(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return i18n.Shell().DateTime(t.Local())
}

func joinDetail(parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString(detailSeparator)
		}
		b.WriteString(p)
	}
	return b.String()
}
