package filechooser

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/glob"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/shell/filebrowser"
)

// Mode is what the chooser is doing.
type Mode uint8

// The chooser's modes.
const (
	ModeOpenFile Mode = iota
	ModeOpenMultiple
	ModeFolder
	ModeFolders
	ModeSave
)

// openMode maps an open request to its mode.
func openMode(r OpenRequest) Mode {
	switch {
	case r.Directory && r.Multiple:
		return ModeFolders
	case r.Directory:
		return ModeFolder
	case r.Multiple:
		return ModeOpenMultiple
	}
	return ModeOpenFile
}

// folders reports a folder pick.
func (m Mode) folders() bool { return m == ModeFolder || m == ModeFolders }

// multiple reports a pick of several.
func (m Mode) multiple() bool { return m == ModeOpenMultiple || m == ModeFolders }

// opening reports a pick of files to open: Recent and Trash list files
// to open, never a folder to save in.
func (m Mode) opening() bool { return m == ModeOpenFile || m == ModeOpenMultiple }

// kinds is what a pick in m lists.
func (m Mode) kinds() filebrowser.Kinds {
	if m.folders() {
		return filebrowser.FoldersOnly
	}
	return filebrowser.FilesAndFolders
}

// selection is how many rows a pick in m selects.
func (m Mode) selection() widget.SelectionMode {
	if m.multiple() {
		return widget.SelectionMultiple
	}
	return widget.SelectionSingle
}

// confirmLabel is the confirm button's text.
func (m Mode) confirmLabel() string {
	switch {
	case m == ModeSave:
		return i18n.T("file-chooser-button-save")
	case m.folders():
		return i18n.T("file-chooser-button-select")
	}
	return i18n.T("file-chooser-button-open")
}

// FilterLabel is a filter's name. Apps that want the patterns seen put
// them in the name; GTK's case-insensitive suffix globs
// (*.[pP][nN][gG]) would read as noise. Only a nameless filter shows
// its patterns, MIME types as their *.ext.
func FilterLabel(f Filter) string {
	if strings.TrimSpace(f.Name) != "" {
		return f.Name
	}
	var tokens []string
	for _, r := range f.Rules {
		token := r.Value
		if r.Kind == RuleMIME {
			if ext := mimeExt(r.Value); ext != "" {
				token = "*." + ext
			}
		}
		if !slices.Contains(tokens, token) {
			tokens = append(tokens, token)
		}
	}
	return strings.Join(tokens, ", ")
}

// mimeExts are the canonical extensions of the types apps put in
// file chooser filters; shared-mime-info lists extensions in no useful
// order.
var mimeExts = map[string]string{
	"application/epub+zip":                            "epub",
	"application/gzip":                                "gz",
	"application/javascript":                          "js",
	"application/json":                                "json",
	"application/msword":                              "doc",
	"application/ogg":                                 "ogx",
	"application/pdf":                                 "pdf",
	"application/rtf":                                 "rtf",
	"application/toml":                                "toml",
	"application/vnd.ms-excel":                        "xls",
	"application/vnd.ms-powerpoint":                   "ppt",
	"application/vnd.oasis.opendocument.presentation": "odp",
	"application/vnd.oasis.opendocument.spreadsheet":  "ods",
	"application/vnd.oasis.opendocument.text":         "odt",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": "pptx",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         "xlsx",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   "docx",
	"application/x-7z-compressed":                                               "7z",
	"application/x-bzip2":                                                       "bz2",
	"application/x-tar":                                                         "tar",
	"application/x-xz":                                                          "xz",
	"application/xml":                                                           "xml",
	"application/yaml":                                                          "yaml",
	"application/zip":                                                           "zip",
	"application/zstd":                                                          "zst",
	"audio/aac":                                                                 "aac",
	"audio/flac":                                                                "flac",
	"audio/mp3":                                                                 "mp3",
	"audio/mp4":                                                                 "m4a",
	"audio/mpeg":                                                                "mp3",
	"audio/ogg":                                                                 "oga",
	"audio/opus":                                                                "opus",
	"audio/wav":                                                                 "wav",
	"audio/webm":                                                                "weba",
	"audio/x-flac":                                                              "flac",
	"audio/x-wav":                                                               "wav",
	"font/otf":                                                                  "otf",
	"font/ttf":                                                                  "ttf",
	"font/woff":                                                                 "woff",
	"font/woff2":                                                                "woff2",
	"image/avif":                                                                "avif",
	"image/bmp":                                                                 "bmp",
	"image/gif":                                                                 "gif",
	"image/heic":                                                                "heic",
	"image/jpeg":                                                                "jpg",
	"image/jxl":                                                                 "jxl",
	"image/png":                                                                 "png",
	"image/svg+xml":                                                             "svg",
	"image/tiff":                                                                "tiff",
	"image/vnd.microsoft.icon":                                                  "ico",
	"image/webp":                                                                "webp",
	"image/x-icon":                                                              "ico",
	"text/calendar":                                                             "ics",
	"text/css":                                                                  "css",
	"text/csv":                                                                  "csv",
	"text/html":                                                                 "html",
	"text/javascript":                                                           "js",
	"text/markdown":                                                             "md",
	"text/plain":                                                                "txt",
	"text/vcard":                                                                "vcf",
	"text/xml":                                                                  "xml",
	"video/mp2t":                                                                "ts",
	"video/mp4":                                                                 "mp4",
	"video/mpeg":                                                                "mpeg",
	"video/ogg":                                                                 "ogv",
	"video/quicktime":                                                           "mov",
	"video/webm":                                                                "webm",
	"video/x-matroska":                                                          "mkv",
	"video/x-msvideo":                                                           "avi",
}

// mimeExt is a type's canonical extension, "" for a wildcard or an
// unknown type (the label then shows the type itself).
func mimeExt(mimeType string) string { return mimeExts[mimeType] }

// matchesEverything reports a * or *.* glob, so no redundant "All
// Files" is added.
func matchesEverything(f Filter) bool {
	return slices.ContainsFunc(f.Rules, func(r Rule) bool {
		return r.Kind == RuleGlob && (r.Value == "*" || r.Value == "*.*")
	})
}

// filterExt is the extension a filter names when it names one: a
// single *.ext glob (GTK's case classes read back as lower case), or a
// single MIME type with a canonical one; "" otherwise.
func filterExt(f Filter) string {
	if len(f.Rules) != 1 {
		return ""
	}
	r := f.Rules[0]
	if r.Kind == RuleMIME {
		return mimeExt(r.Value)
	}
	ext, ok := strings.CutPrefix(r.Value, "*.")
	if !ok || ext == "" {
		return ""
	}
	var b strings.Builder
	for i := 0; i < len(ext); i++ {
		switch {
		case ext[i] == '[' && i+3 < len(ext) && ext[i+3] == ']':
			b.WriteByte(strings.ToLower(ext[i+1 : i+2])[0])
			i += 3
		case strings.ContainsRune("*?[]", rune(ext[i])):
			return ""
		default:
			b.WriteByte(ext[i])
		}
	}
	return b.String()
}

// withExtension gives name the filter's extension in place of its own;
// a name or filter without one is left as it is.
func withExtension(name string, f Filter) string {
	ext := filterExt(f)
	if name == "" || ext == "" {
		return name
	}
	return strings.TrimSuffix(name, filepath.Ext(name)) + "." + ext
}

// allFiles is the escape hatch appended after a request's filters.
func allFiles() Filter {
	return Filter{Name: i18n.T("file-chooser-filter-all"), Rules: []Rule{{RuleGlob, "*"}}}
}

// withAllFiles appends All Files to a non-empty filter list that has
// no catch-all, so a restrictive app filter can always be bypassed.
func withAllFiles(filters []Filter) []Filter {
	if len(filters) == 0 || slices.ContainsFunc(filters, matchesEverything) {
		return filters
	}
	return append(slices.Clip(filters), allFiles())
}

// MatchesFilter reports whether name passes the active filter: no
// filters, an out-of-range index or an empty rule set pass everything;
// otherwise any one rule must match, a glob by pattern, a MIME rule
// against the type guessed from the name.
func MatchesFilter(db *mime.Database, name string, filters []Filter, active int) bool {
	if active < 0 || active >= len(filters) || len(filters[active].Rules) == 0 {
		return true
	}
	return slices.ContainsFunc(filters[active].Rules, func(r Rule) bool {
		if r.Kind == RuleMIME {
			return matchesMIME(db, name, r.Value)
		}
		return matchGlob(name, r.Value)
	})
}

// matchGlob matches a glob rule case-insensitively with full * ? [..]
// semantics: GTK 4's add_suffix arrives as a class glob
// (*.[pP][nN][gG]), so classes are not optional. An unparseable
// pattern compares as an exact name, never matching everything.
func matchGlob(name, pattern string) bool {
	p, ok := glob.Compile(pattern)
	if !ok {
		return strings.EqualFold(name, pattern)
	}
	return p.Matches(name, false)
}

// matchesMIME checks the type guessed from name: a type/* rule by
// prefix, else exact or a subtype.
func matchesMIME(db *mime.Database, name, rule string) bool {
	guessed := db.TypeByName(name)
	if prefix, ok := strings.CutSuffix(rule, "*"); ok {
		return strings.HasPrefix(guessed, prefix)
	}
	return db.IsA(guessed, rule)
}
