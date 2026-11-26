// Package filemanager1 serves org.freedesktop.FileManager1, the
// interface browsers and other apps call to show a folder, or a
// downloaded file in its folder, in the desktop's file manager.
package filemanager1

import (
	"fmt"
	"path/filepath"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/transfer"
)

// D-Bus identity.
const (
	ServiceName = "org.freedesktop.FileManager1"
	ServicePath = dbus.ObjectPath("/org/freedesktop/FileManager1")
	iface       = "org.freedesktop.FileManager1"
)

// Host acts on the requests, each with local paths and the activation
// token the caller sent ("" for none).
type Host interface {
	ShowFolders(paths []string, token string)
	ShowItems(paths []string, token string)
	ShowItemProperties(paths []string, token string)
}

// Daemon serves a Host as org.freedesktop.FileManager1.
type Daemon struct{ host Host }

// NewDaemon wraps host for export.
func NewDaemon(host Host) *Daemon { return &Daemon{host: host} }

// Export serves the interface under its name; the returned release
// drops the name.
func (d *Daemon) Export(conn *dbus.Conn) (func(), error) {
	return dbusx.Serve(conn, dbusx.Service{Name: ServiceName, Path: ServicePath, Interface: iface, Methods: d})
}

// ShowFolders opens each folder.
func (d *Daemon) ShowFolders(uris []string, startupID string) *dbus.Error {
	return d.call(uris, startupID, d.host.ShowFolders)
}

// ShowItems shows each file selected in its folder.
func (d *Daemon) ShowItems(uris []string, startupID string) *dbus.Error {
	return d.call(uris, startupID, d.host.ShowItems)
}

// ShowItemProperties shows each file's properties.
func (d *Daemon) ShowItemProperties(uris []string, startupID string) *dbus.Error {
	return d.call(uris, startupID, d.host.ShowItemProperties)
}

// call hands the request's paths to show; a request naming no file,
// or one that is not local, is refused whole.
func (d *Daemon) call(uris []string, startupID string, show func([]string, string)) *dbus.Error {
	paths, err := Paths(uris)
	if err != nil {
		return dbusx.InvalidArgs(err.Error())
	}
	show(paths, startupID)
	return nil
}

// Paths are the local paths of uris; an empty list or a URI that names
// no local file is an error.
func Paths(uris []string) ([]string, error) {
	if len(uris) == 0 {
		return nil, fmt.Errorf("%s: no URI given", iface)
	}
	paths := make([]string, 0, len(uris))
	for _, u := range uris {
		p, ok := transfer.URIPath(u)
		if !ok {
			return nil, fmt.Errorf("%s: %q is not a local file", iface, u)
		}
		paths = append(paths, filepath.Clean(p))
	}
	return paths, nil
}
