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
// power-saver, lys's Low Power, at once and on every change, until the
// returned stop has returned.
func followLowPower(source powerprofiles.Source, set ...func(bool)) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	saver, err := powerprofiles.FollowSaver(ctx, source, lowPowerReadTimeout, func(err error) {
		log.Printf("bar: Low Power: %v", err)
	})
	if err != nil {
		cancel()
		log.Printf("bar: refreshes keep their pace whatever the power profile: %v", err)
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
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
