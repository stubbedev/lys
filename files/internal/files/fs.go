package files

import (
	"golang.org/x/sys/unix"
)

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
