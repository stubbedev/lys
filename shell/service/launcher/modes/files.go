package modes

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/pango"

	"github.com/stubbedev/lys/shell/service/fileindex"
	"github.com/stubbedev/lys/shell/service/launcher"
)

// FileRows is how many files the files mode lists for a query.
const FileRows = 20

// TrailRunes is how long a query is before file rows compete with the
// other modes' in a combined list; shorter, they come after.
const TrailRunes = 4

// FileIndex is what the files mode searches (*fileindex.Index).
type FileIndex interface {
	Snapshot() *fileindex.Snapshot
}

// FilesConfig is the files mode's knobs.
type FilesConfig struct {
	Index FileIndex
	// Home shortens the folders shown to ~.
	Home string
	// Terminal runs a Terminal=true opener; empty autodetects.
	Terminal string
	// Negate is the matcher's negation char, whose words the index leaves
	// to the matcher.
	Negate rune
	// ShowItem shows a file selected in its folder in the file manager
	// (launcher.ShowItem).
	ShowItem func(ctx context.Context, path string) error
}

// Files finds files by name in the index lys-shell keeps: recently used
// ones first, an empty query listing those alone. Enter opens a file in
// its default app; the alternate accept shows it in the file manager.
type Files struct {
	cfg  FilesConfig
	hits []fileindex.Hit
}

// NewFiles builds the mode over cfg.Index.
func NewFiles(cfg FilesConfig) *Files { return &Files{cfg: cfg} }

// Name is "files".
func (*Files) Name() string { return "files" }

// AllowsCustom refuses typed text: it names no file.
func (*Files) AllowsCustom() bool { return false }

// Trails holds for a query shorter than TrailRunes.
func (*Files) Trails(query string) bool {
	return utf8.RuneCountInString(strings.TrimSpace(query)) < TrailRunes
}

// Load lists the recent files.
func (f *Files) Load(context.Context) launcher.ModeState {
	recent := f.cfg.Index.Snapshot().Recent
	f.hits = make([]fileindex.Hit, 0, min(len(recent), FileRows))
	for _, p := range recent[:min(len(recent), FileRows)] {
		f.hits = append(f.hits, fileindex.Hit{Path: p, Name: filepath.Base(p), Recent: true})
	}
	return f.state()
}

// Query searches the index; the list changes only when the files found
// do, and a search a newer query ended leaves it.
func (f *Files) Query(ctx context.Context, query string) (launcher.ModeState, bool) {
	q := fileindex.ParseQuery(query, f.cfg.Negate)
	if q.Empty() {
		before := f.hits
		state := f.Load(ctx)
		return state, !slices.Equal(before, f.hits)
	}
	hits, err := fileindex.Search(ctx, f.cfg.Index.Snapshot(), q, FileRows)
	if err != nil || slices.Equal(hits, f.hits) {
		return launcher.ModeState{}, false
	}
	f.hits = hits
	return f.state(), true
}

func (f *Files) state() launcher.ModeState {
	items := make([]launcher.Item, len(f.hits))
	for i, h := range f.hits {
		items[i] = f.item(h)
	}
	return launcher.ModeState{Items: items, Prompt: "files", NoCustom: true}
}

// item is a file's row: its name over its folder, a thumbnail drawn
// only once the row is, and the name to match.
func (f *Files) item(h fileindex.Hit) launcher.Item {
	var icon launcher.Icon = launcher.IconName("ld-folder-symbolic")
	if !h.Dir {
		icon = launcher.IconThumbnail{Path: h.Path, Fallback: launcher.MimeIcon(h.Path)}
	}
	display := pango.Escape(h.Name) + "  <i>" + pango.Escape(f.folder(h.Path)) + "</i>"
	return launcher.Item{Display: display, MatchText: h.Name, Icon: icon, Info: &h.Path, Flags: launcher.FlagMarkup}
}

// folder is path's folder as shown: under the home directory, ~/.
func (f *Files) folder(path string) string {
	dir := filepath.Dir(path)
	if f.cfg.Home == "" {
		return dir
	}
	if dir == f.cfg.Home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(dir, f.cfg.Home+"/"); ok {
		return "~/" + rest
	}
	return dir
}

// Activate opens the file, or shows it in the file manager on the
// alternate accept.
func (f *Files) Activate(ctx context.Context, target launcher.Target, kind launcher.ActivateKind, _ string) launcher.Action {
	i, ok := target.Row()
	if !ok || int(i) >= len(f.hits) {
		return launcher.ActionNothing{}
	}
	h := f.hits[i]
	if _, alt := kind.(launcher.ActivateAlt); alt {
		if err := f.cfg.ShowItem(ctx, h.Path); err != nil {
			return failed(f.state(), i18n.T("launcher-files-show-failed", i18n.Str("what", h.Name)), err)
		}
		return launcher.ActionClose{}
	}
	if err := openFile(h.Path, "", f.cfg.Terminal); err != nil {
		return failed(f.state(), RanFailed(h.Name, err), err)
	}
	return launcher.ActionClose{}
}
