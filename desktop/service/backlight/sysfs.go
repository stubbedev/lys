package backlight

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/logind"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/desktop/fswatch"
	"github.com/stubbedev/lys/desktop/uevent"
)

// driftInterval is how often the levels are compared, the correction for
// changes no watch or uevent reports (firmware hotkeys that never touch
// the attribute through a write); a var so tests can quiet it.
var driftInterval = time.Second

// listenUevents opens the uevent socket; a var so tests can inject
// events.
var listenUevents = uevent.Listen

// Sysfs is the backlights under Dir as a Source: read from sysfs,
// written through logind with a sysfs fallback, and watched through
// inotify, the kernel's backlight uevents and a drift probe.
type Sysfs struct {
	logind *logindBacklight

	writing sync.Mutex
	ticks   *Broadcast
	watch   *fswatch.Watcher
	events  uevent.Reader
	stop    chan struct{}
	done    sync.WaitGroup
}

// NewSysfs starts the watch. system is the system bus logind's writes go
// over, and stays the caller's; nil writes through sysfs alone. A
// missing inotify or uevent socket degrades to the drift probe instead
// of failing.
func NewSysfs(system *dbus.Conn) *Sysfs {
	s := &Sysfs{ticks: NewBroadcast(), stop: make(chan struct{})}
	if system != nil {
		s.logind = &logindBacklight{manager: logind.NewManager(system)}
	}
	if w, err := fswatch.New(unix.IN_MODIFY, false); err == nil {
		s.watch = w
	} else {
		log.Printf("backlight: %v", err)
	}
	if reader, err := listenUevents(); err == nil {
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
	entries, err := os.ReadDir(Dir)
	if err != nil {
		log.Printf("backlight: rescan: %v", err)
		entries = nil
	}
	if s.watch != nil {
		for _, entry := range entries {
			_ = s.watch.Add(filepath.Join(Dir, entry.Name()))
		}
	}
	probe.open(entries)
}

func isHotplug(ev uevent.Event) bool {
	return ev.Subsystem == "backlight" && (ev.Action == uevent.Add || ev.Action == uevent.Remove)
}

func isLevelChange(ev uevent.Event) bool {
	return ev.Subsystem == "backlight" && ev.Action == uevent.Change
}

func (s *Sysfs) forwardUevents(out chan<- uevent.Event) {
	for {
		ev, err := s.events.Next()
		if err != nil {
			return
		}
		if !isHotplug(ev) && !isLevelChange(ev) {
			continue
		}
		select {
		case out <- ev:
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
	events := make(chan uevent.Event)
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
		case ev := <-events:
			if isHotplug(ev) {
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

// Devices reads every device under Dir.
func (s *Sysfs) Devices(context.Context) ([]Device, error) {
	return Enumerate(), nil
}

// Subscribe ticks whenever a level may have changed or a device came or
// went, until ctx ends, stop or Close.
func (s *Sysfs) Subscribe(ctx context.Context) (<-chan struct{}, func(), error) {
	return s.ticks.Subscribe(ctx)
}

// Set applies a clamped percentage through logind, and writes sysfs
// directly when logind refuses or there is no system bus.
func (s *Sysfs) Set(ctx context.Context, name string, percent float64) error {
	device, err := ReadDevice(name)
	if err != nil {
		return err
	}
	value := RawLevel(percent, device.Max)
	s.writing.Lock()
	defer s.writing.Unlock()
	if s.logind == nil {
		return writeSysfs(name, value)
	}
	viaLogind := s.logind.set(ctx, name, value)
	if viaLogind == nil {
		return nil
	}
	if err := writeSysfs(name, value); err != nil {
		return errors.Join(viaLogind, err)
	}
	return nil
}
