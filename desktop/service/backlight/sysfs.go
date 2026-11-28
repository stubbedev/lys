package backlight

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/logind"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/udevmonitor"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/desktop/fswatch"
)

// driftInterval is how often the levels are compared, the correction for
// changes no watch or uevent reports (firmware hotkeys that never touch
// the attribute through a write); a var so tests can quiet it.
var driftInterval = time.Second

// subsystems are the devices whose events the watch reads.
var subsystems = []string{"backlight"}

// listenUevents opens the socket udevd broadcasts backlight events on;
// a var so tests can inject events.
var listenUevents = udevmonitor.Listen

// Sysfs is the backlights of a Class as a Source: read from sysfs,
// written through logind with a sysfs fallback, and watched through
// inotify, udevd's backlight events and a drift probe.
type Sysfs struct {
	class  Class
	logind *logindBacklight

	writing sync.Mutex
	ticks   *Broadcast
	watch   *fswatch.Watcher
	events  *udevmonitor.Listener
	stop    chan struct{}
	done    sync.WaitGroup
}

// NewSysfs starts the watch of class. bus is the system bus logind's
// writes go over, and stays the caller's; nil writes through sysfs
// alone. A missing inotify or uevent socket degrades to the drift probe
// instead of failing.
func NewSysfs(class Class, bus *dbus.Conn) *Sysfs {
	s := &Sysfs{class: class, ticks: NewBroadcast(), stop: make(chan struct{})}
	if bus != nil {
		s.logind = &logindBacklight{manager: logind.NewManager(bus)}
	}
	if w, err := fswatch.New(unix.IN_MODIFY, false); err == nil {
		s.watch = w
	} else {
		log.Printf("backlight: %v", err)
	}
	if reader, err := listenUevents(subsystems); err == nil {
		s.events = reader
	} else {
		log.Printf("backlight: no hotplug: %v", err)
	}
	probe := &levelProbe{}
	s.rescan(probe)
	s.done.Go(func() { s.loop(probe) })
	return s
}

// Close stops the watch and waits for its goroutines; later calls are
// no-ops.
func (s *Sysfs) Close() error {
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
	if s.watch != nil {
		_ = s.watch.Close()
	}
	s.done.Wait()
	return nil
}

// rescan looks at the devices present: a watch per device directory
// (adding an existing one is a no-op for inotify) and the drift probe.
func (s *Sysfs) rescan(probe *levelProbe) {
	entries, err := os.ReadDir(string(s.class))
	if err != nil {
		log.Printf("backlight: rescan: %v", err)
		entries = nil
	}
	if s.watch != nil {
		for _, entry := range entries {
			_ = s.watch.Add(s.class.device(entry.Name()))
		}
	}
	probe.open(s.class, entries)
}

// rescanOn says whether ev reports a backlight that came or went, which
// rescans, or one whose level changed, which reads the levels; ok is
// false for neither.
func rescanOn(ev udevmonitor.Event) (rescan, ok bool) {
	if ev.Subsystem != "backlight" {
		return false, false
	}
	switch ev.Action {
	case udevmonitor.Add, udevmonitor.Remove:
		return true, true
	case udevmonitor.Change:
		return false, true
	}
	return false, false
}

func (s *Sysfs) forwardUevents(out chan<- bool) {
	for {
		ev, err := s.events.Next()
		rescan, ok := rescanOn(ev)
		switch {
		case errors.Is(err, udevmonitor.ErrLost):
			log.Printf("backlight: %v", err)
			rescan, ok = true, true
		case errors.Is(err, os.ErrClosed):
			return
		case err != nil:
			log.Printf("backlight: no hotplug: %v", err)
			return
		}
		if !ok {
			continue
		}
		select {
		case out <- rescan:
		case <-s.stop:
			return
		}
	}
}

func (s *Sysfs) loop(probe *levelProbe) {
	var changed <-chan struct{}
	if s.watch != nil {
		changed = s.watch.Changed()
	}
	events := make(chan bool)
	if s.events != nil {
		s.done.Go(func() { s.forwardUevents(events) })
	}
	defer probe.close()
	drift := time.NewTicker(driftInterval)
	defer drift.Stop()
	armDrift := func() {
		if probe.empty() && s.events != nil {
			drift.Stop()
			return
		}
		drift.Reset(driftInterval)
	}
	armDrift()
	for {
		select {
		case <-s.stop:
			return
		case <-changed:
			if _, err := probe.moved(); err != nil {
				s.rescan(probe)
				armDrift()
			}
			s.ticks.Tick()
		case <-drift.C:
			moved, err := probe.moved()
			if err != nil || probe.empty() {
				count := len(probe.levels)
				s.rescan(probe)
				armDrift()
				moved = err != nil || len(probe.levels) != count
			}
			if moved {
				s.ticks.Tick()
			}
		case rescan := <-events:
			if rescan {
				s.rescan(probe)
				armDrift()
			} else if _, err := probe.moved(); err != nil {
				s.rescan(probe)
				armDrift()
			}
			s.ticks.Tick()
		}
	}
}

// Devices reads every device of the class.
func (s *Sysfs) Devices(context.Context) ([]Device, error) {
	return s.class.Enumerate(), nil
}

// Subscribe ticks whenever a level may have changed or a device came or
// went, until ctx ends, stop or Close.
func (s *Sysfs) Subscribe(ctx context.Context) (<-chan struct{}, func(), error) {
	return s.ticks.Subscribe(ctx)
}

// Set applies a clamped percentage through logind, and writes sysfs
// directly when logind refuses or there is no system bus.
func (s *Sysfs) Set(ctx context.Context, name string, percent float64) error {
	device, err := s.class.ReadDevice(name)
	if err != nil {
		return err
	}
	value := RawLevel(percent, device.Max)
	s.writing.Lock()
	defer s.writing.Unlock()
	if s.logind == nil {
		return s.class.write(name, value)
	}
	viaLogind := s.logind.set(ctx, name, value)
	if viaLogind == nil {
		return nil
	}
	if err := s.class.write(name, value); err != nil {
		return errors.Join(viaLogind, err)
	}
	return nil
}
