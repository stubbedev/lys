// Package desktopentry is gio's desktop-entry layer: the key-file
// parser, the XDG application list with gio's load and visibility
// rules, Exec expansion, and launching.
package desktopentry

import "strings"

// Icon returns the Icon key of the desktop entry with the given id
// (with or without the .desktop suffix), searching dirs in order; false
// when no entry loads or it names no icon. The media module resolves
// application icons with it.
func Icon(id string, dirs []string) (string, bool) {
	if id == "" {
		return "", false
	}
	if !strings.HasSuffix(id, ".desktop") {
		id += ".desktop"
	}
	app, ok := FindApp(dirs, id)
	if !ok || app.Icon() == "" {
		return "", false
	}
	return app.Icon(), true
}
