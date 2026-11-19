package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func treemanPageLabels(t *testing.T, installed bool) []string {
	t.Helper()
	was := treemanInstalled
	treemanInstalled = func() bool { return installed }
	t.Cleanup(func() { treemanInstalled = was })
	k := testKit(t, "")
	spec, ok := pageByID(layout(k.store.svc.Config(), bothParts), "treeman")
	if !ok {
		t.Fatal("no treeman page")
	}
	return treetest.Labels(spec.lead(k))
}

func TestTheTreemanPageSaysWhatItShowsAndWhetherTreemanIsInstalled(t *testing.T) {
	tr := locale.T()
	about, missing := tr.Get("settings-treeman-about"), tr.Get("settings-treeman-missing")
	if labels := treemanPageLabels(t, false); !slices.Contains(labels, about) || !slices.Contains(labels, missing) {
		t.Errorf("without treeman the page reads %q, want what it shows and that treeman is missing", labels)
	}
	if labels := treemanPageLabels(t, true); !slices.Contains(labels, about) || slices.Contains(labels, missing) {
		t.Errorf("with treeman the page reads %q, want what it shows alone", labels)
	}
}
