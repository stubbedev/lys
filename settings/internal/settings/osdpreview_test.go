package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const osdScreenW = 900

func onlyPreview(t *testing.T, p *settingsPage) *livePreview {
	t.Helper()
	previews := treetest.All[*livePreview](p)
	if len(previews) != 1 {
		t.Fatalf("the page holds %d previews, want one", len(previews))
	}
	return previews[0]
}

func previewedOsd(t *testing.T, p *settingsPage) (card *widget.Box, at render.Rect) {
	t.Helper()
	preview := onlyPreview(t, p)
	cards := treetest.WithClass(preview, "osd")
	if len(cards) != 1 {
		t.Fatalf("the preview shows %v instead of one OSD card", treetest.Labels(preview))
	}
	screen := treetest.First[*widget.Overlay](t, preview)
	widget.SizeWithin(screen, widget.Size{W: osdScreenW, H: osdPreviewScreenH})
	screen.Arrange(render.Rect{W: osdScreenW, H: osdPreviewScreenH})
	card, ok := cards[0].(*widget.Box)
	if !ok {
		t.Fatal("the OSD card is no box")
	}
	return card, card.Bounds()
}

func TestTheOsdPagePreviewsTheToastShowOsdFlashes(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, osdPage(nil))
	card, at := previewedOsd(t, p)
	sample := osdSampleToast()
	if labels := treetest.Labels(card); !slices.Contains(labels, sample.label) || !slices.Contains(labels, "70%") {
		t.Errorf("the OSD previews %v, want the toast Show OSD flashes", labels)
	}
	if !card.HasClass("bordered") {
		t.Errorf("the default OSD previews unbordered: %v", card.Classes())
	}
	if mid := at.X + at.W/2; at.Y < osdPreviewScreenH/2 || mid < osdScreenW/2-1 || mid > osdScreenW/2+1 {
		t.Errorf("the default OSD previews at %+v, want the bottom middle", at)
	}

	set(t, k, p, "osd.border", false)
	set(t, k, p, "osd.position", "top-left")
	card, at = previewedOsd(t, p)
	if card.HasClass("bordered") {
		t.Errorf("osd.border off previews a bordered card: %v", card.Classes())
	}
	if at.Y+at.H > osdPreviewScreenH/2 || at.X+at.W > osdScreenW/2 {
		t.Errorf("osd.position top-left previews the OSD at %+v", at)
	}
}

func TestADisabledOsdPreviewsThatNothingShows(t *testing.T) {
	k := testKit(t, "osd:\n  enabled: false\n")
	p := buildPage(k, osdPage(nil))
	preview := onlyPreview(t, p)
	if cards := treetest.WithClass(preview, "osd"); len(cards) != 0 {
		t.Error("a disabled OSD still previews its card")
	}
	if want := locale.T().Get("settings-osd-preview-off"); !slices.Contains(treetest.Labels(preview), want) {
		t.Errorf("the preview says %v, want %q", treetest.Labels(preview), want)
	}
	set(t, k, p, "osd.enabled", true)
	previewedOsd(t, p)
}

func TestTheOsdSampleShowsAnIconTheAppShips(t *testing.T) {
	if icon := osdSampleToast().icon; !slices.Contains(listedIcons(t), icon) {
		t.Errorf("the OSD sample shows %s, which resources/icons.txt does not list", icon)
	}
}
