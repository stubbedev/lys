package desktopnotify

import (
	"context"
	"fmt"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/intconv"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/shell/service/notifications"
)

// Action is one notification button: Key is what Ask reports, Label
// what the daemon shows.
type Action struct {
	Key   string
	Label string
}

// outcome is what the daemon said of one notification: the key of the
// button pressed, or that it closed.
type outcome struct {
	id      uint32
	key     string
	pressed bool
	said    dbus.Sequence
}

// question is a notification Ask posted, whose daemon has answered
// Notify: what the daemon says after that reply is about it.
type question struct {
	id           uint32
	posted       dbus.Sequence
	outcomes     <-chan outcome
	unreadable   <-chan error
	watch        *dbusx.Watch
	stopWatching context.CancelFunc
}

// Ask posts a notification with buttons and waits for the user:
// the key of
// the button pressed, or ok=false when the notification closed or
// expired without one, or ctx ended first. timeout is the daemon's
// expire timeout. The icon also goes in the image-path hint, as
// libnotify does, which lys-shell's own popup reads. Only what the
// daemon says after it answered Notify is about this notification: an
// id it used before may be the one it hands out again.
func (s *Sender) Ask(ctx context.Context, appName, summary, body, appIcon string, actions []Action, timeout time.Duration) (key string, ok bool, err error) {
	q, err := s.ask(ctx, appName, summary, body, appIcon, actions, timeout)
	if err != nil {
		return "", false, err
	}
	return q.answer(ctx)
}

// AskOnce is Ask on a session bus connection of its own, for a caller
// that keeps none: it waits up to timeout for the user, plus the time
// the daemon takes to answer.
func AskOnce(appName, summary, body, appIcon string, actions []Action, timeout time.Duration) (key string, ok bool, err error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return "", false, fmt.Errorf("desktopnotify: session bus: %w", err)
	}
	defer func() { _ = conn.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), timeout+sendTimeout)
	defer cancel()
	return NewSender(conn).Ask(ctx, appName, summary, body, appIcon, actions, timeout)
}

// ask posts the notification and returns once the daemon answered
// Notify; answer, which must follow, waits for the user.
func (s *Sender) ask(ctx context.Context, appName, summary, body, appIcon string, actions []Action, timeout time.Duration) (*question, error) {
	watching, stopWatching := context.WithCancel(ctx)
	outcomes, unreadable := make(chan outcome, 16), make(chan error, 1)
	tell := func(o outcome) {
		select {
		case outcomes <- o:
		case <-watching.Done():
		}
	}
	// Watching before sending, so the answer cannot race the id.
	watch, err := dbusx.WatchSignals(watching, s.conn, dbusx.FromOwner(busName),
		func(err error) {
			select {
			case unreadable <- err:
			default:
			}
		},
		dbusx.On(notifications.ActionInvoked, path, func(invoked notifications.ActionInvocation, said dbus.Sequence) error {
			tell(outcome{id: invoked.ID, key: invoked.Key, pressed: true, said: said})
			return nil
		}),
		dbusx.On(notifications.NotificationClosed, path, func(closure notifications.Closure, said dbus.Sequence) error {
			tell(outcome{id: closure.ID, said: said})
			return nil
		}))
	if err != nil {
		stopWatching()
		return nil, fmt.Errorf("desktopnotify: %w", err)
	}
	q := &question{outcomes: outcomes, unreadable: unreadable, watch: watch, stopWatching: stopWatching}

	flat := make([]string, 0, 2*len(actions))
	for _, a := range actions {
		flat = append(flat, a.Key, a.Label)
	}
	call := s.conn.Object(busName, path).CallWithContext(ctx, method, 0,
		appName, uint32(0), appIcon, summary, body, flat, iconHints(appIcon), intconv.Clamp[int32](timeout.Milliseconds()))
	if call.Err != nil {
		q.stop()
		return nil, fmt.Errorf("desktopnotify: Notify: %w", call.Err)
	}
	if err := call.Store(&q.id); err != nil {
		q.stop()
		return nil, fmt.Errorf("desktopnotify: Notify reply: %w", err)
	}
	q.posted = call.ResponseSequence
	return q, nil
}

// answer waits for the press or the close the daemon announces after
// its Notify reply, then stops watching.
func (q *question) answer(ctx context.Context) (key string, ok bool, err error) {
	defer q.stop()
	for {
		select {
		case <-ctx.Done():
			return "", false, nil
		case <-q.watch.Done():
			return "", false, nil
		case err := <-q.unreadable:
			return "", false, fmt.Errorf("desktopnotify: %w", err)
		case o := <-q.outcomes:
			if o.id != q.id || o.said < q.posted {
				continue
			}
			return o.key, o.pressed, nil
		}
	}
}

func (q *question) stop() {
	q.stopWatching()
	_ = q.watch.Stop()
}
