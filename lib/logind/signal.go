package logind

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// Event is one decoded logind signal about a watched session: Sleep,
// Locked, Unlocked, DevicePaused, DeviceResumed, ActiveChanged or
// LockedHintChanged.
type Event interface{ event() }

// Sleep is PrepareForSleep's argument: Start is true as the system is
// about to sleep, false once it has woken.
type Sleep struct{ Start bool }

// Locked is the session's Lock signal: its lock screen is to lock it.
type Locked struct{}

// Unlocked is the session's Unlock signal: its lock screen is to unlock
// it.
type Unlocked struct{}

// PauseKind is how a PauseDevice pauses its device.
type PauseKind string

// The PauseDevice kinds: PausePause waits for PauseDeviceComplete,
// PauseForce has paused the device already, PauseGone says it was
// removed.
const (
	PausePause PauseKind = "pause"
	PauseForce PauseKind = "force"
	PauseGone  PauseKind = "gone"
)

// DevicePaused is the session's PauseDevice signal.
type DevicePaused struct {
	Major, Minor uint32
	Kind         PauseKind
}

// Device is the paused device.
func (p DevicePaused) Device() Device { return Device{Major: p.Major, Minor: p.Minor} }

// DeviceResumed is the session's ResumeDevice signal. FD is a new
// descriptor for the device, the handler's to close.
type DeviceResumed struct {
	Major, Minor uint32
	FD           dbus.UnixFD
}

// Device is the resumed device.
func (r DeviceResumed) Device() Device { return Device{Major: r.Major, Minor: r.Minor} }

// ActiveChanged is the session's Active property changing.
type ActiveChanged struct{ Active bool }

// LockedHintChanged is the session's LockedHint property changing.
type LockedHintChanged struct{ Locked bool }

func (Sleep) event()             {}
func (Locked) event()            {}
func (Unlocked) event()          {}
func (DevicePaused) event()      {}
func (DeviceResumed) event()     {}
func (ActiveChanged) event()     {}
func (LockedHintChanged) event() {}

// The logind signals lys reads, each declared once: PrepareForSleep
// from the manager, the rest from a session.
var (
	PrepareForSleep = dbusx.NewSignal[Sleep](ManagerInterface, "PrepareForSleep")
	Lock            = dbusx.NewSignal[Locked](SessionInterface, "Lock")
	Unlock          = dbusx.NewSignal[Unlocked](SessionInterface, "Unlock")
	PauseDevice     = dbusx.NewSignal[DevicePaused](SessionInterface, "PauseDevice")
	ResumeDevice    = dbusx.NewSignal[DeviceResumed](SessionInterface, "ResumeDevice")
)

// PrepareForSleepMatch matches PrepareForSleep from logind's manager.
func PrepareForSleepMatch() []dbus.MatchOption {
	return []dbus.MatchOption{dbus.WithMatchSender(BusName), dbus.WithMatchObjectPath(ManagerPath), dbus.WithMatchInterface(ManagerInterface), dbus.WithMatchMember("PrepareForSleep")}
}

// routes are the signals a watch of the session reads, each decoded to
// the Event emit gets: the session's own, its Active and LockedHint
// changing, and the manager's PrepareForSleep.
func (s Session) routes(emit func(Event)) []dbusx.Route {
	path := s.obj.path
	return []dbusx.Route{
		dbusx.On(Lock, path, emitAs[Locked](emit)),
		dbusx.On(Unlock, path, emitAs[Unlocked](emit)),
		dbusx.On(PauseDevice, path, emitAs[DevicePaused](emit)),
		dbusx.On(ResumeDevice, path, emitAs[DeviceResumed](emit)),
		s.changes(emit),
		dbusx.On(PrepareForSleep, ManagerPath, emitAs[Sleep](emit)),
	}
}

// changes is the session's Active and LockedHint changing, each decoded
// to the Event emit gets.
func (s Session) changes(emit func(Event)) dbusx.Route {
	path := s.obj.path
	return dbusx.On(dbusx.PropertiesChanged, path, func(c dbusx.ChangedProperties, _ dbus.Sequence) error {
		return emitChanged(path, c.Changed, emit)
	}).WithArg0(SessionInterface)
}

func emitAs[T Event](emit func(Event)) func(T, dbus.Sequence) error {
	return func(v T, _ dbus.Sequence) error {
		emit(v)
		return nil
	}
}

func emitChanged(path dbus.ObjectPath, changed map[string]dbus.Variant, emit func(Event)) error {
	var errs []error
	if v, ok := changed["Active"]; ok {
		active, err := changedBool(path, "Active", v)
		if err == nil {
			emit(ActiveChanged{Active: active})
		}
		errs = append(errs, err)
	}
	if v, ok := changed["LockedHint"]; ok {
		locked, err := changedBool(path, "LockedHint", v)
		if err == nil {
			emit(LockedHintChanged{Locked: locked})
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func changedBool(path dbus.ObjectPath, name string, v dbus.Variant) (bool, error) {
	b, ok := v.Value().(bool)
	if !ok {
		return false, fmt.Errorf("%s of %s changed to a %s, want b", name, path, v.Signature())
	}
	return b, nil
}

// ErrDisconnected is what Watch.Stop reports when the watch ended
// because its connection closed.
var ErrDisconnected = dbusx.ErrDisconnected

// Watch follows one logind object's signals until its context ends, it
// is stopped, or its connection closes.
type Watch struct {
	signals *dbusx.Watch
	object  dbus.ObjectPath
}

// Watch runs handle, on the watch's goroutine and in arrival order, for
// every signal logind sends about the session (Lock, Unlock,
// PauseDevice, ResumeDevice, its Active and LockedHint changing) and
// for the manager's PrepareForSleep. Exactly one of handle's arguments
// is non-nil: the decoded Event, or why a signal from logind could not
// be decoded. Only signals from whoever owns logind's name as they
// arrive count, so the watch outlives a restart of logind; a
// ResumeDevice from anyone else has its descriptor closed. A
// DeviceResumed's descriptor is this watch's own: every watch of the
// session, on this connection or another, is handed one of its own.
// The watch holds match rules and a signal channel on the session's
// connection until it ends, and then removes them, leaving a shared
// connection as it was.
func (s Session) Watch(ctx context.Context, handle func(Event, error)) (*Watch, error) {
	return s.watch(ctx, func(err error) { handle(nil, err) }, s.routes(func(e Event) { handle(e, nil) }))
}

func (s Session) watch(ctx context.Context, failed func(error), routes []dbusx.Route) (*Watch, error) {
	if s.obj.conn == nil {
		return nil, fmt.Errorf("logind: watch a session: %w", errZeroHandle)
	}
	signals, err := dbusx.WatchSignals(ctx, s.obj.conn, dbusx.FromOwner(BusName),
		func(err error) { failed(fmt.Errorf("logind: %w", err)) },
		routes...)
	if err != nil {
		return nil, fmt.Errorf("logind: watch session %s: %w", s.obj.path, err)
	}
	return &Watch{signals: signals, object: s.obj.path}, nil
}

// FollowLid reads whether the lid is closed and then runs onChange, on
// the watch's goroutine and in arrival order, with every LidClosed logind
// announces. The read follows setting the watch up, so no change falls
// between the two. failed gets an announcement that could not be decoded
// and the watch's own failure.
func (m Manager) FollowLid(ctx context.Context, failed func(error), onChange func(closed bool)) (closed bool, w *Watch, err error) {
	if m.obj.conn == nil {
		return false, nil, fmt.Errorf("logind: follow the lid: %w", errZeroHandle)
	}
	var follow lidFollow
	signals, err := dbusx.WatchSignals(ctx, m.obj.conn, dbusx.FromOwner(BusName),
		func(err error) { failed(fmt.Errorf("logind: %w", err)) },
		dbusx.On(dbusx.PropertiesChanged, ManagerPath, func(c dbusx.ChangedProperties, seq dbus.Sequence) error {
			v, ok := c.Changed["LidClosed"]
			if !ok {
				return nil
			}
			closed, err := changedBool(ManagerPath, "LidClosed", v)
			if err != nil {
				return err
			}
			follow.changed(lidChange{closed: closed, seq: seq}, onChange)
			return nil
		}).WithArg0(ManagerInterface))
	if err != nil {
		return false, nil, fmt.Errorf("logind: follow the lid: %w", err)
	}
	w = &Watch{signals: signals, object: ManagerPath}
	answered, err := m.obj.propertyAt(ctx, ManagerInterface, "LidClosed", &closed)
	if err != nil {
		return false, nil, errors.Join(fmt.Errorf("logind: follow the lid: %w", err), w.Stop())
	}
	follow.answer(answered, onChange)
	return closed, w, nil
}

type lidChange struct {
	closed bool
	seq    dbus.Sequence
}

// lidFollow holds the lid's changes back until its first read has been
// answered, then hands on only those logind sent after that answer: a
// change sent before it is already in what the read returned.
type lidFollow struct {
	mu       sync.Mutex
	answered dbus.Sequence
	answerIn bool
	pending  []lidChange
}

func (f *lidFollow) changed(c lidChange, onChange func(bool)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.answerIn {
		f.pending = append(f.pending, c)
		return
	}
	if c.seq > f.answered {
		onChange(c.closed)
	}
}

func (f *lidFollow) answer(answered dbus.Sequence, onChange func(bool)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answerIn, f.answered = true, answered
	for _, c := range f.pending {
		if c.seq > answered {
			onChange(c.closed)
		}
	}
	f.pending = nil
}

// Done is closed once the watch has ended and removed what it held.
func (w *Watch) Done() <-chan struct{} { return w.signals.Done() }

// Stop ends the watch and returns once it has removed what it held:
// with ErrDisconnected when its connection had closed, with the error
// removing its match rules met, or nil. Calling it again waits for the
// same end and reports the same error. Stop must not be called from the
// watch's handler.
func (w *Watch) Stop() error {
	if err := w.signals.Stop(); err != nil {
		return fmt.Errorf("logind: %s: %w", w.object, err)
	}
	return nil
}

// WaitLocked returns once the session's LockedHint is true, with ctx's
// error when it ends first, or with the failure to read a change of the
// session's properties that logind announced meanwhile: the hint may
// have been in it.
func (s Session) WaitLocked(ctx context.Context) error {
	return s.waitTrue(ctx, "LockedHint", s.LockedHint, func(e Event) bool {
		c, ok := e.(LockedHintChanged)
		return ok && c.Locked
	})
}

// WaitActive returns once the session is in the foreground of its seat,
// its Active true, as WaitLocked does for LockedHint: logind has moved
// the seat's devices to the session by then.
func (s Session) WaitActive(ctx context.Context) error {
	return s.waitTrue(ctx, "Active", s.Active, func(e Event) bool {
		c, ok := e.(ActiveChanged)
		return ok && c.Active
	})
}

// waitTrue returns once the session's boolean property name is true:
// read reads it, after the watch for its change is up so that no change
// falls between the two, and turned says an Event is it turning true.
func (s Session) waitTrue(ctx context.Context, name string, read func(context.Context) (bool, error), turned func(Event) bool) (err error) {
	became := make(chan struct{}, 1)
	unread := make(chan error, 1)
	report := func(err error) {
		select {
		case unread <- err:
		default:
		}
	}
	w, err := s.watch(ctx, report, []dbusx.Route{s.changes(func(e Event) {
		if turned(e) {
			select {
			case became <- struct{}{}:
			default:
			}
		}
	})})
	if err != nil {
		return err
	}
	defer func() {
		if stopErr := w.Stop(); stopErr != nil && !errors.Is(err, ErrDisconnected) {
			err = errors.Join(err, stopErr)
		}
	}()
	if now, err := read(ctx); err != nil || now {
		return err
	}
	select {
	case <-became:
		return nil
	case err := <-unread:
		return fmt.Errorf("logind: wait for %s of %s: %w", name, s.obj.path, err)
	case <-w.Done():
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("logind: wait for %s of %s: %w", name, s.obj.path, err)
		}
		return fmt.Errorf("logind: wait for %s of %s: %w", name, s.obj.path, ErrDisconnected)
	}
}
