package udevmonitortest

import (
	"testing"

	"github.com/stubbedev/lys/lib/testbuild"
	"github.com/stubbedev/lys/lib/udevmonitor"
)

func TestImportingCostsNothing(t *testing.T) { testbuild.RequireNoInit(t) }

func TestTheListenerReadsWhatItsFilterPasses(t *testing.T) {
	t.Parallel()
	g := New(t)
	l, err := g.Listen([]string{"drm"})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	g.Send("ACTION=change", "SUBSYSTEM=mem")
	g.Send("ACTION=change", "SUBSYSTEM=drm", "HOTPLUG=1")
	ev, err := l.Next()
	if err != nil || ev.Subsystem != "drm" || ev.Action != udevmonitor.Change {
		t.Fatalf("the first event read is %+v, %v; want drm's change", ev, err)
	}
}

func TestAGroupHasOneListener(t *testing.T) {
	t.Parallel()
	g := New(t)
	l, err := g.Listen([]string{"drm"})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if second, err := g.Listen([]string{"drm"}); err == nil {
		_ = second.Close()
		t.Fatal("a second listener joined the group")
	}
}
