package bar

import (
	"context"
	"sync"
	"testing"

	"github.com/stubbedev/lys/desktop/feed"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/power/autobrightness"

	"github.com/stubbedev/lys/shell/service/kbdlight"
)

type fakeRooms struct {
	mu      sync.Mutex
	room    autobrightness.Room
	changes feed.Feed[struct{}]
}

func (f *fakeRooms) Room() autobrightness.Room {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.room
}

func (f *fakeRooms) Subscribe() (<-chan struct{}, func()) { return f.changes.Subscribe() }

func (f *fakeRooms) set(room autobrightness.Room) {
	f.mu.Lock()
	f.room = room
	f.mu.Unlock()
	f.changes.Publish(struct{}{})
}

type keyboardLED struct {
	mu  sync.Mutex
	raw uint32
}

func (k *keyboardLED) Levels(context.Context) ([]kbdlight.Level, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return []kbdlight.Level{{Name: "tpacpi::kbd_backlight", Raw: k.raw}}, nil
}

func (k *keyboardLED) Set(_ context.Context, _ string, raw uint32) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.raw = raw
	return nil
}

func (k *keyboardLED) level() uint32 {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.raw
}

func TestTheKeyboardBacklightIsOffInABrightRoomOnly(t *testing.T) {
	rooms := &fakeRooms{room: autobrightness.RoomBright}
	led := &keyboardLED{raw: 2}
	keyboard := kbdlight.New(led)
	stop := followBrightRoom(rooms, keyboard)
	defer stop()
	waittest.Until(t, func() bool { return led.level() == 0 }, "the keyboard backlight off in a bright room")
	rooms.set(autobrightness.RoomDim)
	waittest.Until(t, func() bool { return led.level() == 2 }, "the keyboard backlight back on in a dim room")
	rooms.set(autobrightness.RoomBright)
	waittest.Until(t, func() bool { return led.level() == 0 }, "the keyboard backlight off once the room is bright again")
	rooms.set(autobrightness.RoomUnknown)
	waittest.Until(t, func() bool { return led.level() == 2 }, "the keyboard backlight on without an estimate")
	if err := keyboard.Hold(t.Context(), kbdlight.Inactive, true); err != nil {
		t.Fatal(err)
	}
	rooms.set(autobrightness.RoomDim)
	rooms.set(autobrightness.RoomUnknown)
	stop()
	if got := led.level(); got != 0 {
		t.Errorf("a dim room lit the keyboard backlight of an idle session at %d", got)
	}
}
