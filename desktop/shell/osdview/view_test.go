package osdview

import (
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

func TestViewFollowsTheEvent(t *testing.T) {
	cfg := config.DefaultsOsd()
	cfg.TextAlign = config.OsdTextAlignEnd
	v := New(treetest.Font(t), 1)
	v.Apply(Slider("Speakers", VolumeIcon(0, true), 42.6, true), cfg)
	for _, l := range []*widget.Label{v.sliderLabel, v.value, v.toggleLabel} {
		if l.Color() != 0 {
			t.Errorf("label %v pins ink %#08x over the stylesheet", l.Classes(), uint32(l.Color()))
		}
	}
	if !slices.Equal(v.container.Classes(), []string{"osd", css.Glass, "muted", "bordered", css.Card}) {
		t.Errorf("slider classes %v", v.container.Classes())
	}
	if !v.sliderHeader.Visible() || !v.bar.Visible() || v.toggleHeader.Visible() {
		t.Error("a level shows the slider header and the bar only")
	}
	if v.sliderLabel.Text() != "Speakers" || v.value.Text() != "43%" || v.bar.Value() != 0.426 {
		t.Errorf("slider reads %q %q %v", v.sliderLabel.Text(), v.value.Text(), v.bar.Value())
	}

	v.Apply(Toggle("Caps Lock", "ld-a-large-small-symbolic", false), cfg)
	if !slices.Equal(v.container.Classes(), []string{"osd", css.Glass, "toggle-off", "bordered", css.Card}) {
		t.Errorf("toggle classes %v", v.container.Classes())
	}
	if v.sliderHeader.Visible() || v.bar.Visible() || !v.toggleHeader.Visible() {
		t.Error("a toggle shows the toggle header only")
	}
	if v.toggleLabel.Text() != "\u2068Caps Lock\u2069 Off" || v.toggleHeader.HAlign() != widget.AlignEnd {
		t.Errorf("toggle reads %q aligned %v", v.toggleLabel.Text(), v.toggleHeader.HAlign())
	}

	v.Apply(Toast("Done", "").WithClass("success"), cfg)
	if !slices.Equal(v.container.Classes(), []string{"osd", css.Glass, "bordered", "success", css.Card}) {
		t.Errorf("toast classes %v", v.container.Classes())
	}
	if v.toggleIcon.Visible() || v.toggleLabel.Text() != "Done" {
		t.Error("a toast without an icon shows its label only")
	}
	v.Apply(Toast("Named", "no-such-icon-anywhere"), cfg)
	if v.toggleIcon.Visible() {
		t.Error("an icon nothing resolves keeps its empty space")
	}
	v.Apply(Toast("Named", "list-add"), cfg)
	if !v.toggleIcon.Visible() {
		t.Error("an icon that resolves stays hidden")
	}
	cfg.Border = false
	v.Apply(Toast("Plain", "").WithClass("success"), cfg)
	if slices.Contains(v.container.Classes(), "bordered") {
		t.Errorf("osd.border off still borders the card: %v", v.container.Classes())
	}
}

func TestATrackShowsTitleArtistAndArt(t *testing.T) {
	cfg := config.DefaultsOsd()
	v := New(treetest.Font(t), 1)
	v.Apply(Media("Song", "Band", "", true), cfg)
	if !v.mediaHeader.Visible() || v.sliderHeader.Visible() || v.toggleHeader.Visible() || v.bar.Visible() {
		t.Fatal("a track change did not show the media header alone")
	}
	if v.mediaTitle.Text() != "Song" || v.mediaArtist.Text() != "Band" {
		t.Errorf("media header reads %q / %q", v.mediaTitle.Text(), v.mediaArtist.Text())
	}
	if len(v.art.Children()) != 1 {
		t.Error("a track without art shows no placeholder")
	}
	v.Apply(Media("Next", "", "file:///nonexistent/cover.png", true), cfg)
	if v.mediaArtist.Visible() {
		t.Error("an empty artist line stayed visible")
	}
	if len(v.art.Children()) != 1 {
		t.Error("the art box does not hold exactly the cover")
	}
	v.Apply(Slider("Speakers", "", 40, false), cfg)
	if v.mediaHeader.Visible() || !v.sliderHeader.Visible() {
		t.Error("a slider after a track change kept the media header")
	}
}

func TestTheCardKeepsItsWidth(t *testing.T) {
	const scale = 1.5
	v := New(treetest.Font(t), scale)
	v.Apply(Slider("Speakers", VolumeIcon(40, false), 40, false), config.DefaultsOsd())
	sz := widget.SizeWithinAsChild(v.container, widget.Size{W: 1920, H: 1080})
	if want := int(widthPx * scale); sz.W != want {
		t.Errorf("the OSD measures %dpx wide in a 1920px surface, want %d", sz.W, want)
	}
}

func TestAPreviewIsRevealedAtOnceUnderItsConfig(t *testing.T) {
	cfg := config.Defaults()
	cfg.Styling.Scale = 2
	v := Preview(cfg, treetest.Font(t), Slider("Speakers", "", 40, false))
	if !v.Root().Revealed() || v.Root().Progress() != 1 {
		t.Errorf("a preview shows at %v, want fully revealed", v.Root().Progress())
	}
	if v.Scale() != 2 || v.Root().HasClass("osd-host") {
		t.Errorf("a preview at scale %v with classes %v, want scale 2 and no surface role", v.Scale(), v.Root().Classes())
	}
	if v.sliderLabel.Text() != "Speakers" {
		t.Errorf("a preview shows %q, want its event", v.sliderLabel.Text())
	}
}

func TestHostMakesTheRevealerTheSurfaceRoot(t *testing.T) {
	v := New(treetest.Font(t), 1)
	v.Host(nil)
	if !v.Root().HasClass("osd-host") || !v.Root().HasClass("background") {
		t.Errorf("a hosted OSD's root has classes %v", v.Root().Classes())
	}
}

func TestIcons(t *testing.T) {
	for _, tc := range []struct {
		pct   float64
		muted bool
		want  string
	}{
		{50, true, "ld-volume-x-symbolic"},
		{0, false, "ld-volume-x-symbolic"},
		{33, false, "ld-volume-symbolic"},
		{34, false, "ld-volume-1-symbolic"},
		{66, false, "ld-volume-1-symbolic"},
		{67, false, "ld-volume-2-symbolic"},
	} {
		if got := VolumeIcon(tc.pct, tc.muted); got != tc.want {
			t.Errorf("VolumeIcon(%v, %v) = %s, want %s", tc.pct, tc.muted, got, tc.want)
		}
	}
	if MicrophoneIcon(true) != "ld-mic-off-symbolic" || MicrophoneIcon(false) != "ld-mic-symbolic" {
		t.Error("microphone icons")
	}
}

func TestAnEventsDurationOverridesTheConfigs(t *testing.T) {
	cfg := config.DefaultsOsd()
	cfg.Duration = 1500
	if got := Toast("x", "").Duration(cfg); got != 1500*time.Millisecond {
		t.Errorf("a toast without its own duration shows %v, want osd.duration", got)
	}
	if got := Toast("x", "").WithDuration(900).Duration(cfg); got != 900*time.Millisecond {
		t.Errorf("a toast with --duration 900 shows %v", got)
	}
}

func TestOnlyATrackChangeFollowsOsdMedia(t *testing.T) {
	if !Slider("Speakers", "", 40, false).Wanted(config.OsdMediaNever) {
		t.Error("osd.media never silenced a slider")
	}
	if Media("Song", "", "", false).Wanted(config.OsdMediaRequested) {
		t.Error("osd.media requested showed an unrequested track")
	}
	if !Media("Song", "", "", true).Wanted(config.OsdMediaRequested) {
		t.Error("osd.media requested hid a requested track")
	}
}
