package filebrowser

import (
	"bufio"
	"cmp"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/stubbedev/lys/desktop/fileuri"
	"github.com/stubbedev/lys/desktop/glob"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/xdg"
)

// Kinds is what a listing holds.
type Kinds uint8

// The listings.
const (
	// FilesAndFolders lists everything.
	FilesAndFolders Kinds = iota
	// FoldersOnly lists the folders alone.
	FoldersOnly
)

// Entry is one listed file or directory.
type Entry struct {
	Path     string
	IsDir    bool
	Size     int64
	Modified time.Time
	// Label is the name shown in place of the basename: a trashed
	// file's original name; "" shows the basename.
	Label string
}

// Name is the entry's shown name: its label, else its basename.
func (e Entry) Name() string {
	if e.Label != "" {
		return e.Label
	}
	return filepath.Base(e.Path)
}

// kindNamer names an entry's type as the Kind column shows it, in the
// first of langs the shared-mime-info database describes it in.
type kindNamer struct {
	db    *mime.Database
	langs []string
}

// newKindNamer names types in the desktop locale's languages.
func newKindNamer(db *mime.Database) kindNamer {
	var langs []string
	for _, l := range i18n.Shell().Languages() {
		langs = append(langs, l.String())
	}
	return kindNamer{db: db, langs: langs}
}

// kind is Folder, the description of the type a file's name has (PNG
// image), else its extension upper cased (TAR), else File.
func (k kindNamer) kind(e Entry) string {
	if e.IsDir {
		return i18n.T("file-chooser-kind-folder")
	}
	if t := k.db.TypeByName(e.Name()); t != mime.Unknown {
		if c := k.db.CommentIn(t, k.langs); c != "" {
			return c
		}
	}
	if ext := strings.TrimPrefix(filepath.Ext(e.Name()), "."); ext != "" {
		return strings.ToUpper(ext)
	}
	return i18n.T("file-chooser-kind-file")
}

// columns are what the Size and Kind columns read of an entry beyond
// its stat: its kind, and a folder's item count, false while it is not
// counted or cannot be.
type columns struct {
	kind  func(Entry) string
	items func(Entry) (int, bool)
}

// itemsKey is what the Size column sorts a folder by: its item count,
// -1 for none known.
func (c columns) itemsKey(e Entry) int {
	if n, ok := c.items(e); ok {
		return n
	}
	return -1
}

// CountItems is how many entries the folder dir holds, hidden ones
// too, read in batches; safe off the loop.
func CountItems(dir string) (int, error) {
	f, err := os.Open(dir) //nolint:gosec // a folder the browser lists
	if err != nil {
		return 0, fmt.Errorf("count the items of %s: %w", dir, err)
	}
	defer func() { _ = f.Close() }()
	n := 0
	for {
		names, err := f.Readdirnames(countBatch)
		n += len(names)
		if errors.Is(err, io.EOF) {
			return n, nil
		}
		if err != nil {
			return 0, fmt.Errorf("count the items of %s: %w", dir, err)
		}
	}
}

// countBatch is how many names CountItems reads at once.
const countBatch = 256

// hiddenNames are the names the folder loc's .hidden file lists, one a
// line, which GTK hides with the dot files; none without one.
func hiddenNames(mounts *vfs.Mounts, loc string) map[string]bool {
	data := readHidden(mounts, loc)
	if data == nil {
		return nil
	}
	names := map[string]bool{}
	for line := range strings.SplitSeq(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names[line] = true
		}
	}
	return names
}

// hiddenName is GTK's rule for a name in a folder whose .hidden lists
// listed: a dot name, a ~ backup, or one listed.
func hiddenName(name string, listed map[string]bool) bool {
	return strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") || listed[name]
}

// The recursive search's bounds: at most searchCap matches, handed
// over searchBatch at a time so the first rows appear at once without
// a message per file.
const (
	searchCap   = 1000
	searchBatch = 64
)

// searchDepth bounds how deep a recursive search goes below its root:
// past it a folder is a loop a filesystem did not mark as a link.
const searchDepth = 64

// WalkSearch walks the folder root depth first, on whatever filesystem
// it lies on, handing every entry whose name matches query and kinds
// lists to emit in batches as they are found, and returns the match
// count. Hidden entries (GTK's rule) are skipped, and their subtrees not
// entered, unless showHidden; linked folders are not entered, which
// keeps the walk out of loops. A cancelled ctx stops the walk,
// mid-folder too.
func WalkSearch(ctx context.Context, mounts *vfs.Mounts, root, query string, kinds Kinds, showHidden bool, emit func([]Entry)) int {
	match := newTextFilter(query)
	var batch []Entry
	found := 0
	flush := func() {
		if len(batch) > 0 {
			emit(batch)
			batch = nil
		}
	}
	type folder struct {
		loc   string
		depth int
	}
	stack := []folder{{root, 0}}
walk:
	for len(stack) > 0 && found < searchCap {
		dir := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		fsys, p, err := mounts.Resolve(dir.loc)
		if err != nil {
			continue
		}
		infos, err := fsys.ReadDir(ctx, p)
		if err != nil {
			continue
		}
		slices.SortFunc(infos, func(a, b vfs.Info) int { return strings.Compare(a.Name, b.Name) })
		var listed map[string]bool
		if !showHidden {
			listed = hiddenNames(mounts, dir.loc)
		}
		for _, info := range infos {
			if ctx.Err() != nil {
				return found
			}
			if !showHidden && hiddenName(info.Name, listed) {
				continue
			}
			loc := vfs.Join(dir.loc, info.Name)
			if match.matches(info.Name) && (info.IsDir || kinds != FoldersOnly) {
				batch = append(batch, Entry{Path: loc, IsDir: info.IsDir, Size: info.Size, Modified: info.Modified})
				found++
				if len(batch) >= searchBatch {
					flush()
				}
				if found >= searchCap {
					break walk
				}
			}
			if info.IsDir && !info.Link && dir.depth < searchDepth {
				stack = append(stack, folder{loc, dir.depth + 1})
			}
		}
	}
	if ctx.Err() == nil {
		flush()
	}
	return found
}

// SortColumn is a list view column.
type SortColumn uint8

// The sortable columns.
const (
	SortName SortColumn = iota
	SortSize
	SortModified
	SortKind
)

var sortNames = [...]string{"name", "size", "modified", "kind"}

// SortByName orders directories first, then each by name.
func SortByName(entries []Entry) { sortEntries(entries, SortName, true, columns{}) }

// sortEntries orders directories first, then by the column as cols
// read it: folders by their item count under Size, and by name under
// Kind, which is one for them all.
func sortEntries(entries []Entry, key SortColumn, asc bool, cols columns) {
	byName := func(a, b Entry) int { return compareFold(a.Name(), b.Name()) }
	slices.SortStableFunc(entries, func(a, b Entry) int {
		if a.IsDir != b.IsDir {
			if a.IsDir {
				return -1
			}
			return 1
		}
		var c int
		switch {
		case key == SortSize && a.IsDir:
			c = cmp.Or(cmp.Compare(cols.itemsKey(a), cols.itemsKey(b)), byName(a, b))
		case key == SortSize:
			c = cmp.Compare(a.Size, b.Size)
		case key == SortModified:
			c = a.Modified.Compare(b.Modified)
		case key == SortKind && !a.IsDir:
			c = cmp.Or(compareFold(cols.kind(a), cols.kind(b)), byName(a, b))
		default:
			c = byName(a, b)
		}
		if !asc {
			c = -c
		}
		return c
	})
}

// searchRank scores a recursive hit, lower first: an exact name before
// a prefix before a substring, then shallower under root, then the
// earlier the match falls in the name.
func searchRank(e Entry, root, query string) [3]int {
	name := strings.ToLower(e.Name())
	class := 2
	switch {
	case name == query:
		class = 0
	case strings.HasPrefix(name, query):
		class = 1
	}
	rel, err := filepath.Rel(root, e.Path)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = e.Path
	}
	depth := len(strings.Split(strings.Trim(rel, "/"), "/"))
	pos := strings.Index(name, query)
	if pos < 0 {
		pos = 1 << 30
	}
	return [3]int{class, depth, pos}
}

// SortSearch orders recursive hits by relevance, then by path so the
// order is stable.
func SortSearch(entries []Entry, root, query string) {
	q := strings.ToLower(query)
	slices.SortFunc(entries, func(a, b Entry) int {
		ra, rb := searchRank(a, root, q), searchRank(b, root, q)
		return cmp.Or(slices.Compare(ra[:], rb[:]), strings.Compare(a.Path, b.Path))
	})
}

// textFilter is the search box's predicate over a basename: a
// case-insensitive substring, or a glob when the query carries * ? or [.
type textFilter struct {
	needle  string
	pattern *glob.Pattern
}

func newTextFilter(query string) textFilter {
	f := textFilter{needle: strings.ToLower(query)}
	if strings.ContainsAny(query, "*?[") {
		if p, ok := glob.Compile(query); ok {
			f.pattern = &p
		}
	}
	return f
}

func (f textFilter) matches(name string) bool {
	switch {
	case f.needle == "":
		return true
	case f.pattern != nil:
		return f.pattern.Matches(name, false)
	}
	return containsFold(name, f.needle)
}

// compareFold compares a and b as their lower case would, without
// making it.
func compareFold(a, b string) int {
	for a != "" && b != "" {
		ra, na := utf8.DecodeRuneInString(a)
		rb, nb := utf8.DecodeRuneInString(b)
		if c := cmp.Compare(unicode.ToLower(ra), unicode.ToLower(rb)); c != 0 {
			return c
		}
		a, b = a[na:], b[nb:]
	}
	return cmp.Compare(len(a), len(b))
}

// containsFold reports whether s, lowered, contains lower, which is
// lower case already.
func containsFold(s, lower string) bool {
	if lower == "" {
		return true
	}
	for i := range s {
		if hasPrefixFold(s[i:], lower) {
			return true
		}
	}
	return false
}

func hasPrefixFold(s, lower string) bool {
	for lower != "" {
		if s == "" {
			return false
		}
		rs, ns := utf8.DecodeRuneInString(s)
		rl, nl := utf8.DecodeRuneInString(lower)
		if unicode.ToLower(rs) != rl {
			return false
		}
		s, lower = s[ns:], lower[nl:]
	}
	return true
}

// HumanSize is a file size in the desktop locale's binary units.
func HumanSize(n int64) string { return i18n.Shell().ByteSize(intconv.Clamp[uint64](n)) }

// FormatTime is a modification time in local time, in the desktop
// locale's numeric date and time, "" for none.
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return i18n.Shell().DateTime(t.Local())
}

// FormatModified is a modification time as the list reads it, on the
// calendar of now's location: the time alone on now's day, Yesterday
// the day before, the weekday on the five before that, else the
// locale's medium date (a time ahead of now too); "" for none.
func FormatModified(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	t = t.In(now.Location())
	l := i18n.Shell()
	switch days := daysBetween(t, now); {
	case days == 0:
		return l.Time(t)
	case days == 1:
		return i18n.T("file-chooser-modified-yesterday")
	case days > 1 && days < 7:
		return l.Weekday(t)
	}
	return l.Date(t)
}

// daysBetween is how many calendar days t's day falls before now's, in
// their own location, whatever daylight saving does between them.
func daysBetween(t, now time.Time) int {
	day := func(at time.Time) time.Time {
		y, m, d := at.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	return int(day(now).Sub(day(t)).Hours() / 24)
}

// Place is a sidebar shortcut; Bookmark marks one from the GTK
// bookmarks file, which the sidebar can remove.
type Place struct {
	Label, Path, Icon string
	Bookmark          bool
}

// userPlaceDirs are the user directories offered when present: the
// user-dirs.dirs key, the name under home when it sets none, the icon.
var userPlaceDirs = []struct{ key, name, icon string }{
	{"DESKTOP", "Desktop", "ld-monitor-symbolic"},
	{"DOCUMENTS", "Documents", "ld-file-text-symbolic"},
	{"DOWNLOAD", "Downloads", "ld-download-symbolic"},
	{"MUSIC", "Music", "ld-music-symbolic"},
	{"PICTURES", "Pictures", "ld-image-symbolic"},
	{"VIDEOS", "Videos", "ld-video-symbolic"},
}

// UserPlaces is Home, the user directories that exist (where env's
// user-dirs.dirs puts them, under their localized names), then the GTK
// bookmarks. A user directory user-dirs.dirs cannot say, or a
// bookmarks file that cannot be read, is left out, and the error names
// it.
func UserPlaces(env xdg.Env, home string) ([]Place, error) {
	if home == "" {
		return nil, nil
	}
	places := []Place{{Label: i18n.T("file-chooser-place-home"), Path: home, Icon: "ld-house-symbolic"}}
	var errs []error
	for _, d := range userPlaceDirs {
		path, ok, err := env.UserDir(d.key)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !ok {
			path = filepath.Join(home, d.name)
		}
		// user-dirs.dirs points a directory it disables at home.
		if path == home {
			continue
		}
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			places = append(places, Place{Label: filepath.Base(path), Path: path, Icon: d.icon})
		}
	}
	bookmarks, err := gtkBookmarks(env)
	if err != nil {
		errs = append(errs, err)
	}
	return append(places, bookmarks...), errors.Join(errs...)
}

// gtkBookmarks are the folders in GTK 3's bookmarks file, which GTK 4
// reads too; none when there is no config directory or no file.
func gtkBookmarks(env xdg.Env) ([]Place, error) {
	file := BookmarksFile(env)
	if file == "" {
		return nil, nil
	}
	f, err := os.Open(file) //nolint:gosec // the user's own GTK bookmarks
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gtk bookmarks: %w", err)
	}
	defer func() { _ = f.Close() }()
	places, err := bookmarks(f)
	if err != nil {
		return nil, fmt.Errorf("gtk bookmarks: reading %s: %w", f.Name(), err)
	}
	return places, nil
}

// bookmarks are the local folders a GTK bookmarks file lists, one
// "<uri> [label]" a line, labelled by their label else their name;
// remote ones and folders that are gone are left out.
func bookmarks(r io.Reader) ([]Place, error) {
	var places []Place
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		uri, label, _ := strings.Cut(strings.TrimSpace(sc.Text()), " ")
		path, ok := fileuri.Path(uri)
		if !ok {
			continue
		}
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			continue
		}
		if label = strings.TrimSpace(label); label == "" {
			label = filepath.Base(path)
		}
		places = append(places, Place{Label: label, Path: path, Icon: "ld-folder-symbolic", Bookmark: true})
	}
	return places, sc.Err()
}

// recentLocation is the Recent place's directory: not a path, the
// listing of the recently used files.
const recentLocation = "recent:"

// maxRecent bounds the Recent listing to the newest files.
const maxRecent = 100

// RecentFile is the recently-used list GTK writes,
// $XDG_DATA_HOME/recently-used.xbel; "" without a data home.
func RecentFile() string {
	data, ok := xdg.OS.DataHome()
	if !ok {
		return ""
	}
	return filepath.Join(data, "recently-used.xbel")
}

// xbel is the part of a recently-used.xbel the listing reads.
type xbel struct {
	Bookmarks []xbelBookmark `xml:"bookmark"`
}

// xbelBookmark is one used file: its URI and when it was last used
// (an ISO 8601 UTC stamp, which sorts as text).
type xbelBookmark struct {
	Href     string `xml:"href,attr"`
	Modified string `xml:"modified,attr"`
}

// ListRecent lists the recently used local files that still exist,
// the newest maxRecent of them; an unreadable list lists none.
func ListRecent(mounts *vfs.Mounts, path string, kinds Kinds) ([]Entry, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the user's recently-used list
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc xbel
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	slices.SortStableFunc(doc.Bookmarks, func(a, b xbelBookmark) int { return strings.Compare(b.Modified, a.Modified) })
	var entries []Entry
	for _, b := range doc.Bookmarks {
		p, ok := fileuri.Path(b.Href)
		if !ok {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if e, ok := statEntry(mounts, p, kinds); ok && !e.IsDir {
			entries = append(entries, e)
			if len(entries) == maxRecent {
				break
			}
		}
	}
	return entries, nil
}

// OtherLocations is Computer (/) plus the user-visible mounts in a
// /proc/self/mounts listing: what gio's volume monitor shows for unix
// mounts, those under /media, /run/media/<user> and the home
// directory (not hidden ones).
func OtherLocations(mounts io.Reader, home, user string) []Place {
	locations := []Place{{Label: i18n.T("file-chooser-place-computer"), Path: "/", Icon: "ld-hard-drive-symbolic"}}
	for _, path := range VisibleMounts(mounts, home, user) {
		locations = append(locations, Place{Label: filepath.Base(path), Path: path, Icon: "ld-usb-symbolic"})
	}
	return locations
}

// VisibleMounts are the user-visible mount points in a
// /proc/self/mounts listing, each once, in its order; none for a nil
// listing.
func VisibleMounts(mounts io.Reader, home, user string) []string {
	if mounts == nil {
		return nil
	}
	var paths []string
	seen := map[string]bool{}
	sc := bufio.NewScanner(mounts)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		path := unescapeMount(fields[1])
		if seen[path] || !userVisibleMount(path, home, user) {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
	}
	return paths
}

// userVisibleMount is g_unix_mount_guess_should_display's path rule.
func userVisibleMount(path, home, user string) bool {
	under := func(dir string) bool {
		rest, ok := strings.CutPrefix(path, dir+"/")
		return ok && rest != "" && !strings.HasPrefix(rest, ".") && !strings.Contains(rest, "/")
	}
	if under("/media") || (user != "" && under("/run/media/"+user)) {
		return true
	}
	if home != "" && home != "/" {
		rest, ok := strings.CutPrefix(path, home+"/")
		return ok && rest != "" && !strings.HasPrefix(rest, ".")
	}
	return false
}

// unescapeMount resolves /proc/mounts' octal escapes (\040 a space).
func unescapeMount(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) && isOctal(s[i+1]) && isOctal(s[i+2]) && isOctal(s[i+3]) {
			b.WriteByte((s[i+1]-'0')<<6 | (s[i+2]-'0')<<3 | (s[i+3] - '0'))
			i += 3
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isOctal(c byte) bool { return c >= '0' && c <= '7' }

// Crumb is one breadcrumb button.
type Crumb struct {
	Label, Path string
	Current     bool
}

// maxCrumbTail is how many trailing segments stay visible; deeper
// ones collapse into one … crumb.
const maxCrumbTail = 3

// Crumbs is dir's breadcrumb trail: Home for the home directory (else
// /), its named segments, and past maxCrumbTail of them a … crumb
// standing for (and jumping to) the deepest collapsed one. Recent is
// one crumb. A network location's trail starts at its server.
func Crumbs(dir, home string) []Crumb {
	switch dir {
	case recentLocation:
		return []Crumb{{i18n.T("file-chooser-place-recent"), recentLocation, true}}
	case TrashLocation:
		return []Crumb{{i18n.T("file-chooser-place-trash"), TrashLocation, true}}
	}
	base, baseLabel := "/", "/"
	rest := strings.TrimPrefix(dir, "/")
	if vfs.IsRemote(dir) {
		base = vfs.Root(dir)
		baseLabel = vfs.Base(base)
		rest = strings.TrimPrefix(dir, base)
	} else if home != "" && (dir == home || strings.HasPrefix(dir, strings.TrimSuffix(home, "/")+"/")) {
		base, baseLabel = home, i18n.T("file-chooser-place-home")
		rest = strings.TrimPrefix(strings.TrimPrefix(dir, home), "/")
	}
	var names []string
	for n := range strings.SplitSeq(rest, "/") {
		if n != "" {
			names = append(names, n)
		}
	}
	crumbs := []Crumb{{baseLabel, base, len(names) == 0}}
	acc := base
	collapse := max(0, len(names)-maxCrumbTail)
	for i, n := range names {
		acc = vfs.Join(acc, n)
		switch {
		case collapse > 0 && i+1 == collapse:
			crumbs = append(crumbs, Crumb{"…", acc, false})
		case i >= collapse:
			crumbs = append(crumbs, Crumb{n, acc, i+1 == len(names)})
		}
	}
	return crumbs
}

// view is the sticky view preference: the sort and the list or grid.
type view struct {
	sort SortColumn
	asc  bool
	grid bool
}

var defaultView = view{sort: SortName, asc: true}

// parseView reads "<sort> <asc|desc> <list|grid>", defaulting a
// missing or unknown token.
func parseView(text string) view {
	v := defaultView
	tokens := strings.Fields(text)
	tok := func(i int) string {
		if i < len(tokens) {
			return tokens[i]
		}
		return ""
	}
	for col := range SortColumn(len(sortNames)) {
		if sortNames[col] == tok(0) {
			v.sort = col
			break
		}
	}
	v.asc = tok(1) != "desc"
	v.grid = tok(2) == "grid"
	return v
}

// String is the persisted form parseView reads.
func (v view) String() string {
	dir, layout := "asc", "list"
	if !v.asc {
		dir = "desc"
	}
	if v.grid {
		layout = "grid"
	}
	return sortNames[v.sort] + " " + dir + " " + layout + "\n"
}

// countItemsOn counts the items of the folder at loc: a local one as
// CountItems does, one on a network location by listing it there.
func countItemsOn(mounts *vfs.Mounts, loc string) (int, error) {
	if !vfs.IsRemote(loc) {
		return CountItems(loc)
	}
	fsys, p, err := mounts.Resolve(loc)
	if err != nil {
		return 0, fmt.Errorf("count the items of %s: %w", loc, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
	defer cancel()
	infos, err := fsys.ReadDir(ctx, p)
	if err != nil {
		return 0, fmt.Errorf("count the items of %s: %w", loc, err)
	}
	return len(infos), nil
}
