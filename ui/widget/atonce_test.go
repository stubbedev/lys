package widget

import (
	"sync"
	"testing"
)

func atOnce(t *testing.T, workers int, run func(worker int) error) {
	t.Helper()
	errs := make([]error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			<-start
			errs[w] = run(w)
		})
	}
	close(start)
	wg.Wait()
	for w, err := range errs {
		if err != nil {
			t.Errorf("worker %d: %v", w, err)
		}
	}
}
