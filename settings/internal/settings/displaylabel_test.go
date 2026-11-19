package settings

import (
	"testing"

	"github.com/stubbedev/lys/settings/internal/displays"
	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestADisplayIsNamedByItsModelThenItsDescriptionThenItsConnector(t *testing.T) {
	for _, c := range []struct {
		head displays.Head
		want string
	}{
		{displays.Head{Name: "DP-1", Make: "Dell Inc.", Model: "DELL U2720Q", Description: "Dell Inc. DELL U2720Q (DP-1)"}, "Dell Inc. DELL U2720Q"},
		{displays.Head{Name: "HDMI-A-1", Description: "Some TV"}, "Some TV"},
		{displays.Head{Name: "HDMI-A-2"}, "HDMI-A-2"},
		{displays.Head{Name: "eDP-1", Make: "BOE", Model: "0x0BCA"}, locale.T().Get("settings-displays-built-in")},
	} {
		if got := headLabel(c.head); got != c.want {
			t.Errorf("%+v reads %q, want %q", c.head, got, c.want)
		}
	}
	tiles := monitorTiles([]displays.Head{{Name: "DP-1", Make: "Dell Inc.", Model: "U2720Q", Enabled: true, Modes: []displays.Mode{{Width: 1920, Height: 1080}}, Scale: 1}})
	if len(tiles) != 1 || tiles[0].label != "Dell Inc. U2720Q" || tiles[0].name != "DP-1" {
		t.Errorf("tiles %+v", tiles)
	}
}
