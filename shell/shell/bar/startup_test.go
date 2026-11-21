package bar

import "testing"

func TestTheShellStartsOnceEveryPartOfItsStartIsDone(t *testing.T) {
	var s startup
	started := 0
	s.await([]startupPart{startupBar("DP-1"), startupBar("HDMI-A-1"), startupDropdowns, startupTray}, func() { started++ })
	s.done(startupBar("DP-1"))
	s.done(startupDropdowns)
	s.done(startupTray)
	if started != 0 {
		t.Fatal("started with the HDMI-A-1 bar still to show")
	}
	s.done(startupBar("HDMI-A-1"))
	if started != 1 {
		t.Fatalf("started %d times once every part was done, want once", started)
	}
	s.done(startupBar("DP-1"))
	s.done(startupDropdowns)
	if started != 1 {
		t.Errorf("a bar reopened later started the shell again: %d", started)
	}
}

func TestAPartDoneBeforeTheStartIsAwaitedCounts(t *testing.T) {
	var s startup
	started := 0
	s.done(startupTray)
	s.await([]startupPart{startupBar("DP-1"), startupTray}, func() { started++ })
	if started != 0 {
		t.Fatal("started before the bar showed")
	}
	s.done(startupBar("DP-1"))
	if started != 1 {
		t.Errorf("started %d times, want once", started)
	}
}

func TestAShellWithNothingToAwaitHasStartedAtOnce(t *testing.T) {
	var s startup
	started := 0
	s.await(nil, func() { started++ })
	if started != 1 {
		t.Errorf("a shell with nothing to await started %d times, want once at once", started)
	}
}

func TestTheDropdownsAreAwaitedOnlyBehindABar(t *testing.T) {
	if got := startupParts(nil, true); len(got) != 1 || got[0] != startupTray {
		t.Errorf("a shell without bars awaits %v, want the tray alone", got)
	}
	got := startupParts([]string{"DP-1"}, false)
	if len(got) != 2 || got[0] != startupBar("DP-1") || got[1] != startupDropdowns {
		t.Errorf("a bar without a tray awaits %v, want the bar and its dropdowns", got)
	}
}
