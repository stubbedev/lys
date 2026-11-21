package bar

import (
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/config"
)

type configStore interface {
	SetByPath(path string, value any) error
	Save() error
}

type configChange struct {
	path  string
	value any
	done  func(error)
}

// configWriter applies config changes in order on a goroutine of its own and
// saves once whatever queued meanwhile is applied, so neither the bus nor the
// fsync sits on the loop. Each change's done hears, on that goroutine, the
// error that refused or failed to save it, or nil once it is on disk.
type configWriter struct {
	store configStore

	mu      sync.Mutex
	queue   []configChange
	writing bool
	idle    chan struct{}
}

func (w *configWriter) set(path string, value any, done func(error)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.queue = append(w.queue, configChange{path, value, done})
	if !w.writing {
		w.writing = true
		w.idle = make(chan struct{})
		go w.write()
	}
}

func (w *configWriter) write() {
	var unsaved []func(error)
	for {
		w.mu.Lock()
		if len(w.queue) == 0 && len(unsaved) == 0 {
			w.writing = false
			close(w.idle)
			w.mu.Unlock()
			return
		}
		if len(w.queue) == 0 {
			w.mu.Unlock()
			err := w.store.Save()
			if err != nil {
				err = fmt.Errorf("config: save: %w", err)
			}
			for _, done := range unsaved {
				done(err)
			}
			unsaved = nil
			continue
		}
		next := w.queue[0]
		w.queue = w.queue[1:]
		w.mu.Unlock()
		if err := w.store.SetByPath(next.path, next.value); err != nil {
			next.done(fmt.Errorf("config: set %s: %w", next.path, err))
			continue
		}
		unsaved = append(unsaved, next.done)
	}
}

// flush waits for what was asked so far to reach the disk, or for timeout.
func (w *configWriter) flush(timeout time.Duration) {
	w.mu.Lock()
	idle := w.idle
	writing := w.writing
	w.mu.Unlock()
	if !writing {
		return
	}
	select {
	case <-idle:
	case <-time.After(timeout):
		log.Printf("config: a save did not finish in %v", timeout)
	}
}

// setConfig writes a runtime override through ctx.SetConfig and hands done
// its outcome on the loop: the error that refused or lost it, or nil once
// it is saved. done hears it even when the write itself remounted the
// bars and so retired the context's generation.
func (c ModuleContext) setConfig(path string, value any, done func(error)) {
	if c.SetConfig == nil {
		done(fmt.Errorf("config: set %s: the shell runs without a config service", path))
		return
	}
	savedOnLoop(c.SetConfig, c.onLoop)(path, value, done)
}

// savedOnLoop is set with each write's outcome handed to its done through
// invoke, which must run it on the loop whatever generation is mounted.
func savedOnLoop(set func(path string, value any, done func(error)), invoke func(func())) func(path string, value any, done func(error)) {
	return func(path string, value any, done func(error)) {
		set(path, value, func(err error) { invoke(func() { done(err) }) })
	}
}

// barsAffected reports whether a reload changes anything the bars
// render. The recorder's capture settings, which its dropdown writes,
// only shape the next recording: rebuilding the bars for them would
// close the dropdown being edited, and no bar renders the emoji skin tone
// the launcher keeps from a pick. The dropdown sizes, which a drag of a
// dropdown's edge writes, resize the built dropdowns in place
// (dropdownRegistry.reconfigure). The night light follows the strength
// the brightness dropdown writes in place. High contrast and reduce
// transparency restyle the open bars instead (accessibilityChanged).
func barsAffected(old, next *config.Config) bool {
	if old == nil || next == nil {
		return true
	}
	a, b := *old, *next
	for _, c := range []*config.Config{&a, &b} {
		c.Styling.HighContrast, c.Styling.ReduceTransparency = false, false
		r := &c.Recorder
		r.Microphone, r.MicrophoneDevice, r.SystemAudio = false, "", false
		r.WebcamEnabled, r.WebcamDevice = false, ""
		r.WebcamX, r.WebcamY = 0, 0
		c.Launcher.Emoji.SkinTone = ""
		c.Dropdowns = config.DropdownsConfig{}
		c.Nightlight.Temperature = 0
	}
	return !reflect.DeepEqual(a, b)
}

// accessibilityChanged reports a change of high contrast or reduce
// transparency, which restyles the open bars without rebuilding them.
func accessibilityChanged(old, next *config.Config) bool {
	return old.Styling.HighContrast != next.Styling.HighContrast || old.Styling.ReduceTransparency != next.Styling.ReduceTransparency
}
