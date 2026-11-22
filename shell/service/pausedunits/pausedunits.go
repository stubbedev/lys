// Package pausedunits follows, for the shell, the units lys-power pauses
// on battery: what of the system's and the user's own is paused or failed
// to, the notification that says what was just paused, and its two
// answers, resuming everything until the next unplug and keeping those
// units running on battery from now on.
package pausedunits

import (
	"context"
	"errors"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/feed"
	"github.com/stubbedev/lys/desktop/i18n"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/power/pause"

	"github.com/stubbedev/lys/shell/internal/desktopnotify"
	"github.com/stubbedev/lys/shell/service/powerprop"
)

// The notification's buttons.
const (
	ActionResume = "resume"
	ActionKeep   = "keep-running"
)

const (
	appName = "lys-shell"
	icon    = "ld-circle-pause-symbolic"
	expiry  = -time.Millisecond
)

// Monitor is the state lys-power publishes, as much of it as the user
// may see, and the units it newly paused that were not yet told of.
type Monitor struct {
	*powerprop.Follower[pause.State]
	client *pause.Client
	news   news
}

// news is what lys-power paused since the state it was first read in,
// when following started or it came back onto the bus, and is paused
// still.
type news struct {
	mu      sync.Mutex
	known   pause.State
	pending []pause.Unit
	ticks   feed.Tick
}

func (n *news) read(st pause.State) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.pending = stillPaused(n.pending, st)
	n.known = st
}

func (n *news) changed(st pause.State) {
	n.mu.Lock()
	defer n.mu.Unlock()
	newly := pause.NewlyPaused(n.known, st)
	n.pending = append(stillPaused(n.pending, st), newly...)
	n.known = st
	if len(newly) > 0 {
		feed.Notify(&n.ticks)
	}
}

func (n *news) take() []pause.Unit {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := n.pending
	n.pending = nil
	return out
}

func stillPaused(units []pause.Unit, st pause.State) []pause.Unit {
	paused := st.Paused()
	return slices.DeleteFunc(units, func(u pause.Unit) bool { return !slices.Contains(paused, u) })
}

// Follow starts following the state on the system bus conn for the user
// uid; failed is told of each state that could not be read.
func Follow(conn *dbus.Conn, uid uint32, failed func(error)) *Monitor {
	m := &Monitor{client: pause.NewClient(conn)}
	m.Follower = powerprop.Follow(conn, powerprop.Source[pause.State]{
		Name: pause.ServiceName, Path: pause.ServicePath, Interface: pause.Interface,
		Read: func(ctx context.Context) (pause.State, error) {
			st, err := m.client.State(ctx)
			if err != nil {
				return pause.State{}, err
			}
			visible := st.VisibleTo(uid)
			m.news.read(visible)
			return visible, nil
		},
		Changed: func(sig *dbus.Signal) (pause.State, bool, error) {
			st, ok, err := pause.Changed(sig)
			if err != nil || !ok {
				return pause.State{}, ok, err
			}
			visible := st.VisibleTo(uid)
			m.news.changed(visible)
			return visible, true, nil
		},
	}, failed)
	return m
}

// Resume resumes every paused unit until the next unplug.
func (m *Monitor) Resume(ctx context.Context) error { return m.client.Resume(ctx) }

// KeepRunning stops pausing units on battery, which resumes them.
func (m *Monitor) KeepRunning(ctx context.Context, units []pause.Unit) error {
	st, _ := m.Value()
	var errs []error
	if names, ok := Remaining(st, units, pause.ScopeUser); ok {
		errs = append(errs, m.client.PickUser(ctx, names))
	}
	if names, ok := Remaining(st, units, pause.ScopeSystem); ok {
		errs = append(errs, m.client.PickSystem(ctx, names))
	}
	return errors.Join(errs...)
}

// Remaining is the names of the picks of scope without units, and
// whether units hold any of scope at all.
func Remaining(st pause.State, units []pause.Unit, scope pause.Scope) ([]string, bool) {
	if !slices.ContainsFunc(units, func(u pause.Unit) bool { return u.Scope() == scope }) {
		return nil, false
	}
	names := []string{}
	for _, u := range st.Picks {
		if u.Scope() == scope && !slices.Contains(units, u) {
			names = append(names, u.Name())
		}
	}
	return names, true
}

// Notifier posts a notification with buttons and waits for the answer;
// desktopnotify's Sender is one.
type Notifier interface {
	Ask(ctx context.Context, appName, summary, body, appIcon string, actions []desktopnotify.Action, timeout time.Duration) (key string, ok bool, err error)
}

// Announce tells of each unit newly paused until ctx ends, and acts on
// the answer. What lys-power had paused when following started, or when
// it came back onto the bus, is not news; what it paused since is, even
// before Announce started.
func Announce(ctx context.Context, m *Monitor, n Notifier) {
	ticks, stop := m.news.ticks.Subscribe()
	defer stop()
	var asking sync.WaitGroup
	defer asking.Wait()
	for {
		if newly := m.news.take(); len(newly) > 0 {
			asking.Go(func() { m.ask(ctx, n, newly) })
		}
		select {
		case <-ctx.Done():
			return
		case <-ticks:
		}
	}
}

func (m *Monitor) ask(ctx context.Context, n Notifier, units []pause.Unit) {
	names := make([]string, len(units))
	for i, u := range units {
		names[i] = u.Name()
	}
	actions := []desktopnotify.Action{
		{Key: ActionResume, Label: i18n.T("battery-paused-resume")},
		{Key: ActionKeep, Label: i18n.T("battery-paused-keep")},
	}
	key, pressed, err := n.Ask(ctx, appName, i18n.T("battery-paused"), i18n.T("battery-paused-body", i18n.Str("units", strings.Join(names, ", "))), icon, actions, expiry)
	if err != nil {
		log.Printf("paused units: tell of %v: %v", names, err)
		return
	}
	if !pressed {
		return
	}
	call, cancel := context.WithTimeout(ctx, pause.AuthTimeout)
	defer cancel()
	switch key {
	case ActionResume:
		err = m.Resume(call)
	case ActionKeep:
		err = m.KeepRunning(call, units)
	default:
		return
	}
	if err != nil {
		log.Printf("paused units: %s %v: %v", key, names, err)
	}
}
