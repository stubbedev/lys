package settings

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stubbedev/lys/lib/wmipc"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmstate"
)

func outputField(key string) cardField { return cardField{label: wmKey(outputsPath + "." + key)} }

func TestAnOutputRowShowsWhenTheDisplayCanUseItOrItsRuleSetsIt(t *testing.T) {
	none := wmipc.OutputCapabilities{MaxBitDepth: wmipc.Depth8}
	all := wmipc.OutputCapabilities{HDR: true, VRR: true, MaxBitDepth: wmipc.Depth10}
	for _, c := range []struct {
		key  string
		caps wmipc.OutputCapabilities
		rule map[string]any
		want bool
	}{
		{"hdr", all, nil, true},
		{"hdr", none, nil, false},
		{"hdr", none, map[string]any{"hdr": true}, true},
		{"sdr_brightness", none, nil, false},
		{"hdr_peak", all, nil, true},
		{"vrr", none, nil, false},
		{"vrr", all, nil, true},
		{"vrr", none, map[string]any{"vrr": "on"}, true},
		{"bitdepth", none, nil, false},
		{"bitdepth", all, nil, true},
		{"bitdepth", none, map[string]any{"bitdepth": 10}, true},
		{"shader", none, nil, true},
		{"icc", none, nil, true},
	} {
		if got := outputCanUse(outputField(c.key), c.caps, c.rule); got != c.want {
			t.Errorf("%s on %+v with rule %v: shown %t, want %t", c.key, c.caps, c.rule, got, c.want)
		}
	}
}

func TestTheDisplayNoteSaysWhyEveryOptionShows(t *testing.T) {
	tr := locale.T()
	known := map[string]wmipc.OutputCapabilities{"DP-1": {MaxBitDepth: wmipc.Depth8}}
	broken := errors.New("lys-wmctl: broken pipe")
	for _, c := range []struct {
		name    string
		caps    outputCapabilities
		output  string
		hidden  bool
		want    string
		wantErr error
	}{
		{"not read yet", outputCapabilities{}, "DP-1", false, "", nil},
		{"no lys-wm", outputCapabilities{read: true, err: wmstate.ErrNotRunning}, "DP-1", false, tr.Get("settings-displays-caps-not-running"), nil},
		{"an older lys-wm", outputCapabilities{read: true, err: fmt.Errorf("%w: output DP-1", wmstate.ErrOutdated)}, "DP-1", false, tr.Get("settings-displays-caps-outdated"), nil},
		{"a display that is off", outputCapabilities{read: true, byName: known}, "HDMI-A-1", false, tr.Get("settings-displays-caps-off"), nil},
		{"a display with hidden options", outputCapabilities{read: true, byName: known}, "DP-1", true, tr.Get("settings-displays-caps-hidden"), nil},
		{"a display that can use every option", outputCapabilities{read: true, byName: known}, "DP-1", false, "", nil},
		{"capabilities that cannot be read", outputCapabilities{read: true, err: broken}, "DP-1", false, tr.Get("settings-displays-caps-unreadable"), broken},
	} {
		if got, err := c.caps.note(c.output, c.hidden); got != c.want || !errors.Is(err, c.wantErr) {
			t.Errorf("%s: %q, %v, want %q, %v", c.name, got, err, c.want, c.wantErr)
		}
	}
	if _, ok := (outputCapabilities{read: true, byName: known}).of("DP-1"); !ok {
		t.Error("a shown display's capabilities are not known")
	}
	if _, ok := (outputCapabilities{read: true, byName: known, err: errors.New("lys-wmctl: broken")}).of("DP-1"); ok {
		t.Error("capabilities read with an error are known")
	}
}
