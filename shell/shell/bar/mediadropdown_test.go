package bar

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/mpris"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

func TestMediaPickerKeepsItsRowsWhenNothingChanged(t *testing.T) {
	source := newFakeMedia(
		mpris.Player{BusName: "org.mpris.MediaPlayer2.vlc", Identity: "VLC"},
		mpris.Player{BusName: "org.mpris.MediaPlayer2.mpv", Identity: "mpv"},
	)
	ctx := newTestContext(t, config.Defaults())
	ctx.Media = source
	v := mediaDropdown(ctx).(*mediaView)
	defer v.dropdownClosed()
	v.showPicker()
	before := treetest.WithClass(v.sources, "media-source-option")
	v.refresh()
	if after := treetest.WithClass(v.sources, "media-source-option"); !slices.Equal(after, before) {
		t.Error("a refresh that changed no player rebuilt the picker's rows")
	}
	source.players[1].Title = "a new song"
	v.refresh()
	if after := treetest.WithClass(v.sources, "media-source-option"); !slices.Equal(after, before) {
		t.Error("a change the picker does not show rebuilt its rows")
	}
	source.active = 1
	v.refresh()
	after := treetest.WithClass(v.sources, "media-source-option")
	if len(after) != 2 || after[0] == before[0] || !widget.HasClass(after[1], "selected") {
		t.Error("a new active player kept the old rows")
	}
	source.players = append(source.players, mpris.Player{BusName: "org.mpris.MediaPlayer2.spotify", Identity: "Spotify"})
	v.refresh()
	if got := treetest.WithClass(v.sources, "media-source-option"); len(got) != 3 || got[0] == after[0] {
		t.Errorf("a new player: %d rows, rebuilt %v; want three new rows", len(got), len(got) > 0 && got[0] != after[0])
	}
}
