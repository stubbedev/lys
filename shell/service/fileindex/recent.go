package fileindex

import (
	"bufio"
	"encoding/xml"
	"io"
	"net/url"
	"os"
	"slices"
	"strings"
)

// recentBytes bounds the recently-used.xbel read.
const recentBytes = 4 << 20

// ReadRecent reads the most recent local files of an xbel, the format
// every toolkit writes, newest first, each once and only while it
// exists. Missing or unreadable is nothing: the file belongs to whichever
// toolkit made it, and may not exist.
func ReadRecent(path string, limit int) []string {
	file, err := os.Open(path) //nolint:gosec // the desktop's recently-used list
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()
	type recent struct {
		path string
		when string
	}
	var found []recent
	dec := xml.NewDecoder(io.LimitReader(bufio.NewReader(file), recentBytes))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "bookmark" {
			continue
		}
		var href, when string
		for _, a := range start.Attr {
			switch a.Name.Local {
			case "href":
				href = a.Value
			case "modified":
				when = a.Value
			case "visited":
				if when == "" {
					when = a.Value
				}
			}
		}
		u, err := url.Parse(href)
		if err != nil || u.Scheme != "file" || u.Path == "" {
			continue
		}
		found = append(found, recent{path: u.Path, when: when})
	}
	slices.SortStableFunc(found, func(a, b recent) int { return strings.Compare(b.when, a.when) })
	out := make([]string, 0, min(limit, len(found)))
	for _, r := range found {
		if len(out) == limit {
			break
		}
		if _, err := os.Stat(r.path); err == nil && !slices.Contains(out, r.path) {
			out = append(out, r.path)
		}
	}
	return out
}
