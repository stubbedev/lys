package settings

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

func levelEditor(t *testing.T, yaml, page, path string) (*kit, *thresholdEditor) {
	t.Helper()
	k := testKit(t, yaml)
	e := fieldControl(k, k.store, pageRow(t, page, path)).(*thresholdEditor)
	e.bar.Arrange(render.Rect{W: 320, H: e.bar.barH() + levelRowH})
	return k, e
}

func TestEachVolumeIconSitsAboveItsShareOfTheThresholdBar(t *testing.T) {
	const yaml = "modules:\n  audio:\n    speaker-icons: [low, mid, high]\n    speaker-icon-muted: off\n"
	_, e := levelEditor(t, yaml, "audio", "modules.audio.thresholds")
	l := e.bar.levels
	if len(l.icons) != 3 || l.muted == nil || l.muted.Name() != "off" {
		t.Fatalf("%d level icons, muted %v", len(l.icons), l.muted)
	}
	for i, want := range []string{"low", "mid", "high"} {
		icon := l.icons[i]
		lo, hi := levelBand(i, 3)
		mid := icon.Bounds().X + icon.Bounds().W/2
		if icon.Name() != want || mid < e.bar.xOf(lo) || mid > e.bar.xOf(hi) {
			t.Errorf("icon %d is %q centred at %d, outside its band %d–%d", i, icon.Name(), mid, e.bar.xOf(lo), e.bar.xOf(hi))
		}
	}
	if unisolated(l.icons[0].TooltipText()) != "Shown from 0 to 33 %" {
		t.Errorf("the first band reads %q", l.icons[0].TooltipText())
	}
	if l.muted.Bounds().X+l.muted.Bounds().W > e.bar.track().X {
		t.Error("the muted icon overlaps the bar instead of capping its left end")
	}
	if icon := l.icons[0].Bounds(); icon.Y+icon.H > e.bar.track().Y {
		t.Error("the level icons overlap the track")
	}
}

func TestTheLevelBandsFollowTheIconList(t *testing.T) {
	k, e := levelEditor(t, "", thresholdsPageID, "thresholds.battery")
	if err := k.store.set("modules.battery.level-icons", []any{"empty", "full"}); err != nil {
		t.Fatal(err)
	}
	e.refresh()
	if l := e.bar.levels; len(l.icons) != 2 || l.icons[1].Name() != "full" || l.muted != nil {
		t.Fatalf("after two level icons the bar draws %d, muted %v", len(l.icons), l.muted)
	}
	if got := len(e.bar.Children()); got != len(e.bar.stops)+2 {
		t.Errorf("the bar has %d children, want its stops and two icons", got)
	}
}

func TestTheBrightnessFloorIsGreyedOnTheBar(t *testing.T) {
	_, e := levelEditor(t, "modules:\n  brightness:\n    min-brightness: 15\n", "brightness", "modules.brightness.thresholds")
	if e.bar.levels.floor != 15 {
		t.Fatalf("the floor reads %v", e.bar.levels.floor)
	}
	_, none := levelEditor(t, "", "audio", "modules.audio.thresholds")
	if none.bar.levels.floor != 0 {
		t.Errorf("a scale with no floor greys %v", none.bar.levels.floor)
	}
}

func TestAThresholdBarWithoutLevelsDrawsNoBands(t *testing.T) {
	_, e := thresholdKit(t)
	if e.bar.levels != nil {
		t.Fatal("the CPU threshold bar draws level icons")
	}
	if got := e.bar.Measure(widget.Column, 0).Nat; got != e.bar.barH() {
		t.Errorf("a plain threshold bar is %d tall, want %d", got, e.bar.barH())
	}
}

// The battery's bar alone drew a tick across its track at every boundary
// between two level icons, one even inside the fill, unlike every other
// threshold slider. The boundaries are read under the track only: no
// pixel above the track at a boundary is drawn.
func TestTheLevelBandsDrawNoTicksAcrossTheTrack(t *testing.T) {
	const yaml = "modules:\n  audio:\n    speaker-icons: [low, mid, high]\n"
	k, e := levelEditor(t, yaml, "audio", "modules.audio.thresholds")
	themed(k, e.bar)
	r := render.Rect{W: 320, H: e.bar.barH() + levelRowH}
	e.bar.Arrange(r)
	tr := e.bar.track()
	lo, _ := levelBand(1, 3)
	x, y := e.bar.xOf(lo), tr.Y-levelGap/2+1
	if y >= tr.Y {
		t.Fatalf("no room above the track at y=%d", tr.Y)
	}
	stride := render.Stride(r.W)
	data := make([]byte, stride*r.H)
	e.bar.Paint(render.New(data, stride, r.W, r.H))
	if a := data[y*stride+4*x+3]; a != 0 {
		t.Errorf("the bar paints at (%d, %d), above its track at a band boundary", x, y)
	}
}
