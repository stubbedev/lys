package filebrowser

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/vfs"
)

// session is one browse's state, free of widgets: where it is, how
// the listing is filtered and sorted, and the history. The browser
// renders entries and feeds gestures in.
type session struct {
	kinds Kinds
	dir   string
	// files is the type filter a file must pass to show; nil passes
	// every file.
	files func(name string) bool

	// all is the unfiltered listing of listed, re-read only when the
	// directory changes (or changed on disk): every other repaint
	// partitions it in memory; hiddenNames are the names its .hidden
	// lists, listErr is why it could not be read, and loading is set
	// while the listing is being read.
	all         []Entry
	hiddenNames map[string]bool
	listed      string
	listErr     error
	loading     bool
	// mounts are the filesystems folders are listed on; pending are the
	// changes that came while the listing was being read, applied once
	// it lands.
	mounts  *vfs.Mounts
	pending []dirChange
	// request reads dir's listing and hands it to setListing, at once
	// or later (the host reads off the loop).
	request func(dir string)
	// recentFile is the recently-used list the Recent place reads.
	recentFile string
	// trashDirs are the trash directories the Trash place lists.
	trashDirs func() []string

	search    string
	recursive bool
	hidden    bool
	// found accumulates the streamed recursive matches for search.
	found []Entry

	view view
	// cols read the Size and Kind columns the view sorts by, a file's
	// kind once per name through kindCache: a name's kind is its type's,
	// and a sort asks it of each name many times.
	cols      columns
	kindCache map[string]string
	// entries is what the view shows, in order.
	entries []Entry

	back, forward []string
}

// newSession starts at folder, else home, else /. Nothing is listed
// until the first refresh; listings are read at once until request is
// replaced.
func newSession(mounts *vfs.Mounts, kinds Kinds, folder string, files func(string) bool, home string, v view, cols columns) *session {
	s := &session{mounts: mounts, kinds: kinds, files: files, view: v, kindCache: map[string]string{}}
	s.cols = columns{kind: func(e Entry) string { return s.kindOf(e, cols.kind) }, items: cols.items}
	s.request = func(dir string) { s.setListing(dir, s.read(dir)) }
	s.dir = startDir(folder, home)
	return s
}

// UsableFolder is folder when it is one, else "": a stat, for off the
// loop, which a folder to start in passes first.
func UsableFolder(folder string) string {
	if folder == "" {
		return ""
	}
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		return ""
	}
	return folder
}

func startDir(folder, home string) string {
	if folder != "" {
		return folder
	}
	if home != "" {
		return home
	}
	return "/"
}

// kindOf is e's kind as kind names it, named once per name.
func (s *session) kindOf(e Entry, kind func(Entry) string) string {
	name := e.Name()
	k, ok := s.kindCache[name]
	if !ok {
		k = kind(e)
		s.kindCache[name] = k
	}
	return k
}

// searching reports a recursive search in progress: the rows are its
// matches, labelled relative to dir.
func (s *session) searching() bool { return s.recursive && s.search != "" }

// refresh recomputes entries: the cached listing (or the recursive
// matches) through the hidden rules, the type filter (files only) and
// the search text, then sorted by the column, or by relevance while
// searching. A folder not listed yet is requested, and shows nothing
// until its listing arrives.
func (s *session) refresh() {
	if !s.searching() && s.listed != s.dir {
		s.listed, s.all, s.hiddenNames, s.listErr, s.loading, s.pending = s.dir, nil, nil, nil, true, nil
		clear(s.kindCache)
		s.entries = s.entries[:0]
		s.request(s.dir)
		return
	}
	s.partition()
}

// reload reads the folder shown again (it changed on disk), showing
// the listing it has until the new one arrives.
func (s *session) reload() {
	s.loading = true
	s.request(s.dir)
}

// listing is one folder read: its entries, the names its .hidden
// lists, and why it could not be read.
type listing struct {
	entries []Entry
	hidden  map[string]bool
	err     error
}

// read lists dir, or the Recent or trashed files; safe off the loop.
func (s *session) read(dir string) listing {
	switch dir {
	case recentLocation:
		entries, err := ListRecent(s.mounts, s.recentFile, s.kinds)
		return listing{entries: entries, err: err}
	case TrashLocation:
		entries, err := ListTrash(s.mounts, s.trashDirs(), s.kinds)
		return listing{entries: entries, err: err}
	}
	entries, err := listDir(s.mounts, dir, s.kinds)
	return listing{entries: entries, hidden: hiddenNames(s.mounts, dir), err: err}
}

// setListing takes dir's listing when dir is still the folder awaited;
// false for one superseded.
func (s *session) setListing(dir string, l listing) bool {
	if dir != s.listed || !s.loading {
		return false
	}
	s.all, s.hiddenNames, s.listErr, s.loading = l.entries, l.hidden, l.err, false
	pending := s.pending
	s.pending = nil
	s.patch(pending)
	s.partition()
	return true
}

// dirChange is one name in a listed folder as stated after it changed:
// its entry, or gone; hidden is the folder's .hidden list read again
// when the change was to that file.
type dirChange struct {
	name    string
	entry   Entry
	present bool
	hidden  map[string]bool
}

// statChanges states the names of a change batch in dir; safe off the
// loop.
func statChanges(mounts *vfs.Mounts, dir string, names []string, kinds Kinds) []dirChange {
	changes := make([]dirChange, len(names))
	for i, name := range names {
		e, ok := statEntry(mounts, vfs.Join(dir, name), kinds)
		changes[i] = dirChange{name: name, entry: e, present: ok}
		if name == ".hidden" {
			changes[i].hidden = hiddenNames(mounts, dir)
		}
	}
	return changes
}

// applyChanges applies changes to dir's listing, row by row; false
// when dir is not the folder listed. Changes coming while the listing
// is read wait for it: each was stated after its event, so applied
// after the listing in order they leave every name as it last was.
func (s *session) applyChanges(dir string, changes []dirChange) bool {
	if dir != s.listed || VirtualLocation(dir) {
		return false
	}
	if s.loading {
		s.pending = append(s.pending, changes...)
		return false
	}
	s.patch(changes)
	s.refresh()
	return true
}

// patch applies changes to the listing, kept sorted by path.
func (s *session) patch(changes []dirChange) {
	prefix := dirPrefix(s.listed)
	for _, c := range changes {
		if c.hidden != nil || c.name == ".hidden" {
			s.hiddenNames = c.hidden
		}
		path := prefix + c.name
		i, found := slices.BinarySearchFunc(s.all, path, func(e Entry, p string) int { return strings.Compare(e.Path, p) })
		switch {
		case c.present && found:
			s.all[i] = c.entry
		case c.present:
			s.all = slices.Insert(s.all, i, c.entry)
		case found:
			s.all = slices.Delete(s.all, i, i+1)
		}
	}
}

// partition filters and sorts the listing (or the matches) into
// entries.
func (s *session) partition() {
	source := s.all
	if s.searching() {
		source = s.found
	}
	match := newTextFilter(s.search)
	s.entries = s.entries[:0]
	for _, e := range source {
		name := e.Name()
		if !s.hidden && hiddenName(name, s.hiddenNames) {
			continue
		}
		if !e.IsDir && s.files != nil && !s.files(name) {
			continue
		}
		// Recursive matches already matched the query on the walk.
		if !s.searching() && !match.matches(name) {
			continue
		}
		s.entries = append(s.entries, e)
	}
	if s.searching() {
		SortSearch(s.entries, s.dir, s.search)
	} else {
		sortEntries(s.entries, s.view.sort, s.view.asc, s.cols)
	}
}

// displayName is a row's label: the basename, or the path below dir
// for a recursive match.
func (s *session) displayName(e Entry) string {
	if s.searching() {
		if rel, err := filepath.Rel(s.dir, e.Path); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return e.Name()
}

// setSearch changes the query, dropping the previous matches.
func (s *session) setSearch(query string) {
	s.search, s.found = query, nil
	s.refresh()
}

// addFound appends a batch of recursive matches for query; a batch
// for a superseded query, or after the toggle went off, is dropped.
func (s *session) addFound(query string, batch []Entry) bool {
	if query != s.search || !s.recursive {
		return false
	}
	s.found = append(s.found, batch...)
	s.refresh()
	return true
}

// goTo navigates to dir, recording the departure in the back history
// (a fresh navigation forks it: forward clears) and dropping the
// search so the directory lists in full.
func (s *session) goTo(dir string) {
	if dir != s.dir {
		s.back = append(s.back, s.dir)
		s.forward = nil
		s.dir = dir
	}
	s.search, s.found = "", nil
	s.refresh()
}

// step walks the history one way; false at its end.
func (s *session) step(back bool) bool {
	from, to := &s.forward, &s.back
	if back {
		from, to = &s.back, &s.forward
	}
	if len(*from) == 0 {
		return false
	}
	dir := (*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
	*to = append(*to, s.dir)
	s.dir = dir
	s.search, s.found = "", nil
	s.refresh()
	return true
}

// canUp reports a parent to go up to: none at a filesystem's root, nor
// in Recent or Trash.
func (s *session) canUp() bool { return vfs.Dir(s.dir) != vfs.Clean(s.dir) && !VirtualLocation(s.dir) }

// up goes to the parent; false where there is none (canUp).
func (s *session) up() bool {
	if !s.canUp() {
		return false
	}
	s.goTo(vfs.Dir(s.dir))
	return true
}

// dropped navigates to a dropped folder, or reveals a dropped file in
// its folder.
func (s *session) dropped(path string, isDir bool) {
	if isDir {
		s.goTo(path)
		return
	}
	s.goTo(vfs.Dir(path))
}

// setFiles makes files the type filter.
func (s *session) setFiles(files func(string) bool) {
	s.files = files
	s.refresh()
}

// sortBy sorts by column, flipping the direction when it already is.
func (s *session) sortBy(column SortColumn) {
	if s.view.sort == column {
		s.view.asc = !s.view.asc
	} else {
		s.view.sort, s.view.asc = column, true
	}
	s.refresh()
}

// ResolvePath resolves typed text to the location it names: a
// network location (scheme://host/path) or a file:// URI, an absolute
// path, one under ~ (home), or one relative to dir, its . and ..
// resolved; "" for no text.
func ResolvePath(text, dir, home string) string {
	switch {
	case text == "":
		return ""
	case strings.Contains(text, "://"):
		if l, err := vfs.Parse(text); err == nil {
			return l.String()
		}
		return text
	case text == "~" || strings.HasPrefix(text, "~/"):
		text = filepath.Join(home, strings.TrimPrefix(text, "~"))
	case !filepath.IsAbs(text):
		return vfs.Clean(vfs.Join(dir, text))
	}
	return filepath.Clean(text)
}
