package config

// levelIndexFloor is the level math of a battery and a backlight: the
// percentage divided evenly across n icons from the emptiest, clamped.
func levelIndexFloor(percent float64, n int) int {
	if n <= 0 {
		return 0
	}
	return max(0, min(int(percent/100*float64(n)), n-1))
}

// levelIndexSpan is the volume's level math: with n icons, 0% takes the
// first and 1..100% divide the rest evenly, so with n=3 1-33% lands on
// the first, 34-66% on the second and 67-100% on the third.
func levelIndexSpan(percent int, n int) int {
	if n <= 0 || percent <= 0 {
		return 0
	}
	step := 100.0 / float64(n)
	return min(int((float64(percent)-1)/step), n-1)
}

// SignalIndex is which of numIcons a signal strength percentage takes,
// weakest first: strength over 100/numIcons, clamped.
func SignalIndex(strength uint8, numIcons int) int {
	if numIcons == 0 {
		return 0
	}
	bucket := max(100/numIcons, 1)
	return min(min(int(strength), 100)/bucket, numIcons-1)
}

// LevelIcon is the battery's icon discharging at percent: its level
// list bucketed over the percentage, the alert icon without one.
func (c BatteryConfig) LevelIcon(percent float64) string {
	if len(c.LevelIcons) == 0 {
		return c.AlertIcon
	}
	return c.LevelIcons[levelIndexFloor(percent, len(c.LevelIcons))]
}

// LevelIcon is the backlight's icon at percent, "" without a level list.
func (c BrightnessConfig) LevelIcon(percent float64) string {
	if len(c.LevelIcons) == 0 {
		return ""
	}
	return c.LevelIcons[levelIndexFloor(percent, len(c.LevelIcons))]
}

// SpeakerIcon is the speaker's icon at percent: the muted icon while
// muted or without a level list, else the level list spanned over it.
func (c AudioConfig) SpeakerIcon(percent int, muted bool) string {
	if muted || len(c.SpeakerIcons) == 0 {
		return c.SpeakerIconMuted
	}
	return c.SpeakerIcons[levelIndexSpan(percent, len(c.SpeakerIcons))]
}

// WifiSignalIcon is a connected wifi's icon at strength: the signal list
// bucketed over it, the connected icon without one.
func (c NetworkConfig) WifiSignalIcon(strength uint8) string {
	if len(c.WifiSignalIcons) == 0 {
		return c.WifiConnectedIcon
	}
	return c.WifiSignalIcons[SignalIndex(strength, len(c.WifiSignalIcons))]
}
