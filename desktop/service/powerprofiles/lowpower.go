package powerprofiles

import (
	"context"
	"fmt"
	"time"
)

// LowPowerStretch is how many times longer a refresh nobody waits for
// waits while the profile is power-saver, lys's Low Power.
const LowPowerStretch = 3

// Paced is every, stretched by LowPowerStretch under Low Power.
func Paced(every time.Duration, lowPower bool) time.Duration {
	if lowPower {
		return every * LowPowerStretch
	}
	return every
}

// FollowSaver reads whether the active profile is power-saver at once
// and again on each change of the daemon's properties, each read bound
// by timeout, keeping only the latest answer pending. An absent daemon
// reads as not power-saver. The feed closes when ctx ends, when the
// daemon's feed ends, or after a read fails; failed hears of the failure,
// and of what stopping the subscription met.
func FollowSaver(ctx context.Context, source Source, timeout time.Duration, failed func(error)) (<-chan bool, error) {
	ticks, stop, err := source.Subscribe(ctx)
	if err != nil {
		return nil, fmt.Errorf("powerprofiles: follow the active profile: %w", err)
	}
	out := make(chan bool, 1)
	go func() {
		defer close(out)
		defer func() {
			if err := stop(); err != nil {
				failed(fmt.Errorf("powerprofiles: stop following the active profile: %w", err))
			}
		}()
		for {
			saver, err := readSaver(ctx, source, timeout)
			if err != nil {
				failed(err)
				return
			}
			select {
			case <-out:
			default:
			}
			out <- saver
			select {
			case <-ctx.Done():
				return
			case _, ok := <-ticks:
				if !ok {
					return
				}
			}
		}
	}()
	return out, nil
}

func readSaver(ctx context.Context, source Source, timeout time.Duration) (bool, error) {
	call, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	snap, err := source.Read(call)
	if err != nil {
		return false, fmt.Errorf("powerprofiles: read the active profile: %w", err)
	}
	return snap.Available && snap.Active == ProfilePowerSaver, nil
}
