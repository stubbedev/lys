package files

import (
	"slices"

	"github.com/stubbedev/lys/desktop/vfs"
	"golang.org/x/sys/unix"
)

// sameVolume reports whether every one of paths lies where dir does: on
// its local filesystem, or on its network location's server.
func sameVolume(dir string, paths []string) bool {
	if !vfs.IsRemote(dir) && !slices.ContainsFunc(paths, vfs.IsRemote) {
		return sameFilesystem(dir, paths)
	}
	root := vfs.Root(dir)
	for _, p := range paths {
		if !vfs.IsRemote(p) || vfs.Root(p) != root {
			return false
		}
	}
	return true
}

// sameFilesystem reports whether every one of paths is on dir's
// filesystem; one that cannot be read counts as another.
func sameFilesystem(dir string, paths []string) bool {
	var st unix.Stat_t
	if unix.Stat(dir, &st) != nil {
		return false
	}
	dev := st.Dev
	for _, p := range paths {
		if unix.Lstat(p, &st) != nil || st.Dev != dev {
			return false
		}
	}
	return true
}
