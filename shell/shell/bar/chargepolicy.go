package bar

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/power/charge"

	"github.com/stubbedev/lys/shell/service/calendar"
	"github.com/stubbedev/lys/shell/service/chargestate"
)

type chargeSource interface {
	Value() (charge.State, bool)
	Subscribe() (<-chan struct{}, func())
	ChargeFull(ctx context.Context) error
}

// startCharge follows lys-power's charge policy and passes it the start
// of the next calendar event with a location.
func startCharge(ctx *ModuleContext, system *dbus.Conn) func() {
	if system == nil {
		return func() {}
	}
	monitor := chargestate.Follow(system, func(err error) { log.Printf("charge policy: %v", err) })
	ctx.Charge = monitor
	store, clk := ctx.Calendar, ctx.Clock
	if store == nil || clk == nil {
		return closeCharge(monitor, func() {})
	}
	life, cancel := context.WithCancel(context.Background())
	changes := make(chan struct{}, 1)
	forward := func(subscribe subscription) {
		followTicks(life, subscribe, func() {
			select {
			case changes <- struct{}{}:
			default:
			}
		})
	}
	forward(store.Changes)
	forward(clk.Changes)
	done := make(chan struct{})
	go func() {
		defer close(done)
		monitor.PassDepartures(life, clk, func(now time.Time) time.Time { return departure(store, now) }, changes,
			func(err error) { log.Printf("charge policy: pass the next departure: %v", err) })
	}()
	return closeCharge(monitor, func() {
		cancel()
		<-done
	})
}

func closeCharge(monitor *chargestate.Monitor, stop func()) func() {
	return func() {
		stop()
		if err := monitor.Close(); err != nil {
			log.Printf("charge policy: %v", err)
		}
	}
}

// departure is the start of the next event away from the desk, zero
// when none is ahead.
func departure(store *calendar.Store, now time.Time) time.Time {
	o, ok := store.NextAway(now)
	if !ok {
		return time.Time{}
	}
	return o.Start
}

// chargeExplanation words why the battery holds or charges to full; at
// renders a time of day. It is empty while lys-power manages no battery.
func chargeExplanation(st charge.State, at func(time.Time) string) string {
	limit := i18n.Str("limit", strconv.Itoa(st.Limit.Percent()))
	switch st.Reason {
	case charge.ReasonHolding:
		if st.FullBy.IsZero() {
			return i18n.T("dropdown-battery-hold", limit)
		}
		return i18n.T("dropdown-battery-hold-until", limit, i18n.Str("time", at(st.FullBy)))
	case charge.ReasonUnlimited:
		return i18n.T("dropdown-battery-hold-off")
	case charge.ReasonRequested:
		return i18n.T("dropdown-battery-full-requested")
	case charge.ReasonUnplug:
		return i18n.T("dropdown-battery-full-unplug", i18n.Str("time", at(st.FullBy)))
	case charge.ReasonEvent:
		return i18n.T("dropdown-battery-full-event", i18n.Str("time", at(st.FullBy)))
	case charge.ReasonUnmanaged:
	}
	return ""
}
