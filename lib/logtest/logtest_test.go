package logtest

import (
	"bytes"
	"log"
	"strings"
	"sync"
	"testing"
)

func TestCaptureCollectsTheLogUntilTheTestEnds(t *testing.T) {
	before := &bytes.Buffer{}
	out := log.Writer()
	log.SetOutput(before)
	t.Cleanup(func() { log.SetOutput(out) })
	var captured *Log
	t.Run("capture", func(t *testing.T) {
		captured = Capture(t)
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() { log.Print("during") })
		}
		wg.Wait()
	})
	log.Print("after")
	if got := strings.Count(captured.String(), "during"); got != 8 {
		t.Errorf("the capture holds %d of the 8 lines logged during the test: %q", got, captured.String())
	}
	if strings.Contains(captured.String(), "after") {
		t.Errorf("the capture holds a line logged after the test ended: %q", captured.String())
	}
	if got := before.String(); strings.Contains(got, "during") || !strings.Contains(got, "after") {
		t.Errorf("the log the capture replaced holds %q, want only the line logged after the test", got)
	}
}
