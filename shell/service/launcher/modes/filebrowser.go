package modes

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/shlex"
	"github.com/stubbedev/lys/lib/xdg"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/shell/service/fileindex"
	"github.com/stubbedev/lys/shell/service/launcher"
)

// maxEntries caps the collected entries: a recursive walk of a home
// directory is unbounded otherwise.
const maxEntries = 50_000

// FileSort is the file browser's ordering.
type FileSort uint8

// File orderings; the time orders are newest first.
const (
	FileSortName FileSort = iota
	FileSortMtime
	FileSortAtime
	FileSortCtime
)

// FileBrowserConfig is the file browser's knobs.
type FileBrowserConfig struct {
	// Directory is the start directory; empty is $HOME.
	Directory        string
	Sorting          FileSort
	DirectoriesFirst bool
	ShowHidden       bool
	// Command opens a picked file; empty is the built-in opener.
	Command string
	// Terminal runs a Terminal=true opener; empty autodetects.
	Terminal string
	// Recursive lists every file below (recursivebrowser).
	Recursive bool
}

// DefaultFileBrowserConfig is the file browser without any flag or config.
func DefaultFileBrowserConfig() FileBrowserConfig {
	return FileBrowserConfig{DirectoriesFirst: true}
}

type fileEntryKind uint8

const (
	entryParent fileEntryKind = iota
	entryDir
	entryFile
)

type fileEntry struct {
	kind fileEntryKind
	path string
}

// FileBrowser is filebrowser and recursivebrowser.
type FileBrowser struct {
	cfg     FileBrowserConfig
	current string
	entries []fileEntry
}

// NewFileBrowser roots the browser at the configured directory.
func NewFileBrowser(cfg FileBrowserConfig) *FileBrowser {
	current := os.Getenv("HOME")
	if cfg.Directory != "" {
		current = xdg.ExpandHome(cfg.Directory, os.Getenv("HOME"))
	}
	return &FileBrowser{cfg: cfg, current: current}
}

// Name is filebrowser or recursivebrowser.
func (f *FileBrowser) Name() string {
	if f.cfg.Recursive {
		return "recursivebrowser"
	}
	return "filebrowser"
}

type listed struct {
	path  string
	isDir bool
}

func (f *FileBrowser) state(ctx context.Context) launcher.ModeState {
	var entries []listed
	if f.cfg.Recursive {
		entries = walkFiles(ctx, f.current, f.cfg.ShowHidden)
	} else {
		entries = listDir(f.current, f.cfg.ShowHidden)
	}
	sortEntries(entries, f.cfg.Sorting, f.cfg.DirectoriesFirst)
	f.entries = f.entries[:0]
	items := make([]launcher.Item, 0, len(entries)+1)
	if parent := filepath.Dir(f.current); !f.cfg.Recursive && parent != f.current {
		f.entries = append(f.entries, fileEntry{kind: entryParent, path: parent})
		items = append(items, launcher.Item{Display: "..", MatchText: "..", Icon: launcher.IconName("ld-folder-symbolic")})
	}
	for _, e := range entries {
		items = append(items, entryItem(e, f.current, f.cfg.Recursive))
		kind := entryFile
		if e.isDir {
			kind = entryDir
		}
		f.entries = append(f.entries, fileEntry{kind: kind, path: e.path})
	}
	return launcher.ModeState{Items: items, Prompt: xdg.CollapseHome(f.current, os.Getenv("HOME"))}
}

func (f *FileBrowser) open(ctx context.Context, path string) launcher.Action {
	if err := openFile(path, f.cfg.Command, f.cfg.Terminal); err != nil {
		return failed(f.state(ctx), RanFailed(filepath.Base(path), err), err)
	}
	return launcher.ActionClose{}
}

// openFile opens path with command, or in its default app when command
// is blank.
func openFile(path, command, terminal string) error {
	if strings.TrimSpace(command) != "" {
		launcher.RunShell(command + " " + shlex.MustQuote(path))
		return nil
	}
	return launcher.Open(path, terminal, "")
}

// Load lists the current directory.
func (f *FileBrowser) Load(ctx context.Context) launcher.ModeState { return f.state(ctx) }

// Activate enters a directory, opens a file, or follows a typed path.
func (f *FileBrowser) Activate(ctx context.Context, target launcher.Target, kind launcher.ActivateKind, _ string) launcher.Action {
	if i, ok := target.Row(); ok && int(i) < len(f.entries) {
		e := f.entries[i]
		if e.kind == entryFile {
			return f.open(ctx, e.path)
		}
		f.current = e.path
		return launcher.ActionReload{State: f.state(ctx)}
	}
	custom, ok := kind.(launcher.ActivateCustom)
	if !ok {
		return launcher.ActionNothing{}
	}
	path := xdg.ExpandHome(strings.TrimSpace(custom.Text), os.Getenv("HOME"))
	st, err := os.Stat(path) //nolint:gosec // the path the user typed into the browser
	switch {
	case err != nil:
		return launcher.ActionNothing{}
	case st.IsDir():
		f.current = path
		return launcher.ActionReload{State: f.state(ctx)}
	case st.Mode().IsRegular():
		return f.open(ctx, path)
	}
	return launcher.ActionNothing{}
}

func isHidden(path string) bool { return strings.HasPrefix(filepath.Base(path), ".") }

func listDir(dir string, showHidden bool) []listed {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []listed
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if !showHidden && isHidden(path) {
			continue
		}
		if len(out) >= maxEntries {
			break
		}
		st, err := os.Stat(path)
		out = append(out, listed{path: path, isDir: err == nil && st.IsDir()})
	}
	return out
}

// walkFiles lists the files under root as the file index walks
// (fileindex.Walk): breadth first, no symlinks, no dependency or build
// trees, at most fileindex.MaxDepth deep and maxEntries entries in all,
// hidden names only with showHidden.
func walkFiles(ctx context.Context, root string, showHidden bool) []listed {
	var out []listed
	end := fileindex.Walk(ctx, []string{root}, fileindex.Rules{ShowHidden: showHidden}, maxEntries, func(d fileindex.Dir) {
		for _, c := range d.Children {
			if !c.Dir {
				out = append(out, listed{path: filepath.Join(d.Path, c.Name)})
			}
		}
	})
	if end == fileindex.Full {
		log.Printf("launcher: recursive browser hit the %d entry cap; listing truncated", maxEntries)
	}
	return out
}

// fileTime is the sort key for the time orders; unreadable is the
// epoch. Creation time comes from statx's birth time.
func fileTime(path string, sorting FileSort) time.Time {
	switch sorting {
	case FileSortMtime:
		if st, err := os.Stat(path); err == nil {
			return st.ModTime()
		}
	case FileSortAtime, FileSortCtime:
		var sx unix.Statx_t
		if err := unix.Statx(unix.AT_FDCWD, path, 0, unix.STATX_ATIME|unix.STATX_BTIME, &sx); err == nil {
			ts := sx.Atime
			if sorting == FileSortCtime {
				if sx.Mask&unix.STATX_BTIME == 0 {
					return time.Unix(0, 0)
				}
				ts = sx.Btime
			}
			return time.Unix(ts.Sec, int64(ts.Nsec))
		}
	}
	return time.Unix(0, 0)
}

func sortEntries(entries []listed, sorting FileSort, dirsFirst bool) {
	slices.SortStableFunc(entries, func(a, b listed) int {
		if dirsFirst && a.isDir != b.isDir {
			if a.isDir {
				return -1
			}
			return 1
		}
		if sorting == FileSortName {
			return strings.Compare(asciiLower(filepath.Base(a.path)), asciiLower(filepath.Base(b.path)))
		}
		return fileTime(b.path, sorting).Compare(fileTime(a.path, sorting))
	})
}

// asciiLower folds ASCII case only.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// entryItem is a row: a folder icon for a directory, a thumbnail
// request (generated lazily, only for the rows drawn) for a file.
func entryItem(e listed, base string, recursive bool) launcher.Item {
	name := filepath.Base(e.path)
	if recursive {
		if rel, err := filepath.Rel(base, e.path); err == nil {
			name = rel
		}
	}
	var icon launcher.Icon = launcher.IconName("ld-folder-symbolic")
	if !e.isDir {
		icon = launcher.IconThumbnail{Path: e.path, Fallback: launcher.MimeIcon(e.path)}
	}
	return launcher.Item{Display: name, MatchText: name, Icon: icon}
}
