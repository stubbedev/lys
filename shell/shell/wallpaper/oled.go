package wallpaper

import "slices"

// oledDim is what decides whether an output's wallpaper is dimmed: the
// wallpaper.oled-dim option, the power source, and the outputs lys-wm
// reports as OLED.
type oledDim struct {
	option    bool
	onBattery bool
	oled      []string
}

// dims reports whether the wallpaper on the named output is dimmed:
// only with the option on, on battery, on an OLED output.
func (d oledDim) dims(output string) bool {
	return d.option && d.onBattery && slices.Contains(d.oled, output)
}

// SetOnBattery dims or restores the wallpapers of OLED outputs as the
// machine goes on or off its battery. Call it on the loop goroutine.
func (s *Shell) SetOnBattery(on bool) {
	s.dim.onBattery = on
	s.applyDim()
}

// SetOLEDOutputs names the outputs lys-wm reports as OLED. Call it on
// the loop goroutine.
func (s *Shell) SetOLEDOutputs(names []string) {
	s.dim.oled = slices.Clone(names)
	s.applyDim()
}

func (s *Shell) applyDim() {
	for name, surf := range s.surfaces {
		surf.view.setDim(s.dim.dims(name))
	}
}
