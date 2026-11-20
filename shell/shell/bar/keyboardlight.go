package bar

import (
	"context"
	"log"
	"time"

	"github.com/stubbedev/lys/power/autobrightness"

	"github.com/stubbedev/lys/shell/service/kbdlight"
)

// keyboardLightTimeout bounds turning the keyboard backlights off or on.
const keyboardLightTimeout = 5 * time.Second

// roomSource is how lit automatic brightness finds the room
// (*brightness.Auto).
type roomSource interface {
	Room() autobrightness.Room
	Subscribe() (<-chan struct{}, func())
}

// followBrightRoom holds the keyboard backlights off while automatic
// brightness finds the room bright, until the returned stop has returned.
func followBrightRoom(rooms roomSource, keyboard *kbdlight.Light) (stop func()) {
	ticks, unsubscribe := rooms.Subscribe()
	done := make(chan struct{})
	hold := func() {
		ctx, cancel := context.WithTimeout(context.Background(), keyboardLightTimeout)
		defer cancel()
		if err := keyboard.Hold(ctx, kbdlight.Bright, rooms.Room() == autobrightness.RoomBright); err != nil {
			log.Printf("keyboard backlight: %v", err)
		}
	}
	go func() {
		defer close(done)
		hold()
		for range ticks {
			hold()
		}
	}()
	return func() {
		unsubscribe()
		<-done
	}
}
