package autobrightness

import (
	"context"
	"fmt"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// RoomProperty is how lit the room is, as automatic brightness last
// estimated it.
const RoomProperty = "Room"

// Room is how lit the room is: unknown before the first estimate,
// bright where a keyboard backlight helps nobody, dim otherwise.
type Room uint8

// The rooms.
const (
	RoomUnknown Room = iota + 1
	RoomDim
	RoomBright
)

func (r Room) String() string {
	switch r {
	case RoomUnknown:
		return "unknown"
	case RoomDim:
		return "dim"
	case RoomBright:
		return "bright"
	}
	return fmt.Sprintf("Room(%d)", uint8(r))
}

// ParseRoom reads a room as the bus spells it.
func ParseRoom(wire string) (Room, error) {
	switch wire {
	case "unknown":
		return RoomUnknown, nil
	case "dim":
		return RoomDim, nil
	case "bright":
		return RoomBright, nil
	}
	return 0, fmt.Errorf("auto-brightness room %q: want unknown, dim or bright", wire)
}

// PublishRoom announces how lit the room is now.
func (s *Server) PublishRoom(room Room) error {
	if _, err := ParseRoom(room.String()); err != nil {
		return fmt.Errorf("publish %s: %w", RoomProperty, err)
	}
	s.mu.Lock()
	same := s.room == room
	s.room = room
	s.mu.Unlock()
	if same {
		return nil
	}
	if err := s.conn.Emit(ServicePath, propertiesChanged, Interface, map[string]dbus.Variant{RoomProperty: dbus.MakeVariant(room.String())}, []string{}); err != nil {
		return fmt.Errorf("emit %s change to %s: %w", RoomProperty, room, err)
	}
	return nil
}

func (s *Server) roomSnapshot() Room {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.room
}

// Room reads how lit the room is.
func (c *Client) Room(ctx context.Context) (Room, error) {
	var wire string
	if err := c.obj.CallWithContext(ctx, propertiesIface+".Get", 0, Interface, RoomProperty).Store(&wire); err != nil {
		return 0, fmt.Errorf("read %s from %s: %w", RoomProperty, ServiceName, err)
	}
	room, err := ParseRoom(wire)
	if err != nil {
		return 0, fmt.Errorf("read %s from %s: %w", RoomProperty, ServiceName, err)
	}
	return room, nil
}

// RoomChanged is the room a PropertiesChanged of the service announces,
// false when it announces none.
func RoomChanged(sig *dbus.Signal) (Room, bool, error) {
	wire, ok, err := changedString(sig, RoomProperty)
	if err != nil || !ok {
		return 0, false, err
	}
	room, err := ParseRoom(wire)
	if err != nil {
		return 0, false, fmt.Errorf("%s from %s: %w", propertiesChanged, sig.Sender, err)
	}
	return room, true, nil
}
