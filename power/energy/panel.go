package energy

import (
	"fmt"
	"maps"
	"slices"
	"time"
)

const PanelPoll = 250 * time.Millisecond

type SelfRefresh uint8

const (
	SelfRefreshUnsupported SelfRefresh = iota + 1
	SelfRefreshOff
	SelfRefreshPSR1
	SelfRefreshPSR2
	SelfRefreshPanelReplay
	SelfRefreshPanelReplaySU
)

func (s SelfRefresh) String() string {
	switch s {
	case SelfRefreshUnsupported:
		return "unsupported"
	case SelfRefreshOff:
		return "off"
	case SelfRefreshPSR1:
		return "psr1"
	case SelfRefreshPSR2:
		return "psr2"
	case SelfRefreshPanelReplay:
		return "panel-replay"
	case SelfRefreshPanelReplaySU:
		return "panel-replay-su"
	}
	return fmt.Sprintf("SelfRefresh(%d)", uint8(s))
}

func (s SelfRefresh) MarshalText() ([]byte, error) {
	if _, err := ParseSelfRefresh(s.String()); err != nil {
		return nil, err
	}
	return []byte(s.String()), nil
}

func ParseSelfRefresh(wire string) (SelfRefresh, error) {
	for s := SelfRefreshUnsupported; s <= SelfRefreshPanelReplaySU; s++ {
		if s.String() == wire {
			return s, nil
		}
	}
	return 0, fmt.Errorf("panel self refresh %q: want unsupported, off, psr1, psr2, panel-replay or panel-replay-su", wire)
}

type PanelState struct {
	Device     string
	Mode       SelfRefresh
	Refreshing bool
}

type Panel struct {
	Device    string
	Mode      SelfRefresh
	Readings  uint32
	Residency float64
}

func panels(intervals []Interval) []Panel {
	type tally struct {
		mode                SelfRefresh
		readings, refreshed uint32
	}
	tallies := map[string]*tally{}
	for _, iv := range intervals {
		for _, p := range iv.Panels {
			t, ok := tallies[p.Device]
			if !ok {
				t = &tally{}
				tallies[p.Device] = t
			}
			t.mode = p.Mode
			t.readings++
			if p.Refreshing {
				t.refreshed++
			}
		}
	}
	var out []Panel
	for _, device := range slices.Sorted(maps.Keys(tallies)) {
		t := tallies[device]
		out = append(out, Panel{Device: device, Mode: t.mode, Readings: t.readings, Residency: float64(t.refreshed) / float64(t.readings)})
	}
	return out
}
