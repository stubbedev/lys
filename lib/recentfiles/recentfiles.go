// Package recentfiles is the desktop's list of recently used files, the
// XDG recently-used.xbel every toolkit reads and writes: where it lives,
// reading it newest first, and the document that lists entries.
package recentfiles

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/stubbedev/lys/lib/fileuri"
	"github.com/stubbedev/lys/lib/xdg"
)

// maxBytes bounds a read: the list belongs to whichever toolkit wrote
// it, and its consumers show a handful of entries.
const maxBytes = 4 << 20

// ErrNoDataHome is an environment that names no user data directory
// for the list to live in.
var ErrNoDataHome = errors.New("recentfiles: neither XDG_DATA_HOME nor HOME is an absolute path")

// DefaultPath is the spec's location under env's user data directory,
// $XDG_DATA_HOME or ~/.local/share. An environment with neither is
// ErrNoDataHome: the list is never kept relative to the working
// directory.
func DefaultPath(env xdg.Env) (string, error) {
	base, ok := env.DataHome()
	if !ok {
		return "", ErrNoDataHome
	}
	return filepath.Join(base, "recently-used.xbel"), nil
}

// Entry is one recently used file: its path (decoded from the file://
// URI on disk), the application that recorded it as the list names it,
// and when it was last used, zero when the list gives no time.
type Entry struct {
	Path string
	App  string
	When time.Time
}

// Load reads the list at path: its local file:// entries, newest first
// by their modified time, else their visited one, entries the list
// gives the same or no time keeping its order. A missing file is no
// entries; one larger than 4 MiB or that is not an xbel document is an
// error.
func Load(path string) ([]Entry, error) {
	f, err := os.Open(path) //nolint:gosec // the desktop's recently-used list
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recentfiles: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("recentfiles: read %s: %w", path, err)
	}
	if len(data) > maxBytes {
		return nil, fmt.Errorf("recentfiles: %s is larger than %d bytes", path, maxBytes)
	}
	var doc xbel
	if err := xml.NewDecoder(bytes.NewReader(data)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("recentfiles: parse %s: %w", path, err)
	}
	entries := make([]Entry, 0, len(doc.Bookmarks))
	for _, b := range doc.Bookmarks {
		local, ok := fileuri.Path(b.Href)
		if !ok {
			continue
		}
		entries = append(entries, Entry{Path: local, App: b.app(), When: b.when()})
	}
	slices.SortStableFunc(entries, func(a, b Entry) int { return b.When.Compare(a.When) })
	return entries, nil
}

// Marshal is the xbel document listing entries in their order, each
// recorded by owner.
func Marshal(entries []Entry, owner string) ([]byte, error) {
	doc := xbel{Version: "1.0", XMLNSBookmark: "http://www.freedesktop.org/standards/desktop-bookmarks"}
	doc.Bookmarks = make([]xbelBookmark, 0, len(entries))
	for _, e := range entries {
		stamp := e.When.UTC().Format(time.RFC3339)
		doc.Bookmarks = append(doc.Bookmarks, xbelBookmark{
			Href: fileuri.FromPath(e.Path), Added: stamp, Modified: stamp, Visited: stamp,
			Infos: []xbelInfo{{Metadatas: []xbelMetadata{{Owner: owner, App: xbelApp{Name: e.App}}}}},
		})
	}
	data, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("recentfiles: encode %d entries: %w", len(entries), err)
	}
	out := make([]byte, 0, len(xml.Header)+len(data)+1)
	out = append(out, xml.Header...)
	out = append(out, data...)
	return append(out, '\n'), nil
}

// xbel is the document; only the fields lys reads and writes are
// modeled.
type xbel struct {
	XMLName       xml.Name       `xml:"xbel"`
	Version       string         `xml:"version,attr"`
	XMLNSBookmark string         `xml:"xmlns:bookmark,attr"`
	Bookmarks     []xbelBookmark `xml:"bookmark"`
}

type xbelBookmark struct {
	Href     string     `xml:"href,attr"`
	Added    string     `xml:"added,attr,omitempty"`
	Modified string     `xml:"modified,attr,omitempty"`
	Visited  string     `xml:"visited,attr,omitempty"`
	Infos    []xbelInfo `xml:"info"`
}

// when is the bookmark's modified time, else its visited one; zero for
// neither.
func (b xbelBookmark) when() time.Time {
	for _, stamp := range []string{b.Modified, b.Visited} {
		if t, err := time.Parse(time.RFC3339, stamp); err == nil {
			return t
		}
	}
	return time.Time{}
}

// app is the first application the bookmark's metadata names.
func (b xbelBookmark) app() string {
	if len(b.Infos) == 0 || len(b.Infos[0].Metadatas) == 0 {
		return ""
	}
	return b.Infos[0].Metadatas[0].App.Name
}

type xbelInfo struct {
	Metadatas []xbelMetadata `xml:"metadata"`
}

type xbelMetadata struct {
	Owner string  `xml:"owner,attr"`
	App   xbelApp `xml:"app"`
}

type xbelApp struct {
	Name string `xml:"name,attr,omitempty"`
}
