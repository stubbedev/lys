package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const treemanInLayout = "bar:\n  layout:\n    - monitor: \"*\"\n      right: [clock, treeman]\n"

func sidebarLabels(t *testing.T, yaml string, treeman bool) (*window, []string) {
	t.Helper()
	was := treemanInstalled
	treemanInstalled = func() bool { return treeman }
	t.Cleanup(func() { treemanInstalled = was })
	k := testKit(t, yaml)
	w := newWindow(k, k.store.svc.Config(), (&fakeAfter{}).after)
	return w, treetest.Labels(w.sidebar)
}

func TestTheTreemanPageIsListedOnlyWithTreemanOrInTheBar(t *testing.T) {
	treeman, weather := locale.T().Get("settings-nav-treeman"), locale.T().Get("settings-nav-weather")
	w, labels := sidebarLabels(t, "", false)
	if slices.Contains(labels, treeman) || !slices.Contains(labels, weather) {
		t.Errorf("without treeman and outside the bar the sidebar reads %q", labels)
	}
	if _, ok := pageByID(w.sections, "treeman"); !ok {
		t.Error("an unlisted page can no longer be opened by a link or a search")
	}
	w.show("treeman")
	if w.pageID != "treeman" {
		t.Errorf("opening the unlisted page shows %q", w.pageID)
	}
	if _, labels := sidebarLabels(t, "", true); !slices.Contains(labels, treeman) {
		t.Errorf("with treeman installed the sidebar reads %q", labels)
	}
	if _, labels := sidebarLabels(t, treemanInLayout, false); !slices.Contains(labels, treeman) {
		t.Errorf("with treeman in the bar the sidebar reads %q", labels)
	}
}
