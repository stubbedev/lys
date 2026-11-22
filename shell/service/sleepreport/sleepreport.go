// Package sleepreport follows, for the shell, lys-power's report on each
// sleep, and tells the user once of every sleep that stayed shallow while
// it drained the battery, with its drain rate and what kept it shallow or
// woke it.
package sleepreport

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/power/sleep"

	"github.com/stubbedev/lys/shell/service/powerprop"
)

const (
	appName = "lys-shell"
	icon    = "ld-battery-warning-symbolic"
)

// Monitor is the reports lys-power publishes.
type Monitor struct {
	*powerprop.Follower[[]sleep.Report]
}

// Follow starts following the reports on the system bus conn; failed is
// told of each list that could not be read.
func Follow(conn *dbus.Conn, failed func(error)) *Monitor {
	client := sleep.NewClient(conn)
	return &Monitor{powerprop.Follow(conn, powerprop.Source[[]sleep.Report]{
		Name: sleep.ServiceName, Path: sleep.ServicePath, Interface: sleep.Interface,
		Read: client.Sleeps, Changed: sleep.Changed,
	}, failed)}
}

// Seen is the newest sleep the shell has looked at.
type Seen struct {
	woke  time.Time
	known bool
}

// Step is what was seen after reports, oldest first, and the newest
// sleep, when it is new and drained the battery. The first list only
// marks what was seen: a sleep from before the shell started, or
// before lys-power restarted, is not news.
func (s Seen) Step(reports []sleep.Report) (Seen, sleep.Report, bool) {
	if len(reports) == 0 {
		return Seen{woke: s.woke, known: true}, sleep.Report{}, false
	}
	newest := reports[len(reports)-1]
	if s.known && !newest.Woke.After(s.woke) {
		return s, sleep.Report{}, false
	}
	next := Seen{woke: newest.Woke, known: true}
	if !s.known {
		return next, sleep.Report{}, false
	}
	_, drained := newest.Drained()
	return next, newest, drained
}

// Message is the notification on r, a sleep that drained the battery.
func Message(r sleep.Report) (summary, body string) {
	mw, _ := r.Drained()
	lines := []string{i18n.T("battery-sleep-drained-body",
		i18n.Str("watts", strconv.FormatFloat(float64(mw)/1000, 'f', 1, 64)),
		i18n.Str("hours", strconv.FormatFloat(r.Asleep().Hours(), 'f', 1, 64)),
		i18n.Str("percent", strconv.FormatFloat(r.DeepShare*100, 'f', 0, 64)))}
	if len(r.Blockers) > 0 {
		lines = append(lines, i18n.T("battery-sleep-blocked", i18n.Str("devices", strings.Join(r.Blockers, ", "))))
	}
	if len(r.WokenBy) > 0 {
		lines = append(lines, i18n.T("battery-sleep-woken", i18n.Str("devices", strings.Join(r.WokenBy, ", "))))
	}
	return i18n.T("battery-sleep-drained"), strings.Join(lines, "\n")
}

// Notifier posts a notification; desktopnotify's Sender is one.
type Notifier interface {
	Send(ctx context.Context, appName, summary, body, appIcon string) (uint32, error)
}

// callTimeout bounds posting one notification.
const callTimeout = 5 * time.Second

// Announce tells of each new sleep that drained the battery until ctx
// ends.
func Announce(ctx context.Context, m *Monitor, n Notifier) {
	announcing(m).run(ctx, n)
}

// announcer is Announce once it subscribed and marked the reports it
// started on as seen.
type announcer struct {
	m     *Monitor
	ticks <-chan struct{}
	stop  func()
	seen  Seen
}

func announcing(m *Monitor) *announcer {
	a := &announcer{m: m}
	a.ticks, a.stop = m.Subscribe()
	if reports, running := m.Value(); running {
		a.seen, _, _ = a.seen.Step(reports)
	}
	return a
}

func (a *announcer) run(ctx context.Context, n Notifier) {
	defer a.stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.ticks:
		}
		reports, running := a.m.Value()
		if !running {
			continue
		}
		next, r, tell := a.seen.Step(reports)
		a.seen = next
		if !tell {
			continue
		}
		summary, body := Message(r)
		call, cancel := context.WithTimeout(ctx, callTimeout)
		if _, err := n.Send(call, appName, summary, body, icon); err != nil {
			log.Printf("sleep report: tell of the sleep from %s: %v", r.Slept, err)
		}
		cancel()
	}
}
