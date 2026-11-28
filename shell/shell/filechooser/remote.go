package filechooser

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/stubbedev/lys/desktop/fileuri"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/localcopy"

	"github.com/stubbedev/lys/shell/shell/filebrowser"
)

// The portal answers apps with local paths, which they may read, write
// and keep long after the dialog: a file picked on a network location is
// answered as a copy fetched into Copies, and a file saved to one as a
// path in Copies the app writes. Either is followed for the rest of the
// session, so each version the app saves goes back in the file's place,
// unless the file changed there meanwhile; what did not go is told
// (Tell), the version saved kept on the local disk.

// errNoCopies is a file on a network location chosen with no folder to
// fetch or save it in.
var errNoCopies = errors.New("no folder keeps copies of files on network locations")

// remoteTimeout bounds stating a location on the server before a fetch
// or a save.
const remoteTimeout = 30 * time.Second

// copyIn is a new folder of its own under Copies for a copy of the file
// at loc, and the copy's path there.
func (c *Chooser) copyIn(loc string) (string, error) {
	if c.d.Copies == "" {
		return "", errNoCopies
	}
	if err := os.MkdirAll(c.d.Copies, 0o700); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(c.d.Copies, "copy-")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, vfs.Base(loc)), nil
}

// fetched is a file on a network location fetched for an app: where
// it is, its copy and its stamp.
type fetched struct {
	loc, local string
	stamp      localcopy.Stamp
}

// openRemote answers an open with locs, the files on network locations
// among them fetched first, saying so while they are; one that cannot
// be fetched keeps the dialog, saying so. A folder there cannot be
// chosen: an app could not write into a copy of it.
func (c *Chooser) openRemote(locs []string) {
	if c.mode.folders() {
		c.b.Warn(i18n.T("file-chooser-remote-folder"))
		return
	}
	c.b.Warn(i18n.T("file-chooser-fetching", i18n.Str("name", vfs.Base(locs[0]))))
	serial, mounts := c.serial, c.d.Browser.Mounts
	type outcome struct {
		files []fetched
		uris  []string
		err   error
	}
	filebrowser.OffLoop(c.b, func() outcome {
		var out outcome
		for _, loc := range locs {
			if !vfs.IsRemote(loc) {
				out.uris = append(out.uris, fileuri.FromPath(loc))
				continue
			}
			f, err := c.fetch(mounts, loc)
			if err != nil {
				return outcome{err: err}
			}
			out.files = append(out.files, f)
			out.uris = append(out.uris, fileuri.FromPath(f.local))
		}
		return out
	}, func(out outcome) {
		switch {
		case c.serial != serial || c.reply == nil:
		case out.err != nil:
			log.Printf("file chooser: %v", out.err)
			c.b.Warn(i18n.T("file-chooser-fetch-failed", i18n.Str("name", vfs.Base(locs[0]))))
		default:
			for _, f := range out.files {
				c.follow(f.loc, f.local, localcopy.Unchanged(f.stamp))
			}
			c.answer(out.uris, false)
		}
	})
}

// fetch fetches the file at loc into a folder of its own under Copies;
// safe off the loop.
func (c *Chooser) fetch(mounts *vfs.Mounts, loc string) (fetched, error) {
	fsys, p, err := mounts.Resolve(loc)
	if err != nil {
		return fetched{}, fmt.Errorf("fetch %s: %w", loc, err)
	}
	local, err := c.copyIn(loc)
	if err != nil {
		return fetched{}, fmt.Errorf("fetch %s: %w", loc, err)
	}
	stamp, err := localcopy.Fetch(context.Background(), fsys, p, local)
	if err != nil {
		return fetched{}, err
	}
	return fetched{loc: loc, local: local, stamp: stamp}, nil
}

// remoteSave is what saving to a location on a network location found
// there.
type remoteSave struct {
	check saveCheck
	local string
	err   error
}

// saveRemote answers a save to the network location loc as Save does a
// local one, with a path in Copies the app writes, followed to go there:
// over the file there only once the user confirmed replacing it, else
// only while nothing took the name meanwhile.
func (c *Chooser) saveRemote(loc string) {
	serial, mounts := c.serial, c.d.Browser.Mounts
	filebrowser.OffLoop(c.b, func() remoteSave { return c.checkRemoteSave(mounts, loc) }, func(got remoteSave) {
		dir := vfs.Dir(loc)
		switch {
		case c.serial != serial || c.reply == nil:
		case got.err != nil:
			log.Printf("file chooser: save to %s: %v", loc, got.err)
			c.b.Warn(i18n.T("file-chooser-save-not-writable", i18n.Str("folder", dir)))
		case got.check.isDir:
			c.name.SetText("")
			c.b.GoTo(loc)
		case got.check.noFolder:
			c.b.Warn(i18n.T("file-chooser-save-no-folder", i18n.Str("folder", dir)))
		case got.check.exists && c.b.Asking() != loc:
			c.b.Ask(i18n.T("file-chooser-save-exists", i18n.Str("name", vfs.Base(loc)), i18n.Str("folder", vfs.Base(dir))), loc)
		default:
			want := localcopy.Absent()
			if got.check.exists {
				want = localcopy.Anything()
			}
			c.follow(loc, got.local, want)
			c.answer([]string{fileuri.FromPath(got.local)}, false)
		}
	})
}

// checkRemoteSave states loc and its folder on their server and makes
// the path the app is to write; safe off the loop.
func (c *Chooser) checkRemoteSave(mounts *vfs.Mounts, loc string) remoteSave {
	fsys, p, err := mounts.Resolve(loc)
	if err != nil {
		return remoteSave{err: err}
	}
	ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
	defer cancel()
	info, err := fsys.Stat(ctx, p)
	switch {
	case err == nil && info.IsDir:
		return remoteSave{check: saveCheck{isDir: true}}
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return remoteSave{err: err}
	}
	exists := err == nil
	if d, derr := fsys.Stat(ctx, vfs.Dir(p)); derr != nil || !d.IsDir {
		return remoteSave{check: saveCheck{noFolder: true}}
	}
	local, err := c.copyIn(loc)
	if err != nil {
		return remoteSave{err: err}
	}
	return remoteSave{check: saveCheck{exists: exists}, local: local}
}

// follow follows local, the copy handed to an app of the file at loc,
// which want expects there, for the rest of the session; it says when
// it cannot.
func (c *Chooser) follow(loc, local string, want localcopy.Expect) {
	mounts := c.d.Browser.Mounts
	reach := func() (vfs.FS, string, error) { return mounts.Resolve(loc) }
	f, err := localcopy.Follow(local, reach, want, func(r localcopy.Report) { c.told(loc, local, r) })
	if err != nil {
		log.Printf("file chooser: %v", err)
		c.tell(i18n.T("file-chooser-send-failed", i18n.Str("name", vfs.Base(loc)), i18n.Str("server", vfs.Root(loc))), local)
		return
	}
	c.follows = append(c.follows, f)
}

// told says what did not become of a version saved to local, the copy
// of loc: one not sent, kept on the local disk.
func (c *Chooser) told(loc, local string, r localcopy.Report) {
	name, server := i18n.Str("name", vfs.Base(loc)), i18n.Str("server", vfs.Root(loc))
	switch r.Outcome {
	case localcopy.Uploaded:
	case localcopy.Conflicted:
		log.Printf("file chooser: %v", r.Err)
		c.tell(i18n.T("file-chooser-changed-there", name, server), local)
	case localcopy.Failed:
		log.Printf("file chooser: %v", r.Err)
		c.tell(i18n.T("file-chooser-send-failed", name, server), local)
	}
}

// tell tells summary, saying the version saved is kept at local.
func (c *Chooser) tell(summary, local string) {
	if c.d.Tell != nil {
		c.d.Tell(summary, i18n.T("file-chooser-kept-here", i18n.Str("path", local)))
	}
}

// closeFollows stops following the copies handed to apps.
func (c *Chooser) closeFollows() {
	for _, f := range c.follows {
		if err := f.Close(); err != nil {
			log.Printf("file chooser: %v", err)
		}
	}
	c.follows = nil
}
