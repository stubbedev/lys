package bar

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/rfkill"
	"github.com/stubbedev/lys/ui/widget"
)

// fakeRadios stands in for the kernel's kill switches: an airplane
// switch soft-blocks or unblocks every transmitter class, as a
// CHANGE_ALL per class does.
type fakeRadios struct {
	mu      sync.Mutex
	snap    rfkill.Snapshot
	readErr error
	setErr  error
	written []bool
}

func (f *fakeRadios) Read(context.Context) (rfkill.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.readErr != nil {
		return rfkill.Snapshot{}, f.readErr
	}
	return rfkill.Snapshot{Radios: slices.Clone(f.snap.Radios)}, nil
}

func (f *fakeRadios) SetAirplane(_ context.Context, on bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.written = append(f.written, on)
	if f.setErr != nil {
		return f.setErr
	}
	for i, r := range f.snap.Radios {
		if slices.Contains(rfkill.AirplaneTypes, r.Type) {
			f.snap.Radios[i].Soft = on
		}
	}
	return nil
}

func (f *fakeRadios) Subscribe(context.Context) (<-chan struct{}, func() error, error) {
	return make(chan struct{}), func() error { return nil }, nil
}

func (f *fakeRadios) writes() []bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.written)
}

func (f *fakeRadios) radios() []rfkill.Radio {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.snap.Radios)
}

// noRadiosDashboard is the dashboard on a machine without kill switches,
// so no test reads the host's.
func noRadiosDashboard(ctx ModuleContext) widget.Widget {
	return newDashboardDropdown(ctx, &fakeRadios{})
}

func laptopRadios() *fakeRadios {
	return &fakeRadios{snap: rfkill.Snapshot{Radios: []rfkill.Radio{
		{Index: 0, Type: rfkill.TypeWLAN},
		{Index: 1, Type: rfkill.TypeBluetooth},
		{Index: 2, Type: rfkill.TypeWWAN},
		{Index: 3, Type: rfkill.TypeGPS},
	}}}
}

func airplaneDashboard(t *testing.T, radios *fakeRadios) (*dashboardView, *widget.Button) {
	t.Helper()
	ctx := newTestContext(t, config.Defaults())
	v := opened(newDashboardDropdown(ctx, radios)).(*dashboardView)
	t.Cleanup(v.dropdownClosed)
	v.refreshAll()
	return v, quickActionButtons(t, v)[2]
}

func TestTheAirplaneToggleBlocksAndUnblocksEveryRadio(t *testing.T) {
	radios := laptopRadios()
	v, airplane := airplaneDashboard(t, radios)
	actions := quickActionButtons(t, v)
	if onHeadlessLoop(func() bool { return !airplane.Enabled() || airplane.HasClass("active") }) {
		t.Fatal("with every radio on, the airplane toggle is not a sensitive, inactive switch")
	}
	onHeadlessLoop(func() bool { airplane.OnClick(); return true })
	waitHeadless(t, "airplane mode on", func() bool { return airplane.HasClass("active") && airplane.Enabled() })
	for _, r := range radios.radios() {
		transmitter := slices.Contains(rfkill.AirplaneTypes, r.Type)
		if r.Soft != transmitter {
			t.Errorf("after airplane on, radio %d (type %d) soft-blocked %v, want %v", r.Index, r.Type, r.Soft, transmitter)
		}
	}
	if onHeadlessLoop(func() bool { return actions[0].Enabled() || actions[1].Enabled() }) {
		t.Error("the wifi or bluetooth toggle is sensitive in airplane mode")
	}
	onHeadlessLoop(func() bool { airplane.OnClick(); return true })
	waitHeadless(t, "airplane mode off", func() bool { return !airplane.HasClass("active") && airplane.Enabled() })
	for _, r := range radios.radios() {
		if r.Soft {
			t.Errorf("after airplane off, radio %d (type %d) is still soft-blocked", r.Index, r.Type)
		}
	}
	if got := radios.writes(); !slices.Equal(got, []bool{true, false}) {
		t.Errorf("airplane writes = %v, want [true false]", got)
	}
}

func TestARefusedAirplaneSwitchSaysWhyAndShowsTheRealState(t *testing.T) {
	radios := laptopRadios()
	radios.setErr = errors.New("open rfkill /dev/rfkill: permission denied")
	v, airplane := airplaneDashboard(t, radios)
	alert := quickActionAlert(t, v)
	onHeadlessLoop(func() bool { airplane.OnClick(); return true })
	waitHeadless(t, "the failure shown", func() bool { return alert.Visible() })
	onHeadlessLoop(func() bool {
		if alert.Title() != i18n.T("dropdown-dashboard-airplane-failed") || alert.Description() != radios.setErr.Error() {
			t.Errorf("the alert reads %q / %q", alert.Title(), alert.Description())
		}
		if airplane.HasClass("active") || !airplane.Enabled() {
			t.Errorf("after the refused switch: active %v, enabled %v; want the radios' real state, off and sensitive", airplane.HasClass("active"), airplane.Enabled())
		}
		return true
	})
	for _, r := range radios.radios() {
		if r.Soft {
			t.Errorf("a refused switch blocked radio %d", r.Index)
		}
	}
}

func TestTheAirplaneToggleReadsTheKernelStateAfterARestart(t *testing.T) {
	radios := laptopRadios()
	_, first := airplaneDashboard(t, radios)
	onHeadlessLoop(func() bool { first.OnClick(); return true })
	waitHeadless(t, "airplane mode on", func() bool { return first.HasClass("active") })
	_, second := airplaneDashboard(t, radios)
	if !onHeadlessLoop(func() bool { return second.HasClass("active") }) {
		t.Error("a new dashboard over blocked radios shows airplane mode off")
	}
	unblocked := laptopRadios()
	_, fresh := airplaneDashboard(t, unblocked)
	if onHeadlessLoop(func() bool { return fresh.HasClass("active") }) {
		t.Error("a new dashboard over unblocked radios shows airplane mode on")
	}
}

func TestTheAirplaneToggleIsInsensitiveWithoutRadios(t *testing.T) {
	for _, tc := range []struct {
		name   string
		radios *fakeRadios
	}{
		{"no switches", &fakeRadios{}},
		{"only receivers", &fakeRadios{snap: rfkill.Snapshot{Radios: []rfkill.Radio{{Type: rfkill.TypeGPS}}}}},
		{"unreadable device", &fakeRadios{readErr: errors.New("open rfkill /dev/rfkill: no such file or directory")}},
	} {
		_, airplane := airplaneDashboard(t, tc.radios)
		onHeadlessLoop(func() bool {
			if airplane.Enabled() || airplane.HasClass("active") {
				t.Errorf("%s: enabled %v, active %v; want insensitive and off", tc.name, airplane.Enabled(), airplane.HasClass("active"))
			}
			airplane.OnClick()
			return true
		})
		if got := tc.radios.writes(); len(got) != 0 {
			t.Errorf("%s: a click wrote %v", tc.name, got)
		}
	}
}
