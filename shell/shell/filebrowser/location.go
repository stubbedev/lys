package filebrowser

import (
	"strings"

	"github.com/stubbedev/lys/ui/widget"
)

// maxCompletions bounds the location entry's matches.
const maxCompletions = 200

// locationEntry is the typed path entry (Ctrl+L): its last segment
// completes from the folder the text names, and Tab takes the common
// prefix of the matches.
type locationEntry struct {
	*widget.ComboEntry
	tab func() bool
}

// TrapTab implements widget.TabTrapper: Tab completes while there are
// matches, and moves the focus otherwise.
func (l *locationEntry) TrapTab(bool) bool { return l.tab() }

// HitTest answers the wrapper for the entry itself, so a click focuses
// what traps Tab.
func (l *locationEntry) HitTest(p widget.Point) widget.Widget {
	w := l.ComboEntry.HitTest(p)
	if w == widget.Widget(l.Entry) || w == widget.Widget(l.ComboEntry) {
		return l
	}
	return w
}

// pathCompletion is the location entry's completion: the folder listed
// and its entries, the folder being listed, and the matches shown.
type pathCompletion struct {
	dir     string
	entries []Entry
	loading string
	items   []string
}

// completeLocation offers the entries of the folder text names whose
// names start with its last segment, folders first, at most
// maxCompletions, in the form typed; the folder is listed off the loop
// the first time.
func (c *Browser) completeLocation(text string) {
	typedDir, segment := "", text
	if i := strings.LastIndexByte(text, '/'); i >= 0 {
		typedDir, segment = text[:i+1], text[i+1:]
	}
	base := c.s.dir
	if VirtualLocation(base) {
		base = c.d.Home
	}
	dir := base
	if typedDir != "" {
		dir = ResolvePath(typedDir, base, c.d.Home)
	}
	pc := &c.pc
	if dir != pc.dir {
		pc.items = nil
		c.ui.location.SetItems(nil)
		if dir == pc.loading {
			return
		}
		pc.loading = dir
		reader, kinds := c.reader, c.s.kinds
		OffLoop(c, func() []Entry {
			entries, _, err := reader.List(dir, kinds)
			if err != nil {
				return nil
			}
			SortByName(entries)
			return entries
		}, func(entries []Entry) {
			if pc.loading != dir {
				return
			}
			pc.dir, pc.entries, pc.loading = dir, entries, ""
			c.completeLocation(c.ui.location.Text())
		})
		return
	}
	showHidden := c.hidden || strings.HasPrefix(segment, ".")
	pc.items = pc.items[:0]
	for _, e := range pc.entries {
		name := e.Name()
		if !strings.HasPrefix(name, segment) || (!showHidden && hiddenName(name, nil)) {
			continue
		}
		item := typedDir + name
		if e.IsDir {
			item += "/"
		}
		pc.items = append(pc.items, item)
		if len(pc.items) == maxCompletions {
			break
		}
	}
	c.ui.location.SetItems(pc.items)
}

// tabLocation completes the location entry to the longest prefix its
// matches share; false without matches, so Tab moves on.
func (c *Browser) tabLocation() bool {
	items := c.pc.items
	if len(items) == 0 {
		return false
	}
	prefix := items[0]
	for _, item := range items[1:] {
		n := 0
		for n < len(prefix) && n < len(item) && prefix[n] == item[n] {
			n++
		}
		prefix = prefix[:n]
	}
	if len(prefix) > len(c.ui.location.Text()) {
		c.ui.location.SetText(prefix)
	}
	return true
}
