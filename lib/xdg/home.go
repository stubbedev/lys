package xdg

import "strings"

// CutTilde reports whether path names the home directory as a shell
// does, "~" alone or "~/…", and returns what follows the "~": "" or a
// path starting with "/". "~user" and "~foo" are not home paths.
func CutTilde(path string) (rest string, ok bool) {
	rest, ok = strings.CutPrefix(path, "~")
	if !ok || rest != "" && rest[0] != '/' {
		return "", false
	}
	return rest, true
}

// ExpandHome replaces the "~" of a home path (CutTilde) with home,
// keeping the rest, a trailing slash included, as written. Any other
// path, and every path when home is "", is returned unchanged.
func ExpandHome(path, home string) string {
	rest, ok := CutTilde(path)
	if !ok || home == "" {
		return path
	}
	if rest == "" {
		return home
	}
	return strings.TrimSuffix(home, "/") + rest
}

// CutHome reports whether path is home or lies under it, a whole path
// element at a time (/home/me2 is not under /home/me), and returns what
// follows home: "" or a path starting with "/". A home of "" or "/"
// holds nothing.
func CutHome(path, home string) (rest string, ok bool) {
	home = strings.TrimSuffix(home, "/")
	if home == "" {
		return "", false
	}
	rest, ok = strings.CutPrefix(path, home)
	if !ok || rest != "" && rest[0] != '/' {
		return "", false
	}
	return rest, true
}

// CollapseHome is path as shown with home as "~" (CutHome): ~ or ~/…,
// or path unchanged outside home.
func CollapseHome(path, home string) string {
	rest, ok := CutHome(path, home)
	if !ok {
		return path
	}
	return "~" + rest
}
