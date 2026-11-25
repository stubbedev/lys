package config

import "strconv"

// The states the bar's samples show, which the format samples
// (formatFields) and the samples' icons (SampleIcon) both read: a
// speaker, a battery discharging and a backlight at these levels, wifi
// connected at this strength, idle held off, the recorder recording, the
// vault open and the night light's filter off.
const (
	sampleSpeakerPercent    = 45
	sampleBatteryPercent    = 45
	sampleBrightnessPercent = 65
	sampleWifiStrength      = 75
	sampleVaultItems        = 142
)

// SampleIcon is the icon the module m shows in the state its sample
// reads (the sample* states): the level, signal or state icon of that
// state where the module's icon follows one, its icon otherwise; ok is
// false where ModuleIcon's is.
func (c *Config) SampleIcon(m BarModule) (IconConfig, bool) {
	switch m {
	case ModuleAudio:
		return IconWith(c.Audio.IconShow, c.Audio.SpeakerIcon(sampleSpeakerPercent, false), c.Audio.IconColor), true
	case ModuleBattery:
		return IconWith(c.Battery.IconShow, c.Battery.LevelIcon(sampleBatteryPercent), c.Battery.IconColor), true
	case ModuleBrightness:
		return IconWith(c.Brightness.IconShow, c.Brightness.LevelIcon(sampleBrightnessPercent), c.Brightness.IconColor), true
	case ModuleNetwork:
		return IconWith(c.Network.IconShow, c.Network.WifiSignalIcon(sampleWifiStrength), c.Network.IconColor), true
	case ModuleIdleInhibit:
		return c.IdleInhibit.Icons()[IdleInhibitActive], true
	case ModuleRecorder:
		return c.Recorder.Icons()[RecorderRecording], true
	case ModuleBitwarden:
		return c.Bitwarden.Icons()[BitwardenUnlocked], true
	case ModuleNightlight:
		return c.Nightlight.IconOffView(), true
	}
	return c.ModuleIcon(m)
}

func itoa(n int) string { return strconv.Itoa(n) }
