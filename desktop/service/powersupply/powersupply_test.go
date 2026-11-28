package powersupply

import (
	"context"
	"errors"
	"math/bits"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/waittest"
)

const quietWindow = 200 * time.Millisecond

type tree struct {
	t   *testing.T
	dir string
}

func newTree(t *testing.T) tree {
	return tree{t: t, dir: t.TempDir()}
}

func (tr tree) set(name string, attrs ...string) {
	tr.t.Helper()
	if len(attrs)%2 != 0 {
		tr.t.Fatalf("attributes of %s are not pairs: %q", name, attrs)
	}
	for i := 0; i < len(attrs); i += 2 {
		path := filepath.Join(tr.dir, name, attrs[i])
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			tr.t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(attrs[i+1]+"\n"), 0o644); err != nil {
			tr.t.Fatal(err)
		}
	}
}

func (tr tree) remove(name string) {
	tr.t.Helper()
	if err := os.RemoveAll(filepath.Join(tr.dir, name)); err != nil {
		tr.t.Fatal(err)
	}
}

func (tr tree) ac(online bool) {
	value := "0"
	if online {
		value = "1"
	}
	tr.set("AC", "type", "Mains", "online", value)
}

func (tr tree) battery(name, status string, energyNow, energyFull, powerNow int) {
	tr.set(name, "type", "Battery", "status", status, "present", "1",
		"energy_now", strconv.Itoa(energyNow), "energy_full", strconv.Itoa(energyFull),
		"energy_full_design", "60000000", "power_now", strconv.Itoa(powerNow), "cycle_count", "120")
}

// bitmapOf prints bits as the kernel prints a capability bitmap.
func bitmapOf(set ...int) string {
	top := slices.Max(append(set, 0)) / bits.UintSize
	words := make([]uint, top+1)
	for _, b := range set {
		words[b/bits.UintSize] |= 1 << (b % bits.UintSize)
	}
	out := make([]string, 0, len(words))
	for _, w := range slices.Backward(words) {
		out = append(out, strconv.FormatUint(uint64(w), 16))
	}
	return strings.Join(out, " ")
}

func (tr tree) mouse(name string, capacity int) {
	tr.set(name, "type", "Battery", "scope", "Device", "status", "Discharging",
		"capacity", strconv.Itoa(capacity), "model_name", "MX Master 3",
		"device/input/input7/capabilities/key", bitmapOf(btnMouse, keyEsc),
		"device/input/input7/capabilities/rel", bitmapOf(relX, relY),
		"device/input/input8/capabilities/key", bitmapOf(func() []int {
			keys := []int{}
			for k := keyEsc; k <= keyD; k++ {
				keys = append(keys, k)
			}
			return keys
		}()...))
}

func change(g *Groups, name string) {
	g.Send("ACTION=change", "SUBSYSTEM=power_supply", "DEVPATH=/devices/"+name)
}

func TestReadAggregatesTheMachinesBattery(t *testing.T) {
	t.Parallel()
	tr := newTree(t)
	tr.ac(false)
	tr.battery("BAT0", "Discharging", 30000000, 50000000, 10000000)
	tr.set("BAT0", "capacity", "60")
	got, err := New(tr.dir).Read(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := Device{
		Percentage: 60, State: StateDischarging, TimeToEmpty: 3 * time.Hour, IsPresent: true,
		EnergyRate: 10, Energy: 30, EnergyFull: 50, Capacity: 50 * 100.0 / 60, ChargeCycles: 120, WarningLevel: WarningNone,
	}
	if got != want {
		t.Errorf("Read = %+v\nwant %+v", got, want)
	}
}

func TestReadCountsChargeAtTheVoltage(t *testing.T) {
	t.Parallel()
	tr := newTree(t)
	tr.set("BAT1", "type", "Battery", "status", "Charging", "charge_now", "2000000", "charge_full", "4000000",
		"current_now", "-1000000", "voltage_now", "10000000")
	got, err := New(tr.dir).Read(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Percentage != 50 || got.Energy != 20 || got.EnergyFull != 40 || got.EnergyRate != 10 || got.TimeToFull != 2*time.Hour {
		t.Errorf("Read = %+v, want 50%% of 40 Wh charging at 10 W, full in 2h", got)
	}
}

func TestTwoBatteriesReadAsOne(t *testing.T) {
	t.Parallel()
	tr := newTree(t)
	tr.battery("BAT0", "Full", 50000000, 50000000, 0)
	tr.battery("BAT1", "Discharging", 10000000, 50000000, 5000000)
	got, err := New(tr.dir).Read(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Percentage != 60 || got.State != StateDischarging || got.TimeToEmpty != 12*time.Hour {
		t.Errorf("Read = %+v, want 60%% discharging, empty in 12h", got)
	}
}

func TestWarningsFollowTheLevelOnlyOnBattery(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		status   string
		capacity string
		want     WarningLevel
	}{
		{"Discharging", "21", WarningNone},
		{"Discharging", "20", WarningLow},
		{"Discharging", "5", WarningCritical},
		{"Discharging", "2", WarningAction},
		{"Charging", "2", WarningNone},
		{"Not charging", "2", WarningNone},
	} {
		tr := newTree(t)
		tr.set("BAT0", "type", "Battery", "status", tc.status, "capacity", tc.capacity)
		got, err := New(tr.dir).Read(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if got.WarningLevel != tc.want {
			t.Errorf("%s at %s%% warns %v, want %v", tc.status, tc.capacity, got.WarningLevel, tc.want)
		}
	}
}

func TestNoBatteryReadsAbsent(t *testing.T) {
	t.Parallel()
	tr := newTree(t)
	tr.ac(true)
	s := New(tr.dir)
	got, err := s.Read(t.Context())
	if err != nil || got.IsPresent {
		t.Errorf("Read = %+v, %v; want no battery", got, err)
	}
	if on, err := s.OnBattery(t.Context()); err != nil || on {
		t.Errorf("OnBattery = %v, %v; a desktop never runs on battery", on, err)
	}
}

func TestAMissingOrMalformedAttributeIsTheDefinedError(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		attrs []string
		want  error
	}{
		{"no status", []string{"type", "Battery", "capacity", "50"}, ErrMissing},
		{"no level", []string{"type", "Battery", "status", "Charging"}, ErrMissing},
		{"no type", []string{"status", "Charging"}, ErrMissing},
		{"capacity over 100", []string{"type", "Battery", "status", "Charging", "capacity", "140"}, ErrMalformed},
		{"capacity not a number", []string{"type", "Battery", "status", "Charging", "capacity", "full"}, ErrMalformed},
		{"unknown status", []string{"type", "Battery", "status", "Exploding", "capacity", "50"}, ErrMalformed},
		{"charge without voltage", []string{"type", "Battery", "status", "Charging", "charge_now", "1"}, ErrMissing},
		{"peripheral level unknown to the kernel", []string{"type", "Battery", "scope", "Device", "status", "Discharging", "capacity_level", "Soon"}, ErrMalformed},
	} {
		tr := newTree(t)
		tr.set("BAT0", tc.attrs...)
		s := New(tr.dir)
		_, readErr := s.Read(t.Context())
		_, periphErr := s.Peripherals(t.Context())
		for what, err := range map[string]error{"Read": readErr, "Peripherals": periphErr} {
			if !errors.Is(err, tc.want) {
				t.Errorf("%s: %s = %v, want %v", tc.name, what, err, tc.want)
			}
		}
	}
}

func TestPeripheralsAreTheBatteriesOfDevices(t *testing.T) {
	t.Parallel()
	tr := newTree(t)
	tr.ac(true)
	tr.battery("BAT0", "Charging", 30000000, 50000000, 10000000)
	tr.mouse("hidpp_battery_0", 15)
	tr.set("hid-pad-battery", "type", "Battery", "scope", "Device", "status", "Unknown", "capacity_level", "High",
		"device/input/input9/capabilities/key", bitmapOf(btnGamepad))
	tr.set("hid-quiet-battery", "type", "Battery", "scope", "Device", "status", "Unknown", "capacity_level", "Unknown")
	tr.set("hid-gone-battery", "type", "Battery", "scope", "Device", "present", "0", "status", "Unknown", "capacity", "10")
	got, err := New(tr.dir).Peripherals(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := []Peripheral{
		{Name: "hid-pad-battery", Kind: KindGamingInput, Percentage: 70, State: StateUnknown, WarningLevel: WarningNone},
		{Name: "hidpp_battery_0", Kind: KindMouse, Model: "MX Master 3", Percentage: 15, State: StateDischarging, WarningLevel: WarningLow},
	}
	if !slices.Equal(got, want) {
		t.Errorf("Peripherals = %+v\nwant %+v", got, want)
	}
}

func TestInputsTellTheKind(t *testing.T) {
	t.Parallel()
	keyboard := make([]int, 0, keyD)
	for k := keyEsc; k <= keyD; k++ {
		keyboard = append(keyboard, k)
	}
	for _, tc := range []struct {
		name          string
		key, rel, abs []int
		want          DeviceKind
	}{
		{"pen", []int{btnToolPen}, nil, []int{absX}, KindPen},
		{"touchpad", []int{btnToolFinger, btnMouse}, nil, []int{absX}, KindTouchpad},
		{"mouse", []int{btnMouse}, []int{relX, relY}, nil, KindMouse},
		{"keyboard", keyboard, nil, nil, KindKeyboard},
		{"a few keys", keyboard[:10], nil, nil, KindUnknown},
		{"buttons without motion", []int{btnMouse}, nil, nil, KindUnknown},
	} {
		tr := newTree(t)
		attrs := []string{"type", "Battery", "scope", "Device", "status", "Discharging", "capacity", "50"}
		for name, set := range map[string][]int{"key": tc.key, "rel": tc.rel, "abs": tc.abs} {
			if set != nil {
				attrs = append(attrs, "device/input/input1/capabilities/"+name, bitmapOf(set...))
			}
		}
		tr.set("hid-battery", attrs...)
		got, err := New(tr.dir).Peripherals(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Kind != tc.want {
			t.Errorf("%s: Peripherals = %+v, want one of kind %v", tc.name, got, tc.want)
		}
	}
}

func TestAMalformedCapabilityIsTheDefinedError(t *testing.T) {
	t.Parallel()
	tr := newTree(t)
	tr.set("hid-battery", "type", "Battery", "scope", "Device", "status", "Discharging", "capacity", "50",
		"device/input/input1/capabilities/key", "zz")
	if _, err := New(tr.dir).Peripherals(t.Context()); !errors.Is(err, ErrMalformed) {
		t.Errorf("Peripherals = %v, want %v", err, ErrMalformed)
	}
}

func requireQuiet(t *testing.T, ticks <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ticks:
		t.Errorf("a tick for %s", what)
	case <-time.After(quietWindow):
	}
}

func requireStopped(t *testing.T, ticks <-chan struct{}, stop func() error) {
	t.Helper()
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	for range ticks {
	}
}

func TestPlugAndUnplugTickTheBatterySource(t *testing.T) {
	t.Parallel()
	tr := newTree(t)
	tr.ac(true)
	tr.battery("BAT0", "Charging", 30000000, 50000000, 10000000)
	s, g := NewTest(t, tr.dir, 0)
	ticks, stop, err := s.SubscribeOnBattery(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	change(g, "BAT0")
	requireQuiet(t, ticks, "an event that changed no power source")

	tr.ac(false)
	change(g, "AC")
	waittest.For(t, ticks, "the unplug")
	if on, err := s.OnBattery(t.Context()); err != nil || !on {
		t.Errorf("OnBattery after the unplug = %v, %v", on, err)
	}
	tr.ac(true)
	change(g, "AC")
	waittest.For(t, ticks, "the plug")
	if on, err := s.OnBattery(t.Context()); err != nil || on {
		t.Errorf("OnBattery after the plug = %v, %v", on, err)
	}
	requireStopped(t, ticks, stop)
}

func TestALevelChangeTicksAtTheEventOrTheRefresh(t *testing.T) {
	t.Parallel()
	tr := newTree(t)
	tr.battery("BAT0", "Discharging", 30000000, 50000000, 10000000)
	s, g := NewTest(t, tr.dir, 0)
	ticks, stop, err := s.Subscribe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	tr.battery("BAT0", "Discharging", 29000000, 50000000, 10000000)
	change(g, "BAT0")
	waittest.For(t, ticks, "the level the event announced")
	requireStopped(t, ticks, stop)

	refreshed, _ := NewTest(t, tr.dir, 20*time.Millisecond)
	ticks, stop, err = refreshed.Subscribe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	requireQuiet(t, ticks, "a refresh that read the same level")
	tr.battery("BAT0", "Discharging", 28000000, 50000000, 10000000)
	waittest.For(t, ticks, "the level no event announced")
	requireStopped(t, ticks, stop)
}

func TestAPeripheralComingAndGoingTicks(t *testing.T) {
	t.Parallel()
	tr := newTree(t)
	s, g := NewTest(t, tr.dir, 0)
	ticks, stop, err := s.SubscribePeripherals(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	tr.mouse("hidpp_battery_0", 50)
	g.Send("ACTION=add", "SUBSYSTEM=power_supply", "DEVPATH=/devices/hidpp_battery_0")
	waittest.For(t, ticks, "the mouse appearing")
	if got, err := s.Peripherals(t.Context()); err != nil || len(got) != 1 {
		t.Errorf("Peripherals with the mouse = %+v, %v", got, err)
	}
	tr.remove("hidpp_battery_0")
	g.Send("ACTION=remove", "SUBSYSTEM=power_supply", "DEVPATH=/devices/hidpp_battery_0")
	waittest.For(t, ticks, "the mouse leaving")
	if got, err := s.Peripherals(t.Context()); err != nil || len(got) != 0 {
		t.Errorf("Peripherals without the mouse = %+v, %v", got, err)
	}
	requireStopped(t, ticks, stop)
}

func TestTheWatchEndsWithItsContext(t *testing.T) {
	t.Parallel()
	tr := newTree(t)
	s, _ := NewTest(t, tr.dir, 0)
	ctx, cancel := context.WithCancel(t.Context())
	ticks, stop, err := s.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	for range ticks {
	}
	if err := stop(); err != nil {
		t.Errorf("stop after the context ended = %v", err)
	}
}

func TestOnACReadsNoBattery(t *testing.T) {
	t.Parallel()
	tr := newTree(t)
	tr.set("BAT0", "type", "Battery")
	tr.set("ucsi-source-psy-1", "type", "USB", "online", "1")
	if on, err := OnAC(tr.dir); err != nil || !on {
		t.Errorf("OnAC = %v, %v; want online through USB despite a battery with no status", on, err)
	}
	tr.set("ucsi-source-psy-1", "online", "0")
	if on, err := OnAC(tr.dir); err != nil || on {
		t.Errorf("OnAC = %v, %v; want offline", on, err)
	}
}

func TestSystemBatteriesAreTheMachinesOwn(t *testing.T) {
	t.Parallel()
	tr := newTree(t)
	tr.mouse("hid-mouse-battery", 40)
	tr.set("BAT0", "type", "Battery", "scope", "System")
	tr.set("BAT1", "type", "Battery", "present", "0")
	tr.set("CMB0", "type", "Battery")
	tr.ac(true)
	got, err := SystemBatteries(tr.dir)
	if err != nil || !slices.Equal(got, []string{"BAT0", "CMB0"}) {
		t.Errorf("SystemBatteries = %q, %v; want BAT0 and CMB0 without the mouse or the empty bay", got, err)
	}
	tr.set("broken", "scope", "System")
	if _, err := SystemBatteries(tr.dir); !errors.Is(err, ErrMissing) {
		t.Errorf("SystemBatteries over a supply without a type = %v, want ErrMissing", err)
	}
}
