package config

import "testing"

func TestLevelIndexFloor(t *testing.T) {
	for _, tc := range []struct {
		percent float64
		n, want int
	}{{0, 8, 0}, {12.5, 8, 1}, {50, 8, 4}, {100, 8, 7}, {100, 3, 2}, {99.9, 3, 2}, {150, 3, 2}, {-5, 3, 0}, {0, 0, 0}} {
		if got := levelIndexFloor(tc.percent, tc.n); got != tc.want {
			t.Errorf("levelIndexFloor(%v, %d) = %d, want %d", tc.percent, tc.n, got, tc.want)
		}
	}
}

func TestLevelIndexSpan(t *testing.T) {
	for _, tc := range []struct{ percent, n, want int }{
		{0, 3, 0}, {1, 3, 0}, {15, 3, 0}, {33, 3, 0}, {34, 3, 0}, {50, 3, 1}, {67, 3, 1}, {68, 3, 2}, {100, 3, 2}, {100, 2, 1}, {5, 0, 0},
	} {
		if got := levelIndexSpan(tc.percent, tc.n); got != tc.want {
			t.Errorf("levelIndexSpan(%d, %d) = %d, want %d", tc.percent, tc.n, got, tc.want)
		}
	}
}

func TestSignalIndexBuckets(t *testing.T) {
	for _, tc := range []struct {
		strength uint8
		icons    int
		want     int
	}{{0, 3, 0}, {32, 3, 0}, {33, 3, 1}, {100, 3, 2}, {100, 4, 3}, {255, 5, 4}, {50, 0, 0}} {
		if got := SignalIndex(tc.strength, tc.icons); got != tc.want {
			t.Errorf("SignalIndex(%d, %d) = %d, want %d", tc.strength, tc.icons, got, tc.want)
		}
	}
}

// A backlight's level icons split 0-100% into equal bands, the last
// closing at 100%; none is "".
func TestBrightnessLevelIconBands(t *testing.T) {
	c := BrightnessConfig{LevelIcons: []string{"low", "mid", "high"}}
	for _, tc := range []struct {
		percent float64
		want    string
	}{{0, "low"}, {33, "low"}, {34, "mid"}, {66, "mid"}, {67, "high"}, {100, "high"}, {150, "high"}, {-5, "low"}} {
		if got := c.LevelIcon(tc.percent); got != tc.want {
			t.Errorf("%v%% = %q, want %q", tc.percent, got, tc.want)
		}
	}
	if got := (BrightnessConfig{}).LevelIcon(50); got != "" {
		t.Errorf("no icons = %q", got)
	}
}

// The bar's samples showed each module's pre-report icon beside a label
// of another state: Keep Awake's bed beside "On", the battery's alert at
// 45%, the network's weakest signal. Every sample's icon is the icon of
// the state its label reads, from the same sampled values.
func TestEverySampleIconIsItsSampledStatesIcon(t *testing.T) {
	c := Defaults()
	for m, want := range map[BarModule]string{
		ModuleAudio:       c.Audio.SpeakerIcon(sampleSpeakerPercent, false),
		ModuleBattery:     c.Battery.LevelIcon(sampleBatteryPercent),
		ModuleBrightness:  c.Brightness.LevelIcon(sampleBrightnessPercent),
		ModuleNetwork:     c.Network.WifiSignalIcon(sampleWifiStrength),
		ModuleIdleInhibit: c.IdleInhibit.IconActive,
		ModuleRecorder:    c.Recorder.IconRecording,
		ModuleBitwarden:   c.Bitwarden.IconUnlocked,
		ModuleNightlight:  c.Nightlight.IconOff,
	} {
		icon, ok := c.SampleIcon(m)
		if !ok || icon.Name != want {
			t.Errorf("%s's sample shows %q, want %q", m, icon.Name, want)
		}
		if pre, _ := c.ModuleIcon(m); m != ModuleNightlight && pre.Name == icon.Name {
			t.Errorf("%s's sample still shows its pre-report icon %q", m, pre.Name)
		}
	}
	samples := PlaceholderSamples("modules.battery.format")
	if samples["percent"] != itoa(sampleBatteryPercent) {
		t.Errorf("the battery's label samples %q, its icon %d%%", samples["percent"], sampleBatteryPercent)
	}
	if icon, ok := c.SampleIcon(ModuleCPU); !ok || icon != c.CPU.Icon() {
		t.Errorf("the CPU's sample shows %+v, want its one icon", icon)
	}
}
