package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestTheWindowOpensOnTheOverview(t *testing.T) {
	k := testKit(t, "")
	w := newWindow(k, k.store.svc.Config(), (&fakeAfter{}).after)
	w.open(launch{})
	if w.pageID != overviewID {
		t.Errorf("the window opens on %q, want the overview", w.pageID)
	}
}

func TestTheOverviewHasATileOpeningEachSection(t *testing.T) {
	k := testKit(t, "")
	w := newWindow(k, k.store.svc.Config(), (&fakeAfter{}).after)
	tiles := treetest.All[*widget.Button](overviewTiles(k))
	want := len(w.sections) - 1
	if len(tiles) != want {
		t.Fatalf("%d tiles for %d sections besides home", len(tiles), want)
	}
	for i, s := range w.sections[1:] {
		if name := tiles[i].AccessibleName(); name != locale.T().Get(s.key) {
			t.Errorf("tile %d is %q, want %q", i, name, locale.T().Get(s.key))
		}
		tiles[i].OnClick()
		if w.pageID != s.pages[0].id {
			t.Errorf("the %s tile opened %q, want %q", s.key, w.pageID, s.pages[0].id)
		}
		if !locale.T().HasAttr(s.key, "description") {
			t.Errorf("%s has no description for its tile", s.key)
		}
	}
}

func TestTheOverviewLeavesOutWhatIsNotInstalled(t *testing.T) {
	k := testKit(t, "")
	k.parts = parts{shell: true}
	newWindow(k, k.store.svc.Config(), (&fakeAfter{}).after)
	for _, b := range treetest.All[*widget.Button](overviewTiles(k)) {
		if b.AccessibleName() == locale.T().Get("settings-nav-displays") || b.AccessibleName() == locale.T().Get("settings-nav-input") {
			t.Errorf("lys-shell alone shows the %q tile", b.AccessibleName())
		}
	}
}
