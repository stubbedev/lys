package bar

import (
	"context"
	"log"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/shell/internal/desktopnotify"
	"github.com/stubbedev/lys/shell/service/sleepreport"
)

func startSleepReports(system, session *dbus.Conn) func() {
	if system == nil || session == nil {
		return func() {}
	}
	monitor := sleepreport.Follow(system, func(err error) { log.Printf("sleep report: %v", err) })
	life, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		sleepreport.Announce(life, monitor, desktopnotify.NewSender(session))
	}()
	return func() {
		cancel()
		<-done
		if err := monitor.Close(); err != nil {
			log.Printf("sleep report: %v", err)
		}
	}
}
