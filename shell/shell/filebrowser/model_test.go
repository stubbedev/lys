package filebrowser

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/fileuri"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/xdg"

	"github.com/stubbedev/lys/shell/shell/filebrowser/browsertest"
)

func entry(path string, dir bool, size int64) Entry { return Entry{Path: path, IsDir: dir, Size: size} }

func names(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name()
	}
	return out
}

func TestHumanSizeIsTheLocaleByteSize(t *testing.T) {
	for _, n := range []int64{0, 512, 1536, 3 << 30} {
		if got, want := HumanSize(n), i18n.Shell().ByteSize(uint64(n)); got != want {
			t.Errorf("HumanSize(%d) = %q, want the locale's %q", n, got, want)
		}
	}
	if got, want := HumanSize(-1), i18n.Shell().ByteSize(0); got != want {
		t.Errorf("HumanSize(-1) = %q, want a negative size clamped to %q", got, want)
	}
}

func TestFormatTimeIsTheLocaleDateTimeInLocalTime(t *testing.T) {
	if got := FormatTime(time.Time{}); got != "" {
		t.Errorf("no time = %q, want empty", got)
	}
	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	if got, want := FormatTime(at), i18n.Shell().DateTime(at.Local()); got != want {
		t.Errorf("FormatTime = %q, want the locale's local %q", got, want)
	}
}

func TestTextFilterIsSubstringThenGlob(t *testing.T) {
	plain := newTextFilter("REP")
	if !plain.matches("report.txt") || !plain.matches("my-Report") || plain.matches("notes.txt") {
		t.Error("a plain query is a case-insensitive substring")
	}
	if !newTextFilter("").matches("anything") {
		t.Error("an empty query passes everything")
	}
	globbed := newTextFilter("*.rs")
	if !globbed.matches("main.rs") || !globbed.matches("MAIN.RS") || globbed.matches("main.rs.bak") {
		t.Error("a metacharacter makes the query a whole-name glob")
	}
	// A malformed glob stays a substring.
	if !newTextFilter("[a").matches("x[a]y") {
		t.Error("an unparseable glob query falls back to the substring")
	}
}

func TestSortEntriesKeepsDirsFirst(t *testing.T) {
	entries := []Entry{entry("b.txt", false, 10), entry("Adir", true, 0), entry("a.txt", false, 30), entry("Zdir", true, 0)}
	SortEntries(entries, SortName, true)
	if got := names(entries); !slices.Equal(got, []string{"Adir", "Zdir", "a.txt", "b.txt"}) {
		t.Errorf("name asc = %v", got)
	}
	SortEntries(entries, SortName, false)
	if got := names(entries); !slices.Equal(got, []string{"Zdir", "Adir", "b.txt", "a.txt"}) {
		t.Errorf("name desc = %v (folders stay first)", got)
	}
	SortEntries(entries, SortSize, true)
	if got := names(entries); !entries[0].IsDir || !entries[1].IsDir || !slices.Equal(got[2:], []string{"b.txt", "a.txt"}) {
		t.Errorf("size asc = %v", got)
	}
	kinds := []Entry{entry("b.png", false, 0), entry("a.txt", false, 0), entry("a.png", false, 0)}
	SortEntries(kinds, SortKind, true)
	if got := names(kinds); !slices.Equal(got, []string{"a.png", "b.png", "a.txt"}) {
		t.Errorf("kind asc = %v, want grouped by type then name", got)
	}
	old, newer := entry("old", false, 0), entry("new", false, 0)
	old.Modified, newer.Modified = time.Unix(100, 0), time.Unix(200, 0)
	byTime := []Entry{newer, old}
	SortEntries(byTime, SortModified, true)
	if got := names(byTime); !slices.Equal(got, []string{"old", "new"}) {
		t.Errorf("modified asc = %v", got)
	}
}

func TestEntryKindLabelsFolderExtensionOrFile(t *testing.T) {
	for e, want := range map[Entry]string{
		entry("/x/docs", true, 0):         "Folder",
		entry("/x/a.png", false, 0):       "PNG",
		entry("/x/archive.TAR", false, 0): "TAR",
		entry("/x/Makefile", false, 0):    "File",
	} {
		if got := e.Kind(); got != want {
			t.Errorf("Kind(%s) = %q, want %q", e.Path, got, want)
		}
	}
}

func TestSearchRanksCloseAndWellMatchedFirst(t *testing.T) {
	mk := func(rel string) Entry { return entry("/home/u/"+rel, false, 0) }
	entries := []Entry{mk("a/b/c/d/deep-php-helper.rs"), mk("php.ini"), mk("src/php-config.txt"), mk("vendor/x/y/php")}
	SortSearch(entries, "/home/u", "PHP")
	want := []string{"php", "php.ini", "php-config.txt", "deep-php-helper.rs"}
	if got := names(entries); !slices.Equal(got, want) {
		t.Errorf("search order = %v, want %v", got, want)
	}
}

func TestWalkSearchRecursesAndHonorsHidden(t *testing.T) {
	root := t.TempDir()
	browsertest.Touch(t, root, "top_match.txt", "plain.txt", "sub/nested_match.txt", ".hidden/buried_match.txt", ".secret_match.txt")
	run := func(ctx context.Context, query string, hidden bool) []string {
		var out []string
		WalkSearch(ctx, root, query, hidden, func(batch []string) {
			for _, p := range batch {
				out = append(out, filepath.Base(p))
			}
		})
		slices.Sort(out)
		return out
	}
	live := context.Background()
	if got := run(live, "match", false); !slices.Equal(got, []string{"nested_match.txt", "top_match.txt"}) {
		t.Errorf("hidden excluded = %v", got)
	}
	if got := run(live, "MATCH", false); len(got) != 2 {
		t.Errorf("case-insensitive query = %v", got)
	}
	want := []string{".secret_match.txt", "buried_match.txt", "nested_match.txt", "top_match.txt"}
	if got := run(live, "match", true); !slices.Equal(got, want) {
		t.Errorf("hidden shown = %v, want %v", got, want)
	}
	cancelled, cancel := context.WithCancel(live)
	cancel()
	if got := run(cancelled, "match", false); len(got) != 0 {
		t.Errorf("a cancelled walk emitted %v", got)
	}
}

func TestWalkSearchDoesNotFollowSymlinkedDirectories(t *testing.T) {
	root := t.TempDir()
	browsertest.Touch(t, root, "real/match_a")
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(root, "real", "loop")); err != nil {
		t.Fatal(err)
	}
	var got []string
	WalkSearch(context.Background(), root, "match", false, func(b []string) { got = append(got, b...) })
	if len(got) != 1 || got[0] != filepath.Join(root, "real", "match_a") {
		t.Errorf("walk through symlinks = %v, want only the real match", got)
	}
}

func TestWalkSearchStreamsBatchesAndCaps(t *testing.T) {
	root := t.TempDir()
	for i := range searchCap + 10 {
		browsertest.Touch(t, root, filepath.Join("d", "m"+strings.Repeat("x", i%7)+string(rune('a'+i%26))+"_"+time.Duration(i).String()))
	}
	var batches []int
	total := WalkSearch(context.Background(), root, "m", false, func(b []string) { batches = append(batches, len(b)) })
	if total != searchCap {
		t.Errorf("found %d, want the cap %d", total, searchCap)
	}
	sum := 0
	for i, n := range batches {
		sum += n
		if n > searchBatch || (i < len(batches)-1 && n != searchBatch) {
			t.Errorf("batch %d holds %d, want %d (the last may be short)", i, n, searchBatch)
		}
	}
	if sum != searchCap {
		t.Errorf("batches carried %d, want %d", sum, searchCap)
	}
}

func TestListDirReturnsEverythingButHonorsFolderMode(t *testing.T) {
	root := t.TempDir()
	browsertest.Touch(t, root, "sub/", "visible.txt", ".hidden.txt")
	all, _, err := testReader.List(root, FilesAndFolders)
	got := names(all)
	slices.Sort(got)
	if err != nil || !slices.Equal(got, []string{".hidden.txt", "sub", "visible.txt"}) {
		t.Errorf("listing = %v, %v; want everything", got, err)
	}
	folders, _, _ := testReader.List(root, FoldersOnly)
	if got := names(folders); !slices.Equal(got, []string{"sub"}) {
		t.Errorf("folder mode = %v, want only folders", got)
	}
	if got, _, err := testReader.List(filepath.Join(root, "missing"), FilesAndFolders); got != nil || err == nil {
		t.Errorf("an unreadable directory = %v, %v; want none and the error", got, err)
	}
}

func homeEnv(home string) xdg.Env {
	return func(name string) string {
		switch name {
		case "HOME":
			return home
		case "XDG_CONFIG_HOME":
			return filepath.Join(home, ".config")
		}
		return ""
	}
}

func placeLabels(t *testing.T, home string) []string {
	t.Helper()
	places, err := UserPlaces(homeEnv(home), home)
	if err != nil {
		t.Fatalf("UserPlaces: %v", err)
	}
	var got []string
	for _, p := range places {
		got = append(got, p.Label+"="+p.Path)
	}
	return got
}

func TestUserPlacesListsHomeAndExistingDirs(t *testing.T) {
	home := t.TempDir()
	browsertest.Touch(t, home, "Documents/", "Pictures/", "Music")
	// Music is a file, not a folder, so it is not offered.
	want := []string{"Home=" + home, "Documents=" + filepath.Join(home, "Documents"), "Pictures=" + filepath.Join(home, "Pictures")}
	if got := placeLabels(t, home); !slices.Equal(got, want) {
		t.Errorf("places = %v, want %v", got, want)
	}
	if places, err := UserPlaces(homeEnv(""), ""); places != nil || err != nil {
		t.Errorf("no home = %v, %v; want no places", places, err)
	}
}

// A user directory user-dirs.dirs cannot say is left out and named in
// the error, never guessed at its default name; the others stay.
func TestUserPlacesReportsAnUnreadableUserDir(t *testing.T) {
	home := t.TempDir()
	browsertest.Touch(t, home, "Documents/", "Bilder/", ".config/user-dirs.dirs")
	body := "XDG_DOCUMENTS_DIR=Documents\nXDG_PICTURES_DIR=\"$HOME/Bilder\"\n"
	if err := os.WriteFile(filepath.Join(home, ".config", "user-dirs.dirs"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	places, err := UserPlaces(homeEnv(home), home)
	if err == nil || !strings.Contains(err.Error(), "DOCUMENTS") {
		t.Errorf("err = %v, want one naming DOCUMENTS", err)
	}
	var got []string
	for _, p := range places {
		got = append(got, p.Label)
	}
	if !slices.Equal(got, []string{"Home", "Bilder"}) {
		t.Errorf("places = %v, want Home and Bilder", got)
	}
}

func TestUserPlacesReportsAnUnreadableBookmarksFile(t *testing.T) {
	home := t.TempDir()
	browsertest.Touch(t, home, ".config/gtk-3.0/bookmarks/")
	places, err := UserPlaces(homeEnv(home), home)
	if err == nil || !strings.Contains(err.Error(), "bookmarks") {
		t.Errorf("err = %v, want one naming the bookmarks", err)
	}
	if len(places) != 1 || places[0].Label != "Home" {
		t.Errorf("places = %v, want Home only", places)
	}
}

func TestOtherLocationsShowsUserVisibleMounts(t *testing.T) {
	mounts := strings.Join([]string{
		"/dev/nvme0n1p2 / ext4 rw 0 0",
		"proc /proc proc rw 0 0",
		"/dev/sda1 /run/media/u/USB\\040STICK vfat rw 0 0",
		"/dev/sda2 /run/media/other/THEIRS vfat rw 0 0",
		"/dev/sdb1 /media/Backup ext4 rw 0 0",
		"/dev/sdb1 /media/Backup ext4 rw 0 0",
		"/dev/sdc1 /home/u/Mounts ext4 rw 0 0",
		"/dev/sdc2 /home/u/.cache/x ext4 rw 0 0",
		"tmpfs /run/user/1000 tmpfs rw 0 0",
	}, "\n")
	var got []string
	for _, p := range OtherLocations(strings.NewReader(mounts), "/home/u", "u") {
		got = append(got, p.Label+"="+p.Path)
	}
	want := []string{"Computer=/", "USB STICK=/run/media/u/USB STICK", "Backup=/media/Backup", "Mounts=/home/u/Mounts"}
	if !slices.Equal(got, want) {
		t.Errorf("locations = %v, want %v", got, want)
	}
	if got := OtherLocations(nil, "/home/u", "u"); len(got) != 1 {
		t.Errorf("no mount table = %v, want just Computer", got)
	}
}

func TestCrumbsCollapseHomeAndDepth(t *testing.T) {
	render := func(cs []Crumb) string {
		var parts []string
		for _, c := range cs {
			s := c.Label + "=" + c.Path
			if c.Current {
				s += "*"
			}
			parts = append(parts, s)
		}
		return strings.Join(parts, " ")
	}
	for _, c := range []struct{ dir, want string }{
		{"/home/u", "Home=/home/u*"},
		{"/home/u/a/b", "Home=/home/u a=/home/u/a b=/home/u/a/b*"},
		{"/", "/=/*"},
		{"/etc/nix", "/=/ etc=/etc nix=/etc/nix*"},
		// A sibling of home sharing its prefix is not under it.
		{"/home/user2", "/=/ home=/home user2=/home/user2*"},
		// Past three segments the deeper ones fold into …, which jumps
		// to the last folded one.
		{"/home/u/a/b/c/d/e", "Home=/home/u …=/home/u/a/b c=/home/u/a/b/c d=/home/u/a/b/c/d e=/home/u/a/b/c/d/e*"},
	} {
		if got := render(Crumbs(c.dir, "/home/u")); got != c.want {
			t.Errorf("Crumbs(%s) = %s\n want %s", c.dir, got, c.want)
		}
	}
}

func TestViewRoundTripsAndDefaults(t *testing.T) {
	for _, v := range []view{defaultView, {SortSize, false, true}, {SortKind, true, false}, {SortModified, false, false}} {
		if got := parseView(v.String()); got != v {
			t.Errorf("round trip %+v = %+v", v, got)
		}
	}
	if got := parseView(""); got != defaultView {
		t.Errorf("empty = %+v, want the default", got)
	}
	if got := parseView("bogus sideways mosaic"); got != defaultView {
		t.Errorf("garbage = %+v, want the default", got)
	}
}

// The user directories are where user-dirs.dirs puts them, under their
// localized names; one it points at home is left out; the GTK
// bookmarks follow, the local folders that exist.
func TestUserPlacesFollowUserDirsAndBookmarks(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, ".config")
	browsertest.Touch(t, home, "Dokumente/", "Bilder/", "Projects/", "Music/")
	browsertest.Touch(t, config, "user-dirs.dirs", "gtk-3.0/bookmarks")
	write := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(config, rel), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("user-dirs.dirs", "XDG_DOCUMENTS_DIR=\"$HOME/Dokumente\"\nXDG_PICTURES_DIR=\"$HOME/Bilder\"\nXDG_MUSIC_DIR=\"$HOME/\"\n")
	write("gtk-3.0/bookmarks", fileuri.FromPath(filepath.Join(home, "Projects"))+" Work\nsftp://host/x Remote\n"+fileuri.FromPath(filepath.Join(home, "gone"))+"\n")
	got := placeLabels(t, home)
	want := []string{"Home=" + home, "Dokumente=" + filepath.Join(home, "Dokumente"), "Bilder=" + filepath.Join(home, "Bilder"), "Work=" + filepath.Join(home, "Projects")}
	if !slices.Equal(got, want) {
		t.Errorf("places = %v, want %v", got, want)
	}
}

// Recent lists the used local files that still exist, newest first,
// and a missing list none.
func TestListRecent(t *testing.T) {
	dir := t.TempDir()
	browsertest.Touch(t, dir, "old.txt", "new.txt", "sub/")
	xbelPath := filepath.Join(dir, "recently-used.xbel")
	body := `<?xml version="1.0"?><xbel version="1.0">
<bookmark href="` + fileuri.FromPath(filepath.Join(dir, "old.txt")) + `" modified="2026-01-01T00:00:00Z"/>
<bookmark href="` + fileuri.FromPath(filepath.Join(dir, "new.txt")) + `" modified="2026-10-01T00:00:00Z"/>
<bookmark href="` + fileuri.FromPath(filepath.Join(dir, "gone.txt")) + `" modified="2026-10-02T00:00:00Z"/>
<bookmark href="` + fileuri.FromPath(filepath.Join(dir, "sub")) + `" modified="2026-10-03T00:00:00Z"/>
<bookmark href="https://example.org/x" modified="2026-10-04T00:00:00Z"/>
</xbel>`
	if err := os.WriteFile(xbelPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := ListRecent(xbelPath, FilesAndFolders)
	if got := names(entries); err != nil || !slices.Equal(got, []string{"new.txt", "old.txt"}) {
		t.Errorf("recent = %v, %v", got, err)
	}
	if entries, err := ListRecent(filepath.Join(dir, "none.xbel"), FilesAndFolders); err != nil || len(entries) != 0 {
		t.Errorf("a missing list = %v, %v", entries, err)
	}
	if err := os.WriteFile(xbelPath, []byte("<xbel"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ListRecent(xbelPath, FilesAndFolders); err == nil {
		t.Error("a broken list read without an error")
	}
	if crumbs := Crumbs(recentLocation, "/home/u"); len(crumbs) != 1 || crumbs[0].Label != "Recent" {
		t.Errorf("Recent crumbs = %v", crumbs)
	}
}

// The Kind column orders folders, which have one kind, by name, and a
// file without an extension by the label it shows.
func TestKindSortOrdersFoldersByNameAndUnnamedFilesByTheirLabel(t *testing.T) {
	entries := []Entry{entry("z.d", true, 0), entry("a.x", true, 0), entry("b.zip", false, 0), entry("Makefile", false, 0), entry("a.bin", false, 0)}
	SortEntries(entries, SortKind, true)
	want := []string{"a.x", "z.d", "a.bin", "Makefile", "b.zip"}
	if got := names(entries); !slices.Equal(got, want) {
		t.Errorf("kind asc = %v, want %v (a file without an extension sorts as %q)", got, want, i18n.T("file-chooser-kind-file"))
	}
}
