// Package powerprofile follows the power profile lys-power publishes on
// the system bus, so lys-wm can save power while the user asked for it.
package powerprofile

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/power/profile"

	"github.com/stubbedev/lys/wm/internal/loop"
)

const callTimeout = 5 * time.Second

type state struct {
	asOf  dbus.Sequence
	owned bool
	saver bool
}

type Watcher struct {
	conn    *dbus.Conn
	loop    *loop.Loop
	changed func(saver bool) error
	report  func(error)
	cancel  context.CancelFunc
	watch   *dbusx.Watch
	read    chan struct{}

	mu   sync.Mutex
	lost []error

	last  state
	saver bool
}

func Watch(l *loop.Loop, address string, changed func(saver bool) error, report func(error)) (*Watcher, error) {
	conn, err := dbus.Connect(address)
	if err != nil {
		return nil, fmt.Errorf("powerprofile: connect to the system bus: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &Watcher{conn: conn, loop: l, changed: changed, report: report, cancel: cancel, read: make(chan struct{})}
	w.watch, err = dbusx.WatchSignals(ctx, conn, dbusx.FromOwner(profile.ServiceName), w.failed,
		dbusx.OnOwnerChange(profile.ServiceName, func(_, owner string, announced dbus.Sequence) error {
			if owner == "" {
				return w.post(state{asOf: announced})
			}
			return w.refresh(ctx)
		}),
		dbusx.On(dbusx.PropertiesChanged, profile.ServicePath, w.signalled).WithArg0(profile.Interface))
	if err != nil {
		close(w.read)
		return nil, errors.Join(fmt.Errorf("powerprofile: follow %s: %w", profile.ServiceName, err), w.Close())
	}
	go func() {
		defer close(w.read)
		if err := w.refresh(ctx); err != nil {
			w.failed(err)
		}
	}()
	return w, nil
}

// Read reports whether lys-power's profile is power-saver now, asked
// once on the system bus at address: what lys-wm lights its outputs for
// before Watch follows the profile. No lys-power is no power saver.
func Read(address string) (bool, error) {
	conn, err := dbus.Connect(address)
	if err != nil {
		return false, fmt.Errorf("powerprofile: connect to the system bus: %w", err)
	}
	st, err := read(context.Background(), conn)
	return st.owned && st.saver, errors.Join(err, conn.Close())
}

func (w *Watcher) signalled(c dbusx.ChangedProperties, sent dbus.Sequence) error {
	v, ok := c.Changed[profile.Property]
	if !ok {
		return nil
	}
	saver, err := isSaver(v)
	if err != nil {
		return err
	}
	return w.post(state{asOf: sent, owned: true, saver: saver})
}

func isSaver(v dbus.Variant) (bool, error) {
	name, ok := v.Value().(string)
	if !ok {
		return false, fmt.Errorf("powerprofile: %s.%s is a %s, not a string", profile.Interface, profile.Property, v.Signature())
	}
	p, err := profile.Parse(name)
	if err != nil {
		return false, fmt.Errorf("powerprofile: %s.%s: %w", profile.Interface, profile.Property, err)
	}
	return p == profile.PowerSaver, nil
}

func (w *Watcher) refresh(ctx context.Context) error {
	st, err := read(ctx, w.conn)
	if err != nil {
		return err
	}
	return w.post(st)
}

func read(ctx context.Context, conn *dbus.Conn) (state, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	has := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, profile.ServiceName)
	var owned bool
	if err := has.Store(&owned); err != nil {
		return state{}, fmt.Errorf("powerprofile: ask whether %s runs: %w", profile.ServiceName, err)
	}
	if !owned {
		return state{asOf: has.ResponseSequence}, nil
	}
	get := conn.Object(profile.ServiceName, profile.ServicePath).CallWithContext(ctx, dbusx.PropertiesInterface+".Get", 0, profile.Interface, profile.Property)
	var v dbus.Variant
	if err := get.Store(&v); err != nil {
		return state{}, fmt.Errorf("powerprofile: read %s.%s: %w", profile.Interface, profile.Property, err)
	}
	saver, err := isSaver(v)
	if err != nil {
		return state{}, err
	}
	return state{asOf: get.ResponseSequence, owned: true, saver: saver}, nil
}

func (w *Watcher) post(st state) error {
	if err := w.loop.Post(func() error { return w.apply(st) }); err != nil {
		return fmt.Errorf("powerprofile: hand %s's profile to the loop: %w", profile.ServiceName, err)
	}
	return nil
}

func (w *Watcher) apply(st state) error {
	if st.asOf <= w.last.asOf {
		return nil
	}
	w.last = st
	saver := st.owned && st.saver
	if saver == w.saver {
		return nil
	}
	w.saver = saver
	return w.changed(saver)
}

func (w *Watcher) failed(err error) {
	if perr := w.loop.Post(func() error {
		w.report(err)
		return nil
	}); perr != nil {
		w.mu.Lock()
		w.lost = append(w.lost, fmt.Errorf("powerprofile: report %w to the loop: %w", err, perr))
		w.mu.Unlock()
	}
}

func (w *Watcher) Close() error {
	w.cancel()
	<-w.read
	var errs []error
	if w.watch != nil {
		if err := w.watch.Stop(); err != nil && !errors.Is(err, dbusx.ErrDisconnected) {
			errs = append(errs, fmt.Errorf("powerprofile: %w", err))
		}
	}
	errs = append(errs, w.conn.Close())
	w.mu.Lock()
	defer w.mu.Unlock()
	return errors.Join(append(errs, w.lost...)...)
}
