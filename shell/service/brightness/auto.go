package brightness

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/feed"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/power/autobrightness"
)

// autoTimeout bounds one call to automatic brightness.
const autoTimeout = 5 * time.Second

// ErrAutoNotRunning is what ResumeAuto wraps when automatic brightness
// is off: nothing owns its name on the session bus.
var ErrAutoNotRunning = errors.New("automatic brightness is not running: nothing owns " + autobrightness.ServiceName + " on the session bus")

// Control is who sets the backlight: the user alone while automatic
// brightness is off, or automatic brightness, following the room's
// light or paused by a manual change.
type Control uint8

// Controls.
const (
	ControlManual Control = iota
	ControlFollowing
	ControlPaused
)

func (c Control) String() string {
	switch c {
	case ControlManual:
		return "manual"
	case ControlFollowing:
		return "following"
	case ControlPaused:
		return "paused"
	}
	return fmt.Sprintf("Control(%d)", uint8(c))
}

func controlOf(state autobrightness.State) (Control, error) {
	switch state {
	case autobrightness.Following:
		return ControlFollowing, nil
	case autobrightness.Paused:
		return ControlPaused, nil
	}
	return ControlManual, fmt.Errorf("automatic brightness reports %v", state)
}

// Auto follows automatic brightness on a session bus: its State on
// each PropertiesChanged of whichever connection owns its name, read
// again each time the name gains an owner, and ControlManual while
// nobody owns it.
type Auto struct {
	conn    *dbus.Conn
	client  *autobrightness.Client
	failed  func(error)
	cancel  context.CancelFunc
	watch   *dbusx.Watch
	changes feed.Feed[struct{}]

	// serial orders the first read against the watch's handlers.
	serial sync.Mutex

	mu      sync.Mutex
	control Control
	room    autobrightness.Room
}

// FollowAuto starts following automatic brightness on conn. failed is
// told of each state that could not be read or understood, which
// leaves the control as it was.
func FollowAuto(conn *dbus.Conn, failed func(error)) (*Auto, error) {
	ctx, cancel := context.WithCancel(context.Background())
	a := &Auto{conn: conn, client: autobrightness.NewClient(conn), failed: failed, cancel: cancel, room: autobrightness.RoomUnknown}
	a.serial.Lock()
	defer a.serial.Unlock()
	watch, err := dbusx.WatchSignals(ctx, conn, dbusx.FromOwner(autobrightness.ServiceName), failed,
		dbusx.OnOthers(dbusx.PropertiesInterface, autobrightness.ServicePath, a.changed).WithArg0(autobrightness.Interface),
		dbusx.OnOwnerChange(autobrightness.ServiceName, func(_, owner string, _ dbus.Sequence) error {
			a.serial.Lock()
			defer a.serial.Unlock()
			if owner == "" {
				a.set(ControlManual, autobrightness.RoomUnknown)
				return nil
			}
			return a.read(ctx)
		}))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("follow automatic brightness: %w", err)
	}
	a.watch = watch
	if err := a.read(ctx); err != nil {
		failed(err)
	}
	return a, nil
}

func (a *Auto) changed(sig *dbus.Signal) error {
	a.serial.Lock()
	defer a.serial.Unlock()
	state, stateOK, stateErr := autobrightness.Changed(sig)
	room, roomOK, roomErr := autobrightness.RoomChanged(sig)
	if err := errors.Join(stateErr, roomErr, sig.CloseFDs()); err != nil {
		return err
	}
	if roomOK {
		a.setRoom(room)
	}
	if !stateOK {
		return nil
	}
	return a.apply(state)
}

// read asks automatic brightness for its state and the room:
// ControlManual in an unknown room when nobody owns its name.
func (a *Auto) read(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, autoTimeout)
	defer cancel()
	state, err := a.client.State(ctx)
	if dbusx.Unowned(err) {
		a.set(ControlManual, autobrightness.RoomUnknown)
		return nil
	}
	if err != nil {
		return err
	}
	room, err := a.client.Room(ctx)
	if err != nil {
		return err
	}
	a.setRoom(room)
	return a.apply(state)
}

func (a *Auto) apply(state autobrightness.State) error {
	control, err := controlOf(state)
	if err != nil {
		return err
	}
	a.mu.Lock()
	room := a.room
	a.mu.Unlock()
	a.set(control, room)
	return nil
}

func (a *Auto) setRoom(room autobrightness.Room) {
	a.mu.Lock()
	control := a.control
	a.mu.Unlock()
	a.set(control, room)
}

func (a *Auto) set(control Control, room autobrightness.Room) {
	a.mu.Lock()
	same := a.control == control && a.room == room
	a.control, a.room = control, room
	a.mu.Unlock()
	if !same {
		a.changes.Publish(struct{}{})
	}
}

// Room is how lit automatic brightness last found the room: unknown
// while it is off, paused or has no estimate yet.
func (a *Auto) Room() autobrightness.Room {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.room
}

// Control is who sets the backlight now.
func (a *Auto) Control() Control {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.control
}

// Subscribe ticks whenever the control or the room changed.
func (a *Auto) Subscribe() (<-chan struct{}, func()) { return a.changes.Subscribe() }

// Resume hands the backlight back to automatic brightness.
func (a *Auto) Resume(ctx context.Context) error { return ResumeAuto(ctx, a.conn) }

// Close stops following, once the watch has taken its match rules off
// the bus.
func (a *Auto) Close() error {
	a.cancel()
	defer a.changes.Close()
	if err := a.watch.Stop(); err != nil && !errors.Is(err, dbusx.ErrDisconnected) {
		return fmt.Errorf("stop following automatic brightness: %w", err)
	}
	return nil
}

// ResumeAuto hands the backlight back to automatic brightness on conn,
// which then follows the room's light again; ErrAutoNotRunning when it
// is off.
func ResumeAuto(ctx context.Context, conn *dbus.Conn) error {
	ctx, cancel := context.WithTimeout(ctx, autoTimeout)
	defer cancel()
	err := autobrightness.NewClient(conn).Resume(ctx)
	if dbusx.Unowned(err) {
		return fmt.Errorf("%w: %w", ErrAutoNotRunning, err)
	}
	return err
}
