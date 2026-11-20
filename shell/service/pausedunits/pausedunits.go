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
// may see.
type Monitor struct {
	*powerprop.Follower[pause.State]
	client *pause.Client
}

// Follow starts following the state on the system bus conn for the user
// uid; failed is told of each state that could not be read.
func Follow(conn *dbus.Conn, uid uint32, failed func(error)) (*Monitor, error) {
	client := pause.NewClient(conn)
	f, err := powerprop.Follow(conn, powerprop.Source[pause.State]{
		Name: pause.ServiceName, Path: pause.ServicePath, Interface: pause.Interface,
		Read: func(ctx context.Context) (pause.State, error) {
			st, err := client.State(ctx)
			return st.VisibleTo(uid), err
		},
		Changed: func(sig *dbus.Signal) (pause.State, bool, error) {
			st, ok, err := pause.Changed(sig)
			return st.VisibleTo(uid), ok, err
		},
	}, failed)
	if err != nil {
		return nil, err
	}
	return &Monitor{Follower: f, client: client}, nil
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
// the answer. What was paused before it started, or before lys-power
// restarted, is not news.
func Announce(ctx context.Context, m *Monitor, n Notifier) {
	ticks, stop := m.Subscribe()
	defer stop()
	var asking sync.WaitGroup
	defer asking.Wait()
	prev, _ := m.Value()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
		}
		cur, running := m.Value()
		if !running {
			continue
		}
		newly := pause.NewlyPaused(prev, cur)
		prev = cur
		if len(newly) > 0 {
			asking.Go(func() { m.ask(ctx, n, newly) })
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
