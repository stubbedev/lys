package settings

import (
	"context"
	"log"
	"sync"
	"time"
)

func callWithin[T any](wait time.Duration, fn func(context.Context) (T, error)) (T, error) {
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	return fn(ctx)
}

func doWithin(wait time.Duration, work func(context.Context) error) error {
	_, err := callWithin(wait, func(ctx context.Context) (struct{}, error) { return struct{}{}, work(ctx) })
	return err
}

func mergeTicks(what string, stop func() error, feeds ...<-chan struct{}) <-chan struct{} {
	out := make(chan struct{}, 1)
	var wg sync.WaitGroup
	for _, feed := range feeds {
		if feed == nil {
			continue
		}
		wg.Go(func() {
			for range feed {
				select {
				case out <- struct{}{}:
				default:
				}
			}
		})
	}
	go func() {
		wg.Wait()
		if err := stop(); err != nil {
			log.Printf("settings: stop following %s: %v", what, err)
		}
		close(out)
	}()
	return out
}

func followTicks(k *kit, ticks <-chan struct{}, reload func()) {
	if k.background == nil {
		return
	}
	go func() {
		for range ticks {
			k.invoke(reload)
		}
	}()
}
