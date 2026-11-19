package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestSettingIndexListsEveryRowOfEveryPage(t *testing.T) {
	sections := layout(config.Defaults(), bothParts)
	index := settingIndex(sections)
	rows := 0
	for _, s := range sections {
		for _, p := range s.pages {
			n := 0
			for _, sec := range p.sections {
				n += len(sec.flatRows())
			}
			for i := range n {
				if !slices.ContainsFunc(index, func(e settingEntry) bool { return e.page == p.id && e.row == i }) {
					t.Errorf("row %d of %s is not indexed", i, p.id)
				}
			}
			rows += n
		}
	}
	if len(index) != rows {
		t.Errorf("the index holds %d entries for %d rows", len(index), rows)
	}
}

func TestSearchRanksLabelsOverPlacesOverDescriptions(t *testing.T) {
	index := []settingEntry{
		{page: "a", label: "Width", place: "Bar › Opacity", order: 0},
		{page: "b", label: "Height", description: "the opacity of nothing", order: 1},
		{page: "c", label: "Background Opacity", place: "Bar › General", order: 2},
		{page: "d", label: "Location", place: "Bar › General", order: 3},
	}
	got := searchSettings(index, "opacity")
	want := []string{"c", "a", "b"}
	if len(got) != len(want) {
		t.Fatalf("hits %v, want pages %v", got, want)
	}
	for i, h := range got {
		if h.page != want[i] {
			t.Errorf("hit %d is %s, want %s", i, h.page, want[i])
		}
	}
	if got := searchSettings(index, "bar opacity"); len(got) != 2 || got[0].page != "c" {
		t.Errorf("every word must match: %v", got)
	}
}

func TestSearchWithoutAWordOrAMatchFindsNothing(t *testing.T) {
	index := settingIndex(layout(config.Defaults(), bothParts))
	for _, q := range []string{"", "   ", "zqxjv"} {
		if got := searchSettings(index, q); len(got) != 0 {
			t.Errorf("%q found %d settings", q, len(got))
		}
	}
	if got := searchSettings(index, "e"); len(got) != searchLimit {
		t.Errorf("a one-letter search lists %d settings, want the cap %d", len(got), searchLimit)
	}
}

func TestSearchJumpsToTheSetting(t *testing.T) {
	k := testKit(t, "")
	w := newWindow(k, k.store.svc.Config(), (&fakeAfter{}).after)
	var focused widget.Widget
	w.focus = func(target widget.Widget) { focused = target }
	w.search.SetText("tearing mode")
	if w.stack.Visible() != searchPage {
		t.Fatalf("typing shows %q, want the results", w.stack.Visible())
	}
	hit, ok := w.results.first()
	if !ok || hit.page != displaysID {
		t.Fatalf("the best hit for tearing mode is %+v", hit)
	}
	if len(w.results.button) != len(w.results.hits) || w.results.empty.Visible() {
		t.Fatalf("%d result rows for %d hits", len(w.results.button), len(w.results.hits))
	}
	w.results.button[0].ClickAt(widget.Point{})
	if w.pageID != displaysID || w.stack.Visible() != w.names[len(w.names)-1] || w.search.Text() != "" {
		t.Fatalf("after the jump page %q shows %q, search %q", w.pageID, w.stack.Visible(), w.search.Text())
	}
	row := w.page.rows[hit.row]
	if row.path != "general.tearing-mode" || w.page.revealing != row {
		t.Fatalf("the jump reveals %v, want the tearing row", w.page.revealing)
	}
	if focused != nil {
		t.Fatalf("focus went to %T before the page was laid out", focused)
	}
	w.root.Arrange(render.Rect{W: windowWidth, H: windowHeight})
	if focused == nil || focused != widget.FirstFocusable(row.ctl) {
		t.Errorf("focus went to %T, want the row's switch", focused)
	}
}

func TestSearchShowsItsEmptyStateAndEndsOnNavigation(t *testing.T) {
	k := testKit(t, "")
	w := newWindow(k, k.store.svc.Config(), (&fakeAfter{}).after)
	start := w.stack.Visible()
	w.search.SetText("zqxjv")
	if !w.results.empty.Visible() || w.results.list.Visible() {
		t.Fatal("a search without hits shows no empty state")
	}
	w.search.OnActivate(w.search.Text())
	if w.stack.Visible() != searchPage {
		t.Fatal("Enter without a hit left the results")
	}
	w.search.SetText("")
	if w.stack.Visible() != start {
		t.Fatalf("clearing shows %q, want the page %q", w.stack.Visible(), start)
	}
	w.search.SetText("scale")
	second := otherPage(w, w.pageID)
	sidebarItem(t, w, second.navKey).ClickAt(widget.Point{})
	if w.search.Text() != "" || w.pageID != second.id || w.stack.Visible() == searchPage {
		t.Errorf("navigating kept the search: text %q, page %q, shown %q", w.search.Text(), w.pageID, w.stack.Visible())
	}
}

func otherPage(w *window, id string) pageSpec {
	for _, s := range w.sections {
		for _, p := range s.pages {
			if p.id != id {
				return p
			}
		}
	}
	return pageSpec{}
}

func TestEveryRowOnEveryPageHasItsLabel(t *testing.T) {
	tr := locale.T()
	rows := 0
	for _, s := range layout(config.Defaults(), bothParts) {
		for _, p := range s.pages {
			for _, sec := range p.sections {
				for _, r := range sec.flatRows() {
					rows++
					if key := r.labelKey(); !tr.Has(key) {
						t.Errorf("%s on %s: no label (%s)", r.path, p.id, key)
					}
				}
			}
		}
	}
	if rows < 1000 {
		t.Fatalf("only %d rows; the walk missed the pages", rows)
	}
}

func TestEveryLauncherSearchKeyHasALabelledRow(t *testing.T) {
	p, _ := pageByID(layout(config.Defaults(), bothParts), "launcher-modes")
	labelled := map[string]bool{}
	for _, sec := range p.sections {
		for _, r := range sec.flatRows() {
			labelled[r.path] = locale.T().Has(r.labelKey())
		}
	}
	for _, key := range []string{"providers", "web-engine", "web-url", "file-roots", "file-exclude", "commands-dir", "currency-rates", "previews", "quicklinks", "snippets"} {
		if !labelled["launcher.search."+key] {
			t.Errorf("launcher.search.%s has no labelled row", key)
		}
	}
}

var unlistedShellKeys = map[string]string{
	"imports": "which files the config is assembled from, edited in the file itself",
}

func shellRowPaths() map[string]bool {
	rows := map[string]bool{}
	for _, s := range layout(config.Defaults(), bothParts) {
		for _, p := range s.pages {
			for _, sec := range p.sections {
				for _, r := range sec.flatRows() {
					if !p.wm && !r.wm {
						rows[r.path] = true
					}
				}
			}
		}
	}
	return rows
}

func rowCovers(rows map[string]bool, leaf string) bool {
	for at := leaf; ; at = at[:strings.LastIndex(at, ".")] {
		if rows[at] {
			return true
		}
		if !strings.Contains(at, ".") {
			break
		}
	}
	for r := range rows {
		if strings.HasPrefix(r, leaf+".") {
			return true
		}
	}
	return false
}

func TestEveryShellConfigKeyHasARow(t *testing.T) {
	runCompositor(t, "")
	rows := shellRowPaths()
	leaves := config.LeafPaths()
	if len(leaves) < 500 {
		t.Fatalf("only %d config leaves; the walk missed the schema", len(leaves))
	}
	for _, leaf := range leaves {
		_, unlisted := unlistedShellKeys[leaf]
		switch covered := rowCovers(rows, leaf); {
		case !covered && !unlisted:
			t.Errorf("%s has no row on any page", leaf)
		case covered && unlisted:
			t.Errorf("%s has a row but is listed as having none", leaf)
		}
	}
	for key := range unlistedShellKeys {
		if !slices.Contains(leaves, key) {
			t.Errorf("%s is listed as having no row but is no config key", key)
		}
	}
}

func TestARowCoversItsKeyItsParentAndItsChildren(t *testing.T) {
	rows := map[string]bool{"dock.pinned": true, "dropdowns.audio.width": true}
	for leaf, want := range map[string]bool{
		"dock.pinned":     true,
		"dock.pinned.x":   true,
		"dropdowns.audio": true,
		"dock.enabled":    false,
		"dropdowns.media": false,
		"dock.pin":        false,
	} {
		if got := rowCovers(rows, leaf); got != want {
			t.Errorf("rowCovers(%s) = %v, want %v", leaf, got, want)
		}
	}
}
