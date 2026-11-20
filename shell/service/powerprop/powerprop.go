// Package powerprop follows one property lys-power publishes on the
// system bus: read at once and whenever lys-power comes onto the bus,
// replaced by each PropertiesChanged, and absent while lys-power is.
package powerprop

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/feed"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const callTimeout = 5 * time.Second

// Source names the service and decodes its property: Read fetches it
// and Changed reads it out of a PropertiesChanged, false when the
// signal does not carry it.
type Source[T any] struct {
	Name      string
	Path      dbus.ObjectPath
	Interface string
	Read      func(ctx context.Context) (T, error)
	Changed   func(*dbus.Signal) (T, bool, error)
}

// Follower is the property as last read or announced.
type Follower[T any] struct {
	src     Source[T]
	cancel  context.CancelFunc
	watch   *dbusx.Watch
	changes feed.Feed[struct{}]

	serial sync.Mutex

	mu      sync.Mutex
	value   T
	running bool
}

// Follow starts following src on the system bus conn; failed is told of
// each value that could not be read.
func Follow[T any](conn *dbus.Conn, src Source[T], failed func(error)) (*Follower[T], error) {
	ctx, cancel := context.WithCancel(context.Background())
	f := &Follower[T]{src: src, cancel: cancel}
	f.serial.Lock()
	defer f.serial.Unlock()
	watch, err := dbusx.WatchSignals(ctx, conn, dbusx.FromOwner(src.Name), failed,
		dbusx.OnOthers(dbusx.PropertiesInterface, src.Path, f.changed).WithArg0(src.Interface),
		dbusx.OnOwnerChange(src.Name, func(_, owner string, _ dbus.Sequence) error {
			f.serial.Lock()
			defer f.serial.Unlock()
			if owner == "" {
				var zero T
				f.set(zero, false)
				return nil
			}
			return f.read(ctx)
		}))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("follow %s: %w", src.Name, err)
	}
	f.watch = watch
	if err := f.read(ctx); err != nil {
		failed(err)
	}
	return f, nil
}

func (f *Follower[T]) changed(sig *dbus.Signal) error {
	f.serial.Lock()
	defer f.serial.Unlock()
	v, ok, err := f.src.Changed(sig)
	err = errors.Join(err, sig.CloseFDs())
	if err != nil || !ok {
		return err
	}
	f.set(v, true)
	return nil
}

func (f *Follower[T]) read(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	v, err := f.src.Read(ctx)
	if dbusx.Unowned(err) {
		var zero T
		f.set(zero, false)
		return nil
	}
	if err != nil {
		return err
	}
	f.set(v, true)
	return nil
}

func (f *Follower[T]) set(v T, running bool) {
	f.mu.Lock()
	f.value, f.running = v, running
	f.mu.Unlock()
	f.changes.Publish(struct{}{})
}

// Value is the last value, and whether lys-power serves one.
func (f *Follower[T]) Value() (T, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.value, f.running
}

// Subscribe ticks whenever the value changed.
func (f *Follower[T]) Subscribe() (<-chan struct{}, func()) { return f.changes.Subscribe() }

// Close stops following.
func (f *Follower[T]) Close() error {
	f.cancel()
	defer f.changes.Close()
	if err := f.watch.Stop(); err != nil && !errors.Is(err, dbusx.ErrDisconnected) {
		return fmt.Errorf("stop following %s: %w", f.src.Name, err)
	}
	return nil
}
