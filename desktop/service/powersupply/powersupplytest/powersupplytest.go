// Package powersupplytest is a scriptable powersupply.Source: the
// batteries and peripherals the test sets, each change ticking the
// subscriptions that follow it, and every Read counted.
package powersupplytest

import (
	"context"
	"slices"
	"sync"

	"github.com/stubbedev/lys/desktop/service/powersupply"
)

// Fake is the machine's power supplies as the test scripts them.
type Fake struct {
	mu          sync.Mutex
	device      powersupply.Device
	peripherals []powersupply.Peripheral
	reads       int
	periphReads int
	deviceSubs  map[chan struct{}]struct{}
	periphSubs  map[chan struct{}]struct{}
}

var _ powersupply.Source = (*Fake)(nil)

// New is a Fake whose batteries read as device, with no peripheral.
func New(device powersupply.Device) *Fake {
	return &Fake{device: device, deviceSubs: map[chan struct{}]struct{}{}, periphSubs: map[chan struct{}]struct{}{}}
}

// Battery is a present battery at percentage, in state and warning.
func Battery(percentage float64, state powersupply.DeviceState, warning powersupply.WarningLevel) powersupply.Device {
	return powersupply.Device{IsPresent: true, Percentage: percentage, State: state, WarningLevel: warning}
}

// SetDevice replaces the batteries and ticks their subscriptions.
func (f *Fake) SetDevice(d powersupply.Device) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.device = d
	tickAll(f.deviceSubs)
}

// SetPeripherals replaces the peripherals and ticks their
// subscriptions.
func (f *Fake) SetPeripherals(ps ...powersupply.Peripheral) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.peripherals = slices.Clone(ps)
	tickAll(f.periphSubs)
}

// Peripheral is a discharging peripheral named name.
func Peripheral(name string, kind powersupply.DeviceKind, model string, percentage float64, warning powersupply.WarningLevel) powersupply.Peripheral {
	return powersupply.Peripheral{Name: name, Kind: kind, Model: model, Percentage: percentage, State: powersupply.StateDischarging, WarningLevel: warning}
}

// PeripheralReads is how many Peripherals calls the Fake has answered.
func (f *Fake) PeripheralReads() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.periphReads
}

// Reads is how many Reads the Fake has answered.
func (f *Fake) Reads() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads
}

// Read answers the batteries as last set.
func (f *Fake) Read(context.Context) (powersupply.Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	return f.device, nil
}

// Peripherals answers the peripherals as last set.
func (f *Fake) Peripherals(context.Context) ([]powersupply.Peripheral, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.periphReads++
	return slices.Clone(f.peripherals), nil
}

// Subscribe ticks per SetDevice.
func (f *Fake) Subscribe(ctx context.Context) (<-chan struct{}, func() error, error) {
	return f.subscribe(ctx, f.deviceSubs)
}

// SubscribePeripherals ticks per SetPeripherals.
func (f *Fake) SubscribePeripherals(ctx context.Context) (<-chan struct{}, func() error, error) {
	return f.subscribe(ctx, f.periphSubs)
}

func (f *Fake) subscribe(ctx context.Context, subs map[chan struct{}]struct{}) (<-chan struct{}, func() error, error) {
	ch := make(chan struct{}, 1)
	f.mu.Lock()
	subs[ch] = struct{}{}
	f.mu.Unlock()
	var once sync.Once
	stop := func() error {
		once.Do(func() {
			f.mu.Lock()
			delete(subs, ch)
			f.mu.Unlock()
			close(ch)
		})
		return nil
	}
	context.AfterFunc(ctx, func() { _ = stop() })
	return ch, stop, nil
}

func tickAll(subs map[chan struct{}]struct{}) {
	for ch := range subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
