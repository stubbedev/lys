package daemon

import (
	"bytes"
	"testing"

	"github.com/stubbedev/lys/splash/internal/kms"
	"github.com/stubbedev/lys/splash/internal/protocol"
	"github.com/stubbedev/lys/splash/internal/theme"
)

func TestUpdateProgressShowsOnceKnownAndNeverGoesBack(t *testing.T) {
	clock := newFakeClock()
	d, _ := clockedDaemon(t, clock)
	steps := []struct {
		req      protocol.Request
		updating bool
		known    bool
		percent  protocol.Percent
	}{
		{protocol.ChangeMode{Mode: "firmware-upgrade"}, true, false, 0},
		{protocol.SystemUpdate{Progress: 40}, true, true, 40},
		{protocol.SystemUpdate{Progress: 30}, true, true, 40},
		{protocol.ChangeMode{Mode: "updates"}, true, true, 40},
		{protocol.SystemUpdate{Progress: 55}, true, true, 55},
		{protocol.ChangeMode{Mode: "boot-up"}, false, false, 0},
		{protocol.SystemUpdate{Progress: 5}, true, true, 5},
	}
	for i, s := range steps {
		if err := d.apply(s.req); err != nil {
			t.Fatal(err)
		}
		p := d.state.Progress
		if d.state.Updating != s.updating || p.Known != s.known || p.Percent() != s.percent {
			t.Errorf("step %d (%#v): updating %v, known %v at %d%%; want %v, %v at %d%%", i, s.req, d.state.Updating, p.Known, p.Percent(), s.updating, s.known, s.percent)
		}
	}
}

func TestAnEasedBarDrawsOnlyWhileItMoves(t *testing.T) {
	clock := newFakeClock()
	d, s := clockedDaemon(t, clock)
	for _, p := range []protocol.Percent{10, 60} {
		if err := d.apply(protocol.SystemUpdate{Progress: p}); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.redraw(); err != nil {
		t.Fatal(err)
	}
	d.ticking, d.phaseAt = true, clock.Now()
	first, wakes := s.presents, 0
	for d.animating() {
		clock.advance(d.untilNextChange())
		d.step()
		if err := d.redraw(); err != nil {
			t.Fatal(err)
		}
		wakes++
		if wakes > 2*int(theme.Frames) {
			t.Fatal("the bar never settles")
		}
	}
	if got := s.presents - first; got != wakes || wakes < 3 {
		t.Errorf("%d presents for %d wakes of an eased bar, want one each and a motion", got, wakes)
	}
	settled := d.state.Advance(theme.Frames)
	if settled.Progress.Shown() != theme.FillOf(60) {
		t.Fatalf("the motion ends at %d", settled.Progress.Shown())
	}
	f := kms.Frame{Pix: make([]byte, len(s.frame.Pix)), Stride: s.frame.Stride, Size: s.frame.Size}
	if _, err := d.renderer.Draw(f, 1, nil, settled); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(f.Pix, s.frame.Pix) {
		t.Error("the bar where it stopped drawing differs from the bar at its target")
	}
}
