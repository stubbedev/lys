package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestTheMotionPageAnimatesEverySurfaceInOneTable(t *testing.T) {
	k := testKit(t, "")
	spec := pageRow(t, motionPageID, "animations.notifications.enter")
	edited := append([]string{spec.path}, spec.also...)
	surfaces := animationSurfaces()
	for _, want := range []string{"animations.notifications", "animations.lock", "animations.launcher", "animations.menu", "animations.print"} {
		if !slices.Contains(surfaces, want) {
			t.Errorf("the surfaces %v lack %s", surfaces, want)
		}
	}
	if slices.Contains(surfaces, "animations.enter") {
		t.Error("the default enter transition is taken for a surface")
	}
	for _, s := range surfaces {
		for _, leaf := range []string{".enter", ".exit", ".enter-duration", ".exit-duration"} {
			if !slices.Contains(edited, s+leaf) {
				t.Errorf("the table does not edit %s%s", s, leaf)
			}
		}
	}
	if spec.enabledWhen.path != "animations.enabled" {
		t.Errorf("the table follows %q, not the animations switch", spec.enabledWhen.path)
	}
	g, ok := fieldControl(k, k.store, spec).(*cellGrid)
	if !ok {
		t.Fatal("the surface animations are not a table")
	}
	if len(g.cells) != 4*len(surfaces) {
		t.Fatalf("%d cells for %d surfaces", len(g.cells), len(surfaces))
	}
	tr := locale.T()
	labels := treetest.Labels(g)
	for _, key := range []string{"settings-animations-lock", "settings-animations-launcher", "settings-animations-enter-duration"} {
		if !slices.Contains(labels, tr.Get(key)) {
			t.Errorf("the table lacks %q: %q", tr.Get(key), labels)
		}
	}
}

func TestASurfacePageLinksToItsAnimationInsteadOfOwningIt(t *testing.T) {
	pages := layout(config.Defaults(), bothParts)
	for _, id := range []string{barPageID, "notifications", "osd", "launcher", "wallpaper", "lock", "privacy"} {
		p, ok := pageByID(pages, id)
		if !ok {
			t.Fatalf("no page %q", id)
		}
		for _, sec := range p.sections {
			for _, r := range sec.flatRows() {
				if strings.HasPrefix(r.path, "animations.") {
					t.Errorf("the %s page still edits %s", id, r.path)
				}
			}
		}
		if !slices.Contains(p.links, surfaceAnimationLink) {
			t.Errorf("the %s page does not link to its animation", id)
		}
	}
	k := testKit(t, "")
	k.nav = layout(config.Defaults(), parts{})
	if labels := treetest.Labels(linksSection(k, "settings-section-related", []pageLink{surfaceAnimationLink})); len(labels) != 0 {
		t.Errorf("without the Motion page the link still shows %q", labels)
	}
}
