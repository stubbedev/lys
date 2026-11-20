package bar

import (
	"context"
	"log"
	"time"

	"github.com/stubbedev/lys/desktop/service/powerprofiles"
)

// lowPowerReadTimeout bounds one read of the active profile.
const lowPowerReadTimeout = 5 * time.Second

// followLowPower hands each of set whether the power profile is
// power-saver, lys's Low Power, once it is known and on every change,
// until the returned stop has returned. It subscribes off the caller's
// goroutine, which is the loop's while the bar mounts.
func followLowPower(source powerprofiles.Source, set ...func(bool)) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		saver, err := powerprofiles.FollowSaver(ctx, source, lowPowerReadTimeout, func(err error) {
			log.Printf("bar: Low Power: %v", err)
		})
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("bar: refreshes keep their pace whatever the power profile: %v", err)
			}
			return
		}
		for on := range saver {
			for _, f := range set {
				f(on)
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}
