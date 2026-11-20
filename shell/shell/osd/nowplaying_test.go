package osd

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/mpris"
	"github.com/stubbedev/lys/desktop/shell/osdview"
)

func track(title string, requested bool) mpris.TrackChange {
	return mpris.TrackChange{Player: mpris.Player{Title: title, Artist: "Band"}, Requested: requested}
}

func TestOsdMediaFiltersTrackChanges(t *testing.T) {
	for _, tc := range []struct {
		mode      config.OsdMedia
		requested bool
		want      bool
	}{
		{config.OsdMediaRequested, true, true},
		{config.OsdMediaRequested, false, false},
		{config.OsdMediaAlways, false, true},
		{config.OsdMediaAlways, true, true},
		{config.OsdMediaNever, true, false},
		{config.OsdMediaNever, false, false},
	} {
		cfg := quietConfig()
		cfg.Osd.Media = tc.mode
		h := readyHarness(t, cfg)
		h.osd.Show(NowPlaying(track("Song", tc.requested)))
		if got := len(h.opened) == 1; got != tc.want {
			t.Errorf("osd.media %s, requested %v: shown %v, want %v", tc.mode, tc.requested, got, tc.want)
		}
	}
	cfg := quietConfig()
	cfg.Osd.Media = config.OsdMediaNever
	h := readyHarness(t, cfg)
	h.osd.Show(osdview.Slider("Speakers", "", 40, false))
	if len(h.opened) != 1 {
		t.Error("osd.media never also silenced the volume slider")
	}
}

func TestNowPlayingCarriesTheTrack(t *testing.T) {
	c := mpris.TrackChange{Player: mpris.Player{Title: "Song", Artist: "Band", ArtURL: "file:///covers/song.png"}, Requested: true}
	if got, want := NowPlaying(c), osdview.Media("Song", "Band", "file:///covers/song.png", true); got != want {
		t.Errorf("a track change flashes %+v, want %+v", got, want)
	}
	if NowPlaying(track("Song", false)) == NowPlaying(track("Song", true)) {
		t.Error("an unrequested track change flashes as a requested one")
	}
}
