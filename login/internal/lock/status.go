package lock

import (
	"math"
	"net/url"

	"github.com/stubbedev/lys/desktop/service/mpris"
	"github.com/stubbedev/lys/desktop/service/powersupply"
	"github.com/stubbedev/lys/desktop/shell/lockview"
)

type nowPlaying struct {
	bus string
	lockview.Track
}

func nowPlayingOf(p mpris.Player, ok bool) (nowPlaying, bool) {
	if !ok || p.Title == "" {
		return nowPlaying{}, false
	}
	np := nowPlaying{bus: p.BusName, Title: p.Title, Artist: p.Artist, Playing: p.State == mpris.StatePlaying, CanNext: p.CanGoNext}
	if u, err := url.Parse(p.ArtURL); err == nil && u.Scheme == "file" {
		np.ArtPath = u.Path
	}
	return np, true
}

func batteryLevelOf(d powersupply.Device) (lockview.BatteryLevel, bool) {
	if !d.IsPresent {
		return lockview.BatteryLevel{}, false
	}
	charging := d.State == powersupply.StateCharging || d.State == powersupply.StatePendingCharge
	return lockview.BatteryLevel{Percent: int(math.Round(d.Percentage)), Charging: charging}, true
}
