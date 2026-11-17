package logtest

import (
	"bytes"
	"log"
	"strings"
	"sync"
	"testing"
)

func logTo(t *testing.T, w *bytes.Buffer, flags int) {
	t.Helper()
	out, before := log.Writer(), log.Flags()
	log.SetOutput(w)
	log.SetFlags(flags)
	t.Cleanup(func() {
		log.SetOutput(out)
		log.SetFlags(before)
	})
}

func TestCaptureCollectsTheLogUntilTheTestEnds(t *testing.T) {
	before := &bytes.Buffer{}
	logTo(t, before, log.LstdFlags)
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

func TestCaptureDropsTheFlagsUntilTheTestEnds(t *testing.T) {
	before := &bytes.Buffer{}
	logTo(t, before, log.Lshortfile)
	var captured *Log
	t.Run("capture", func(t *testing.T) {
		captured = Capture(t)
		log.Print("during")
	})
	if got := captured.String(); got != "during\n" {
		t.Errorf("the capture holds %q, want the bare line", got)
	}
	if got := log.Flags(); got != log.Lshortfile {
		t.Errorf("the log's flags are %d after the test, want the %d it had", got, log.Lshortfile)
	}
}

func TestResetDropsWhatWasCollected(t *testing.T) {
	logTo(t, &bytes.Buffer{}, 0)
	captured := Capture(t)
	log.Print("before")
	captured.Reset()
	log.Print("after")
	if got := captured.String(); got != "after\n" {
		t.Errorf("the capture holds %q after a reset, want only the later line", got)
	}
}

func TestTeeAlsoWritesWhereTheLogWent(t *testing.T) {
	before := &bytes.Buffer{}
	logTo(t, before, log.LstdFlags)
	var teed *Log
	t.Run("tee", func(t *testing.T) {
		teed = Tee(t)
		log.Print("during")
	})
	log.Print("after")
	if got := teed.String(); got != "during\n" {
		t.Errorf("the tee holds %q, want only the bare line logged during the test", got)
	}
	if got := before.String(); !strings.HasPrefix(got, "during\n") || !strings.Contains(got, "after") {
		t.Errorf("the log the tee wrapped holds %q, want the line logged during the test and the one after", got)
	}
}
