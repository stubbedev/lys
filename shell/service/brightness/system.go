package brightness

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stubbedev/lys/desktop/service/backlight"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/udevmonitor"
)

// hotplugSettle is the pause before a hotplug-triggered DDC scan, so
// DRM has settled and the monitor's I²C bus answers the probe.
const hotplugSettle = 500 * time.Millisecond

// subsystems are the devices whose events re-scan DDC.
var subsystems = []string{"drm"}

// listenUevents opens the socket udevd broadcasts DRM hotplug on; a var
// so tests can inject events.
var listenUevents = udevmonitor.Listen

func isDRMHotplug(ev udevmonitor.Event) bool {
	return ev.Subsystem == "drm" && ev.Action == udevmonitor.Change && ev.Props["HOTPLUG"] == "1"
}

// System is the real source: the kernel's backlights, plus, when
// external monitors are enabled, DDC/CI monitors found by a detached
// scan and re-scanned on DRM hotplug.
type System struct {
	sysfs *backlight.Sysfs
	ddc   atomic.Pointer[ddcManager]
	ticks *backlight.Broadcast

	mu        sync.Mutex
	pending   map[string]uint32
	requested map[string]uint32

	events *udevmonitor.Listener
	stop   chan struct{}
	done   sync.WaitGroup
	settle time.Duration
}

// NewSystem starts the source. external enables DDC/CI discovery
// (the enable-external key); the first scan runs detached so internal
// panels are usable at once. system is the system bus logind's writes
// go over, and stays the caller's; nil writes through sysfs. A missing
// udev socket degrades to no DDC hotplug instead of failing.
func NewSystem(system *dbus.Conn, external bool) *System {
	s := &System{
		sysfs:     backlight.NewSysfs(system),
		ticks:     backlight.NewBroadcast(),
		pending:   map[string]uint32{},
		requested: map[string]uint32{},
		stop:      make(chan struct{}),
		settle:    hotplugSettle,
	}
	s.start()
	s.SetExternal(external)
	return s
}

// SetExternal follows the enable-external key: on starts a detached DDC
// scan, off closes every i2c bus held and scans no more until it is on
// again.
func (s *System) SetExternal(on bool) {
	if !on {
		if m := s.ddc.Swap(nil); m != nil {
			m.close()
			s.ticks.Tick()
		}
		return
	}
	if m := newDDCManager(); s.ddc.CompareAndSwap(nil, m) {
		s.scanDDC(m, 0)
	}
}

func (s *System) start() {
	backlights, stopBacklights, err := s.sysfs.Subscribe(context.Background())
	if err != nil {
		panic(fmt.Errorf("brightness: watch the backlights just started: %w", err))
	}
	if reader, err := listenUevents(subsystems); err == nil {
		s.events = reader
	} else {
		log.Printf("brightness: no DDC hotplug: %v", err)
	}
	s.done.Go(func() {
		defer stopBacklights()
		s.loop(backlights)
	})
}

// Close stops the watchers and waits for the goroutines; later calls
// are no-ops.
func (s *System) Close() error {
	select {
	case <-s.stop:
		return nil
	default:
	}
	close(s.stop)
	s.ticks.Close()
	if s.events != nil {
		_ = s.events.Close()
	}
	err := s.sysfs.Close()
	s.done.Wait()
	return err
}

func (s *System) forwardHotplugs(out chan<- struct{}) {
	for {
		ev, err := s.events.Next()
		switch {
		case errors.Is(err, udevmonitor.ErrLost):
			log.Printf("brightness: %v", err)
		case errors.Is(err, os.ErrClosed):
			return
		case err != nil:
			log.Printf("brightness: no DDC hotplug: %v", err)
			return
		case !isDRMHotplug(ev):
			continue
		}
		select {
		case out <- struct{}{}:
		case <-s.stop:
			return
		}
	}
}

// loop passes the backlights' ticks on and re-scans DDC on DRM hotplug.
func (s *System) loop(backlights <-chan struct{}) {
	hotplug := make(chan struct{})
	if s.events != nil {
		s.done.Go(func() { s.forwardHotplugs(hotplug) })
	}
	for {
		select {
		case <-s.stop:
			return
		case _, ok := <-backlights:
			if !ok {
				backlights = nil
				continue
			}
			s.ticks.Tick()
		case <-hotplug:
			if m := s.ddc.Load(); m != nil {
				s.scanDDC(m, s.settle)
			}
		}
	}
}

// scanDDC re-scans detached after settle and ticks subscribers when
// the monitor set changed.
func (s *System) scanDDC(m *ddcManager, settle time.Duration) {
	s.done.Go(func() {
		if settle > 0 {
			select {
			case <-time.After(settle):
			case <-s.stop:
				return
			}
		}
		added, removed := m.refresh()
		for _, name := range added {
			log.Printf("brightness: external DDC monitor connected: %s", name)
		}
		for _, name := range removed {
			log.Printf("brightness: external DDC monitor disconnected: %s", name)
		}
		if len(added)+len(removed) > 0 {
			s.ticks.Tick()
		}
	})
}

// Devices reads every sysfs device, then the DDC monitors from their
// cached levels (a DDC read is too slow to make per refresh), each at
// the level a queued write is heading to.
func (s *System) Devices(ctx context.Context) ([]Device, error) {
	devices, err := s.sysfs.Devices(ctx)
	if err != nil {
		return nil, err
	}
	m := s.ddc.Load()
	if m == nil {
		return devices, nil
	}
	monitors := m.devices()
	s.mu.Lock()
	for i, d := range monitors {
		if level, ok := s.requested[d.Name]; ok {
			monitors[i].Brightness = min(level, d.Max)
		}
	}
	s.mu.Unlock()
	return append(devices, monitors...), nil
}

// Set applies a clamped percentage. A DDC monitor's write is handed to
// its writer goroutine and Set returns at once: DDC I/O takes tens of
// milliseconds a transaction, and a slider drag coalesces to the
// latest value instead of queueing every step. Write failures are
// logged.
func (s *System) Set(ctx context.Context, name string, percent float64) error {
	if m := s.ddc.Load(); m != nil {
		if d, ok := m.display(name); ok {
			s.queueDDC(m, name, backlight.RawLevel(percent, d.max))
			return nil
		}
	}
	return s.sysfs.Set(ctx, name, percent)
}

// queueDDC records the latest target, reports it at once, and starts
// the monitor's writer if none is running. DDC has no kernel
// notification, so the writer reports again when it settles, which
// shows the monitor's real level after a failed write.
func (s *System) queueDDC(m *ddcManager, name string, value uint32) {
	s.mu.Lock()
	s.pending[name] = value
	_, running := s.requested[name]
	s.requested[name] = value
	s.mu.Unlock()
	s.ticks.Tick()
	if running {
		return
	}
	s.done.Go(func() {
		for {
			s.mu.Lock()
			target, ok := s.pending[name]
			delete(s.pending, name)
			if !ok {
				delete(s.requested, name)
				s.mu.Unlock()
				s.ticks.Tick()
				return
			}
			s.mu.Unlock()
			if err := m.setRaw(name, target); err != nil {
				log.Printf("%v", err)
			}
		}
	})
}

// Subscribe registers for change ticks until ctx ends, stop or Close.
func (s *System) Subscribe(ctx context.Context) (<-chan struct{}, func(), error) {
	return s.ticks.Subscribe(ctx)
}
