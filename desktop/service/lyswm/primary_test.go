package lyswm_test

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/service/lyswm"
	"github.com/stubbedev/lys/desktop/service/lyswm/lyswmtest"
)

func outputEvent(change, name string, primary bool) string {
	p := ""
	if primary {
		p = `,"primary":true`
	}
	return `{"event":"output","change":"` + change + `","output":{"name":"` + name + `","description":"","rect":{"x":0,"y":0,"width":1,"height":1},"scale":1,"mode":{"width":1,"height":1,"refresh_mhz":60000},"workspace":1,"focused":false,"powered":true` + p + `}}`
}

func expectPrimary(t *testing.T, w *lyswm.PrimaryWatcher, want string) {
	t.Helper()
	if got := w.Primary(); got != want {
		t.Fatalf("primary = %q, want %q", got, want)
	}
}

func TestPrimaryIsSeededFromTheOutputs(t *testing.T) {
	f := lyswmtest.Start(t)
	f.Result("outputs", `[{"name":"DP-1"},{"name":"HDMI-A-1","primary":true}]`)
	w, err := lyswm.WatchPrimary()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	expectPrimary(t, w, "HDMI-A-1")
}

func TestNoOutputMarkedPrimaryIsNoPrimary(t *testing.T) {
	f := lyswmtest.Start(t)
	f.Result("outputs", `[{"name":"DP-1"},{"name":"HDMI-A-1"}]`)
	w, err := lyswm.WatchPrimary()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	expectPrimary(t, w, "")
}

func TestPrimaryFailsWhenTheSeedFails(t *testing.T) {
	f := lyswmtest.Start(t)
	f.Answer("outputs", `{"version":1,"error":"unknown request"}`)
	if w, err := lyswm.WatchPrimary(); err == nil {
		w.Close()
		t.Fatal("WatchPrimary succeeded without a seed")
	}
}

func TestPrimaryFollowsTheOutputEvents(t *testing.T) {
	f := lyswmtest.Start(t)
	f.Result("outputs", `[{"name":"DP-1","primary":true},{"name":"HDMI-A-1"}]`)
	w, err := lyswm.WatchPrimary()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	f.Events <- `{"event":"workspace","output":"DP-1","workspace":2}`
	select {
	case <-w.Ticks():
		t.Fatal("ticked on a workspace event")
	case <-time.After(quietWindow):
	}
	expectPrimary(t, w, "DP-1")
	steps := []struct {
		event, want string
	}{
		{outputEvent("changed", "HDMI-A-1", true), "HDMI-A-1"},
		{outputEvent("changed", "DP-1", false), "HDMI-A-1"},
		{outputEvent("changed", "HDMI-A-1", false), ""},
		{outputEvent("added", "DP-2", true), "DP-2"},
		{outputEvent("removed", "HDMI-A-1", false), "DP-2"},
		{outputEvent("removed", "DP-2", true), ""},
	}
	for _, s := range steps {
		f.Events <- s.event
		awaitTick(t, w.Watcher)
		expectPrimary(t, w, s.want)
	}
}
