package search

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/xdg"

	"github.com/stubbedev/lys/shell/service/fileindex"
	"github.com/stubbedev/lys/shell/service/launcher"
)

const (
	// recentRank lifts a recent file over one merely indexed.
	recentRank = 24
	// pathRowsMax bounds a directory listing answered for a typed path.
	pathRowsMax = 30
)

// FileIndex is the index the files source reads (*fileindex.Index).
type FileIndex interface {
	Snapshot() *fileindex.Snapshot
	Changes() (ticks <-chan struct{}, stop func())
}

// FilesConfig is what Files reads and how it shows it.
type FilesConfig struct {
	Index FileIndex
	// Home abbreviates paths and resolves a leading ~.
	Home string
	// Previews asks for the side pane.
	Previews bool
}

// Files searches the recent files and the shell's file index, and
// answers a typed path with its directory's contents.
type Files struct {
	env Env
	cfg FilesConfig
}

// NewFiles builds the provider.
func NewFiles(env Env, cfg FilesConfig) *Files { return &Files{env: env, cfg: cfg} }

// Spec implements Provider.
func (*Files) Spec() Spec {
	return Spec{Section: SectionFiles, MinQuery: 2, Weight: -8}
}

// Load implements Provider.
func (f *Files) Load(ctx context.Context, publish func([]Entry)) { f.LoadStaged(ctx, publish) }

// LoadStaged implements Staged: the index as it stands, and once more
// when its first walk ends if it had not. Only the session's end before
// that cuts it short.
func (f *Files) LoadStaged(session context.Context, publish func([]Entry)) bool {
	ticks, stop := f.cfg.Index.Changes()
	defer stop()
	snap := f.cfg.Index.Snapshot()
	publish(f.entries(snap))
	for !snap.Ready {
		select {
		case <-session.Done():
			return false
		case _, open := <-ticks:
			if !open {
				return false
			}
		}
		snap = f.cfg.Index.Snapshot()
		publish(f.entries(snap))
	}
	return true
}

// entries are the rows: recents first and only once. A row's path and
// its folder as shown live in one block the rows share, its title in
// the index's own name, so fifty thousand rows are a few allocations
// rather than a few per row.
func (f *Files) entries(snap *fileindex.Snapshot) []Entry {
	total := len(snap.Recent) + len(snap.Files)
	seen := make(map[string]struct{}, len(snap.Recent))
	out := make([]Entry, 0, total)
	data := make([]fileData, total)
	size := 0
	for _, p := range snap.Recent {
		size += len(p)
	}
	for _, file := range snap.Files {
		size += len(file.Folder) + 1 + len(file.Name)
	}
	var block strings.Builder
	block.Grow(size)
	icons := map[string]launcher.Icon{}
	add := func(path, title, shown string, dir bool, rank int) {
		d := &data[len(out)]
		*d = fileData{path: path, dir: dir}
		out = append(out, f.entry(d, title, shown, icons, rank))
	}
	for _, p := range snap.Recent {
		seen[p] = struct{}{}
		add(p, p[strings.LastIndexByte(p, '/')+1:], f.displayDir(folderOf(p), &block), false, recentRank)
	}
	folder, shown := "", ""
	for _, file := range snap.Files {
		if file.Folder != folder {
			folder, shown = file.Folder, f.displayDir(file.Folder, &block)
		}
		at := block.Len()
		block.WriteString(folder)
		if !strings.HasSuffix(folder, "/") {
			block.WriteByte('/')
		}
		block.WriteString(file.Name)
		path := block.String()[at:]
		if _, dup := seen[path]; dup {
			continue
		}
		add(path, file.Name, shown, file.Dir, 0)
	}
	return out
}

// folderOf is the folder path is in.
func folderOf(path string) string { return path[:max(strings.LastIndexByte(path, '/'), 1)] }

// displayDir is dir as shown: under the home directory, ~/. Outside it
// dir's own bytes serve, and within it the shortened form goes in the
// shared block.
func (f *Files) displayDir(dir string, block *strings.Builder) string {
	rest, ok := xdg.CutHome(dir, f.cfg.Home)
	if !ok {
		return dir
	}
	start := block.Len()
	block.WriteByte('~')
	block.WriteString(rest)
	return block.String()[start:]
}

func (f *Files) entry(d *fileData, title, shown string, icons map[string]launcher.Icon, rank int) Entry {
	path := d.path
	e := Entry{ID: path, Learn: path, Title: title, Subtitle: shown, Data: d, Rank: rank}
	if d.dir {
		e.Icon = iconFolder
		e.Completion = strings.TrimSuffix(e.Subtitle, "/") + "/" + e.Title + "/"
		return e
	}
	ext := e.Title[max(strings.LastIndexByte(e.Title, '.'), 0):]
	icon, ok := icons[ext]
	if !ok {
		icon = launcher.IconName(launcher.MimeIcon(path))
		if icons != nil && strings.HasPrefix(ext, ".") {
			icons[ext] = icon
		}
	}
	e.Icon = icon
	if f.cfg.Previews {
		e.Preview = launcher.Preview{Path: path}
	}
	return e
}

type fileData struct {
	path string
	dir  bool
}

// Answer implements Answerer: a query that is a path lists its directory
// filtered by the name being typed. It is one directory read, on the
// engine goroutine.
func (f *Files) Answer(query string) []Entry {
	q := strings.TrimSpace(query)
	if !strings.HasPrefix(q, "/") && q != "~" && !strings.HasPrefix(q, "~/") {
		return nil
	}
	full := xdg.ExpandHome(q, f.cfg.Home)
	dir, prefix := full, ""
	if !strings.HasSuffix(full, "/") {
		dir, prefix = filepath.Dir(full), filepath.Base(full)
	}
	if q == "~" {
		dir, prefix = f.cfg.Home, ""
	}
	listing, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	folded := strings.ToLower(prefix)
	var block strings.Builder
	shown := f.displayDir(filepath.Clean(dir), &block)
	out := make([]Entry, 0, min(len(listing), pathRowsMax))
	for _, d := range listing {
		name := d.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(name), folded) {
			continue
		}
		data := &fileData{path: filepath.Join(dir, name), dir: d.IsDir()}
		out = append(out, f.entry(data, name, shown, nil, 0))
		if len(out) == pathRowsMax {
			break
		}
	}
	return out
}

// Actions implements Provider.
func (f *Files) Actions(e Entry) []Action {
	d, ok := e.Data.(*fileData)
	if !ok {
		return nil
	}
	parent := filepath.Dir(d.path)
	open := Action{Title: i18n.T("launcher-action-open"), Icon: "ld-external-link-symbolic", Run: func(context.Context) Outcome {
		return f.env.open(i18n.T("launcher-action-open"), d.path)
	}}
	reveal := Action{Title: i18n.T("launcher-action-reveal"), Icon: "ld-folder-open-symbolic", Run: func(context.Context) Outcome {
		return f.env.open(i18n.T("launcher-action-reveal"), parent)
	}}
	terminalAt := parent
	if d.dir {
		terminalAt = d.path
	}
	terminal := Action{Title: i18n.T("launcher-action-open-terminal-here"), Icon: "ld-terminal-symbolic", Run: func(context.Context) Outcome {
		return f.env.inTerminal(i18n.T("launcher-action-open-terminal-here"), "sh", "-c", `cd "$1" && exec "${SHELL:-sh}"`, "sh", terminalAt)
	}}
	return []Action{
		open, reveal, terminal,
		copyAction(i18n.T("launcher-action-copy-path"), d.path),
		copyAction(i18n.T("launcher-action-copy-name"), filepath.Base(d.path)),
	}
}
