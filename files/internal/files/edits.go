package files

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/stubbedev/lys/desktop/fileops"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/localcopy"
	"github.com/stubbedev/lys/ui/app"

	"github.com/stubbedev/lys/files/internal/locale"
)

// edit is a copy of a file on a network location opened in an app:
// followed while the file manager runs, so each version saved to it
// goes back in the file's place. asking is set while a conflict's
// question shows.
type edit struct {
	loc    string
	f      *localcopy.Follower
	asking bool
}

// statTimeout bounds stating a file on a network location before it is
// fetched.
const statTimeout = 30 * time.Second

// openFile opens path in its default app: a file on a network location
// as a followed copy (openRemote).
func (f *Files) openFile(path string) {
	if vfs.IsRemote(path) {
		f.openRemote(path, f.cfg.Host.OpenFile)
		return
	}
	f.cfg.Host.OpenFile(path)
}

// openRemote copies the file at the network location loc into a folder
// of its own under the fetch folder, as a copy that shows its progress,
// opens the copy with open, and follows it: each version an app saves
// goes back in the file's place, unless the file changed there since it
// was fetched, which asks whether to replace it.
func (f *Files) openRemote(loc string, open func(local string)) {
	failed := func(err error) {
		log.Printf("files: open %s: %v", loc, err)
		f.cfg.Host.Toast(locale.T().Get("files-open-failed", i18n.Str("name", vfs.Base(loc))), nil)
	}
	if f.cfg.FetchDir == "" {
		failed(errors.New("no fetch folder"))
		return
	}
	type fetching struct {
		dir   string
		stamp localcopy.Stamp
		err   error
	}
	offLoop(f, func() fetching {
		stamp, err := f.stampRemote(loc)
		if err != nil {
			return fetching{err: err}
		}
		if err := os.MkdirAll(f.cfg.FetchDir, 0o700); err != nil {
			return fetching{err: err}
		}
		dir, err := os.MkdirTemp(f.cfg.FetchDir, "open-")
		return fetching{dir: dir, stamp: stamp, err: err}
	}, func(got fetching) {
		if got.err != nil {
			failed(got.err)
			return
		}
		local := filepath.Join(got.dir, vfs.Base(loc))
		f.jobs.start(copying, fileops.Copy{Sources: []string{loc}, Dest: got.dir, Mounts: f.mounts()}, func(*fileops.Undo) {
			if err := f.follow(loc, local, got.stamp); err != nil {
				failed(err)
				return
			}
			open(local)
		})
	})
}

// stampRemote is the stamp of the file at loc now; safe off the loop.
func (f *Files) stampRemote(loc string) (localcopy.Stamp, error) {
	fsys, p, err := f.mounts().Resolve(loc)
	if err != nil {
		return localcopy.Stamp{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), statTimeout)
	defer cancel()
	info, err := fsys.Stat(ctx, p)
	if err != nil {
		return localcopy.Stamp{}, err
	}
	return localcopy.StampOf(info), nil
}

// follow follows local, the copy of loc fetched as stamp stamps the
// file, until the file manager closes.
func (f *Files) follow(loc, local string, stamp localcopy.Stamp) error {
	e := &edit{loc: loc}
	reach := func() (vfs.FS, string, error) { return f.mounts().Resolve(loc) }
	fo, err := localcopy.Follow(local, reach, localcopy.Unchanged(stamp), func(r localcopy.Report) {
		f.cfg.Host.Invoke(func() { f.reported(e, r) })
	})
	if err != nil {
		return err
	}
	e.f = fo
	f.edits = append(f.edits, e)
	return nil
}

// reported says what became of a version of e saved: nothing once it
// went back, a toast when it could not, and a question when the file
// changed on the server since.
func (f *Files) reported(e *edit, r localcopy.Report) {
	tr := locale.T()
	args := []i18n.Arg{i18n.Str("name", vfs.Base(e.loc)), i18n.Str("server", vfs.Root(e.loc))}
	switch r.Outcome {
	case localcopy.Uploaded:
		f.cfg.Host.Toast(tr.Get("files-sent-back", args...), nil)
	case localcopy.Failed:
		log.Printf("files: send %s back: %v", e.loc, r.Err)
		f.cfg.Host.Toast(tr.Get("files-send-back-failed", args...), nil)
	case localcopy.Conflicted:
		if e.asking {
			return
		}
		e.asking = true
		f.cfg.Host.Ask(Question{
			Title: tr.Get("files-conflict-remote-title", args...),
			Body:  tr.Get("files-conflict-remote-body", args...),
			Buttons: []app.DialogButton{
				{Label: tr.Get("files-keep-server"), Response: answerCancel, Role: app.ButtonRoleCancel},
				{Label: tr.Get("files-replace"), Response: answerReplace, Role: app.ButtonRoleDefault, Appearance: app.AppearanceDestructive},
			},
			Default: answerCancel, Cancel: answerCancel,
			Answer: func(response string) {
				e.asking = false
				if response == answerReplace {
					e.f.Overwrite()
				}
			},
		})
	}
}

// closeEdits stops following the copies opened, each once an upload
// under way ended.
func (f *Files) closeEdits() {
	for _, e := range f.edits {
		if err := e.f.Close(); err != nil {
			log.Printf("files: stop following %s: %v", e.loc, err)
		}
	}
	f.edits = nil
}
