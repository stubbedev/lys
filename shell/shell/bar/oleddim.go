package bar

import (
	"context"
	"log"

	"github.com/stubbedev/lys/desktop/service/upower"
)

// tellWallpaperOLED tells the wallpaper, on the loop, which outputs
// lys-wm reports as OLED.
func (s *shellStart) tellWallpaperOLED(names []string) {
	if s.wall == nil {
		return
	}
	s.application.Invoke(func() { s.wall.SetOLEDOutputs(names) })
}

// followPowerSourceForWallpaper tells the wallpaper whether the machine
// runs on its battery, now and at every change, until the returned stop.
func (s *shellStart) followPowerSourceForWallpaper(battery *upower.System) (stop func()) {
	if s.wall == nil {
		return func() {}
	}
	life, cancel := context.WithCancel(context.Background())
	ticks, stopTicks, err := battery.SubscribeOnBattery(life)
	if err != nil {
		cancel()
		log.Printf("wallpaper: power source: %v; no wallpaper dims for OLED", err)
		return func() {}
	}
	read := func() {
		reading, done := context.WithTimeout(life, upowerTimeout)
		on, err := battery.OnBattery(reading)
		done()
		if err != nil {
			log.Printf("wallpaper: power source: %v; the OLED dim keeps its last state", err)
			return
		}
		s.application.Invoke(func() { s.wall.SetOnBattery(on) })
	}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		defer stopFollowing("wallpaper: power source", stopTicks)
		read()
		for range ticks {
			read()
		}
	}()
	return func() {
		cancel()
		<-finished
	}
}
