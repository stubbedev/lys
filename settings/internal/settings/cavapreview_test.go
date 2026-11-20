package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/cavaview"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func previewedSpectrum(t *testing.T, p *settingsPage) *cavaview.Spectrum {
	t.Helper()
	var found []*cavaview.Spectrum
	for _, preview := range treetest.All[*livePreview](p) {
		if preview.AccessibleName() == locale.T().Get("settings-cava-preview") {
			found = append(found, treetest.All[*cavaview.Spectrum](preview)...)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the cava page previews %d visualizers, want one", len(found))
	}
	return found[0]
}

func spectrumLength(s *cavaview.Spectrum) int {
	return widget.SizeWithin(s, widget.Size{W: 4000, H: 4000}).W
}

func TestTheCavaPagePreviewsTheVisualizerItsDraftDraws(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, cavaPage(nil))
	s := previewedSpectrum(t, p)
	bars := int(config.DefaultsCava().Bars)
	if got := s.Values(); !slices.Equal(got, cavaSampleFrame(bars)) {
		t.Errorf("the default visualizer previews %v, want the sample frame over %d bars", got, bars)
	}
	before := spectrumLength(s)

	set(t, k, p, cavaPath+".bars", int64(13))
	if got := previewedSpectrum(t, p).Bars(); got != 13 {
		t.Errorf("13 bars preview %d", got)
	}
	set(t, k, p, cavaPath+".stereo", true)
	if got := previewedSpectrum(t, p).Bars(); got != 14 {
		t.Errorf("13 stereo bars preview %d, want them rounded up to 14 as lys-shell does", got)
	}
	set(t, k, p, cavaPath+".bars", int64(bars))
	set(t, k, p, cavaPath+".stereo", false)
	set(t, k, p, cavaPath+".bar-width", int64(config.DefaultsCava().BarWidth)+4)
	if after := spectrumLength(previewedSpectrum(t, p)); after != before+4*bars {
		t.Errorf("bars 4px wider preview %dpx long, want %d", after, before+4*bars)
	}
}

func TestThePeaksPreviewShowsFallingCaps(t *testing.T) {
	cfg := config.Defaults()
	cfg.Cava.Style = config.CavaStylePeaks
	k := testKit(t, "")
	s, err := cavaSampleSpectrum(cfg, k.palette())
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Values(); !slices.Equal(got, cavaSampleFrame(s.Bars())) {
		t.Errorf("the peaks style previews %v, want the sample frame", got)
	}
	cfg.Cava.Color = config.ColorValue{Kind: config.ColorCustom, Hex: "nonsense"}
	if _, err := cavaSampleSpectrum(cfg, k.palette()); err == nil {
		t.Error("a color that does not resolve previewed a visualizer")
	}
}
