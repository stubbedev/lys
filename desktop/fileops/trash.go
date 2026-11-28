package fileops

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/lib/fileuri"
	"github.com/stubbedev/lys/lib/xdg"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/desktop/vfs"
)

// deletionDate is the trash spec's DeletionDate layout, in local time.
const deletionDate = "2006-01-02T15:04:05"

const (
	infoSuffix   = ".trashinfo"
	nameAttempts = 10000
)

// Volume is the filesystem a path lies on: its device and the
// directory it is mounted on, the spec's $topdir.
type Volume struct {
	Dev uint64
	Top string
}

// Bin trashes files as the freedesktop trash spec says: into the home
// trash under DataHome for a file on its filesystem, else into the
// file's own volume's trash. Volume finds a path's volume and Now the
// deletion time.
type Bin struct {
	DataHome string
	UID      int
	Volume   func(path string) (Volume, error)
	Now      func() time.Time
}

// SystemBin is the user's bin: the XDG data home, the process's user,
// the volumes the filesystem reports and the clock.
func SystemBin() (Bin, error) {
	data, ok := xdg.OS.DataHome()
	if !ok {
		return Bin{}, errors.New("find the trash: there is no XDG data home")
	}
	return Bin{DataHome: data, UID: os.Getuid(), Volume: SystemVolume, Now: time.Now}, nil
}

// SystemVolume is path's volume: its device, and the highest directory
// above it still on that device.
func SystemVolume(path string) (Volume, error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return Volume{}, &os.PathError{Op: "stat", Path: path, Err: err}
	}
	dev, top := st.Dev, filepath.Clean(path)
	for {
		parent := filepath.Dir(top)
		if parent == top {
			return Volume{Dev: dev, Top: top}, nil
		}
		if err := unix.Stat(parent, &st); err != nil {
			return Volume{}, &os.PathError{Op: "stat", Path: parent, Err: err}
		}
		if st.Dev != dev {
			return Volume{Dev: dev, Top: top}, nil
		}
		top = parent
	}
}

// Item is a file in a trash: its trash directory, its name in that
// trash's files/, where it was and when it was deleted (zero when its
// info records no date).
type Item struct {
	Dir      string
	Name     string
	Original string
	Deleted  time.Time
}

// Path is where the trashed file is now.
func (it Item) Path() string { return filepath.Join(it.Dir, "files", it.Name) }

func (it Item) infoPath() string { return filepath.Join(it.Dir, "info", it.Name+infoSuffix) }

// Dirs are the trash directories of the freedesktop trash spec: the
// home trash under dataHome, then for each mount point its
// $topdir/.Trash/$uid when .Trash is a real, sticky directory, and its
// $topdir/.Trash-$uid.
func Dirs(dataHome string, mounts []string, uid int) []string {
	var dirs []string
	if dataHome != "" {
		dirs = append(dirs, filepath.Join(dataHome, "Trash"))
	}
	for _, top := range mounts {
		dirs = append(dirs, volumeTrashes(top, sharedTrashUsable(top), uid)...)
	}
	return dirs
}

// sharedTrashUsable reports whether top's .Trash may hold users'
// trashes: a directory, not a link, with the sticky bit.
func sharedTrashUsable(top string) bool {
	info, err := os.Lstat(filepath.Join(top, ".Trash"))
	return err == nil && info.IsDir() && info.Mode()&os.ModeSticky != 0
}

// volumeTrashes are the user's trash directories on the volume at top,
// in the order the spec tries them.
func volumeTrashes(top string, shared bool, uid int) []string {
	id := strconv.Itoa(uid)
	own := filepath.Join(top, ".Trash-"+id)
	if shared {
		return []string{filepath.Join(top, ".Trash", id), own}
	}
	return []string{own}
}

// List lists the files in each trash directory's files/ that have
// readable, well-formed info: a file without is the spec's orphan and
// left out. A trash directory that does not exist lists nothing, and
// one that cannot be read is an error naming it.
func List(dirs []string) ([]Item, error) {
	var items []Item
	for _, dir := range dirs {
		files := filepath.Join(dir, "files")
		des, err := os.ReadDir(files)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read the trash %s: %w", files, err)
		}
		for _, de := range des {
			it := Item{Dir: dir, Name: de.Name()}
			info, err := os.ReadFile(it.infoPath())
			if err != nil {
				continue
			}
			p, deleted, ok := parseInfo(info, time.Local)
			if !ok {
				continue
			}
			if it.Original, ok = original(dir, p); ok {
				it.Deleted = deleted
				items = append(items, it)
			}
		}
	}
	return items, nil
}

// parseInfo reads a .trashinfo: the percent-decoded Path and the
// DeletionDate of its [Trash Info] group, which must come first; false
// for info that is not that group, has no Path, or has an undecodable
// Path or an unreadable date.
func parseInfo(info []byte, loc *time.Location) (string, time.Time, bool) {
	sc := bufio.NewScanner(bytes.NewReader(info))
	group, p, found := "", "", false
	var deleted time.Time
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if group == "" && line != "[Trash Info]" {
				return "", time.Time{}, false
			}
			group = line
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if group != "[Trash Info]" || !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "Path":
			decoded, err := url.PathUnescape(value)
			if err != nil {
				return "", time.Time{}, false
			}
			p, found = decoded, true
		case "DeletionDate":
			t, err := time.ParseInLocation(deletionDate, value, loc)
			if err != nil {
				return "", time.Time{}, false
			}
			deleted = t
		}
	}
	return p, deleted, found
}

// formatInfo is the .trashinfo recording a file trashed from p at t.
func formatInfo(p string, t time.Time) []byte {
	return []byte("[Trash Info]\nPath=" + fileuri.Encode(p, "/") + "\nDeletionDate=" + t.Format(deletionDate) + "\n")
}

// trashBase is the directory a trash's relative paths start from: the
// volume's top for $topdir/.Trash/$uid, else the trash's parent (the
// data home, or the top of $topdir/.Trash-$uid).
func trashBase(dir string) string {
	parent := filepath.Dir(dir)
	if filepath.Base(parent) == ".Trash" {
		return filepath.Dir(parent)
	}
	return parent
}

// original is the absolute path an info's Path names in the trash
// dir: false for a relative path climbing out with "..", or one that
// names no file.
func original(dir, p string) (string, bool) {
	if !filepath.IsAbs(p) {
		if slices.Contains(strings.Split(p, "/"), "..") {
			return "", false
		}
		p = filepath.Join(trashBase(dir), p)
	}
	p = filepath.Clean(p)
	if p == "/" {
		return "", false
	}
	return p, true
}

// trashCandidate is a trash directory a file may go to, and the top
// its info's Path is relative to ("" to record it absolute).
type trashCandidate struct{ dir, top string }

// trashCandidates are where the spec puts a file on volume file: the
// home trash for one on the home trash's volume, else the volume's own
// trashes in order.
func trashCandidates(file, home Volume, homeTrash string, shared bool, uid int) []trashCandidate {
	if file.Dev == home.Dev {
		return []trashCandidate{{dir: homeTrash}}
	}
	dirs := volumeTrashes(file.Top, shared, uid)
	cs := make([]trashCandidate, 0, len(dirs))
	for _, d := range dirs {
		cs = append(cs, trashCandidate{dir: d, top: file.Top})
	}
	return cs
}

// Trash moves Paths to the trash, each into the trash of its volume.
type Trash struct {
	Bin   Bin
	Paths []string
}

// ErrNoTrash is a network location trashed: it has no trash, and is
// deleted for good instead, once the user agreed.
var ErrNoTrash = fmt.Errorf("a network location has no trash: %w", vfs.ErrUnsupported)

func (t Trash) run(r *runner) (*Undo, error) {
	for _, path := range t.Paths {
		if vfs.IsRemote(path) {
			return nil, fmt.Errorf("trash %s: %w", path, ErrNoTrash)
		}
	}
	r.begin(int64(len(t.Paths)), 0)
	var trashed []Item
	for _, path := range t.Paths {
		if err := r.ctx.Err(); err != nil {
			return undoTrash(trashed), fmt.Errorf("trash %s: %w", path, err)
		}
		it, err := t.Bin.trash(path)
		if err != nil {
			return undoTrash(trashed), err
		}
		trashed = append(trashed, it)
		r.advance(path, 1, 0)
	}
	return undoTrash(trashed), nil
}

func undoTrash(items []Item) *Undo {
	if len(items) == 0 {
		return nil
	}
	return &Undo{trashed: items}
}

func (b Bin) trash(path string) (Item, error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return Item{}, fmt.Errorf("trash %s: not an absolute path", path)
	}
	if _, err := os.Lstat(path); err != nil {
		return Item{}, fmt.Errorf("trash %s: %w", path, err)
	}
	file, err := b.Volume(path)
	if err != nil {
		return Item{}, fmt.Errorf("trash %s: %w", path, err)
	}
	homeTrash := filepath.Join(b.DataHome, "Trash")
	if err := makeTrash(homeTrash, false, b.UID); err != nil {
		return Item{}, fmt.Errorf("trash %s: %w", path, err)
	}
	home, err := b.Volume(homeTrash)
	if err != nil {
		return Item{}, fmt.Errorf("trash %s: %w", path, err)
	}
	shared := file.Dev != home.Dev && sharedTrashUsable(file.Top)
	var errs []error
	for _, c := range trashCandidates(file, home, homeTrash, shared, b.UID) {
		if err := makeTrash(c.dir, c.top != "", b.UID); err != nil {
			errs = append(errs, err)
			continue
		}
		it, err := b.put(path, c)
		if err != nil {
			return Item{}, fmt.Errorf("trash %s into %s: %w", path, c.dir, err)
		}
		return it, nil
	}
	return Item{}, fmt.Errorf("trash %s: no trash on its volume can be used: %w", path, errors.Join(errs...))
}

// makeTrash makes the trash directory dir with its files/ and info/:
// a volume's must be a real directory of the user's alone.
func makeTrash(dir string, onVolume bool, uid int) error {
	if !onVolume {
		return errors.Join(os.MkdirAll(filepath.Join(dir, "files"), 0o700), os.MkdirAll(filepath.Join(dir, "info"), 0o700))
	}
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("create the trash %s: %w", dir, err)
	}
	var st unix.Stat_t
	if err := unix.Lstat(dir, &st); err != nil {
		return &os.PathError{Op: "stat", Path: dir, Err: err}
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR || int64(st.Uid) != int64(uid) {
		return fmt.Errorf("the trash %s is not a directory of user %d", dir, uid)
	}
	for _, sub := range []string{"files", "info"} {
		if err := os.Mkdir(filepath.Join(dir, sub), 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("create the trash %s: %w", dir, err)
		}
	}
	return nil
}

// put moves path into the trash c under the first free name: its info
// is claimed first, never over another's, then the file renamed in.
func (b Bin) put(path string, c trashCandidate) (_ Item, err error) {
	recorded := path
	if c.top != "" {
		if recorded, err = filepath.Rel(c.top, path); err != nil {
			return Item{}, err
		}
	}
	now := b.Now().Round(0).Truncate(time.Second)
	tmp, err := os.CreateTemp(filepath.Join(c.dir, "info"), ".trashinfo-")
	if err != nil {
		return Item{}, err
	}
	claimed := false
	defer func() {
		if !claimed {
			err = errors.Join(err, os.Remove(tmp.Name()))
		}
	}()
	if _, err := tmp.Write(formatInfo(recorded, now)); err != nil {
		return Item{}, errors.Join(err, tmp.Close())
	}
	if err := errors.Join(tmp.Sync(), tmp.Close()); err != nil {
		return Item{}, err
	}
	base := filepath.Base(path)
	for i := 1; i <= nameAttempts; i++ {
		name := base
		if i > 1 {
			name = base + "." + strconv.Itoa(i)
		}
		it := Item{Dir: c.dir, Name: name, Original: path, Deleted: now}
		if err := renameNoReplace(tmp.Name(), it.infoPath()); errors.Is(err, fs.ErrExist) {
			continue
		} else if err != nil {
			return Item{}, err
		}
		claimed = true
		err := renameNoReplace(path, it.Path())
		if err == nil {
			return it, nil
		}
		if rbErr := os.Rename(it.infoPath(), tmp.Name()); rbErr != nil {
			return Item{}, errors.Join(err, rbErr)
		}
		claimed = false
		if !errors.Is(err, fs.ErrExist) {
			return Item{}, err
		}
	}
	return Item{}, fmt.Errorf("every one of %d names for %s is taken", nameAttempts, base)
}

// Restore moves a trashed file back where it was, making the folders
// it was in again when they are gone, never over something now there,
// and drops its info.
func Restore(it Item) error {
	if err := os.MkdirAll(filepath.Dir(it.Original), 0o777); err != nil { //nolint:gosec // the user's folders made again, as the umask allows
		return fmt.Errorf("restore %s to %s: %w", it.Path(), it.Original, err)
	}
	if err := renameNoReplace(it.Path(), it.Original); err != nil {
		return fmt.Errorf("restore %s to %s: %w", it.Path(), it.Original, err)
	}
	if err := os.Remove(it.infoPath()); err != nil {
		return fmt.Errorf("restore %s: drop its trash info: %w", it.Original, err)
	}
	return nil
}

// EmptyTrash removes everything in the trash directories Dirs for
// good: the trashed files, their info and the size cache.
type EmptyTrash struct {
	Dirs []string
}

func (e EmptyTrash) run(r *runner) (*Undo, error) {
	var paths []string
	for _, dir := range e.Dirs {
		for _, sub := range []string{"files", "info"} {
			des, err := os.ReadDir(filepath.Join(dir, sub))
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("empty the trash %s: %w", dir, err)
			}
			for _, de := range des {
				paths = append(paths, filepath.Join(dir, sub, de.Name()))
			}
		}
	}
	r.begin(int64(len(paths)), 0)
	for _, p := range paths {
		if err := r.ctx.Err(); err != nil {
			return nil, fmt.Errorf("empty the trash: %w", err)
		}
		if err := os.RemoveAll(p); err != nil {
			return nil, fmt.Errorf("empty the trash: %w", err)
		}
		r.advance(p, 1, 0)
	}
	for _, dir := range e.Dirs {
		if err := os.Remove(filepath.Join(dir, "directorysizes")); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("empty the trash %s: %w", dir, err)
		}
	}
	return nil, nil
}
