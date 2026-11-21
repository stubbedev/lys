package bar

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"

	"github.com/stubbedev/lys/shell/internal/apptest"
)

func TestBarsAffected(t *testing.T) {
	base := config.Defaults()
	same := config.Defaults()
	if barsAffected(base, same) {
		t.Error("identical configs rebuild the bars")
	}
	capture := config.Defaults()
	capture.Recorder.Microphone = !base.Recorder.Microphone
	capture.Recorder.MicrophoneDevice = "mic0"
	capture.Recorder.WebcamX, capture.Recorder.WebcamY = 33, 66
	capture.Recorder.WebcamEnabled = !base.Recorder.WebcamEnabled
	if barsAffected(base, capture) {
		t.Error("recorder capture settings rebuild the bars")
	}
	tone := config.Defaults()
	tone.Launcher.Emoji.SkinTone = config.SkinToneDark
	if barsAffected(base, tone) {
		t.Error("the emoji skin tone picked in the launcher rebuilds the bars")
	}
	strength := config.Defaults()
	strength.Nightlight.Temperature = 3200
	if barsAffected(base, strength) {
		t.Error("the night light strength the brightness dropdown writes rebuilds the bars")
	}
	sized := config.Defaults()
	width := config.Px(480)
	sized.Dropdowns.Audio.Width = &width
	if barsAffected(base, sized) {
		t.Error("a dropdown size, which a drag of the dropdown writes, rebuilds the bars")
	}
	gamma := config.Defaults()
	gamma.Nightlight.Gamma = 80
	if !barsAffected(base, gamma) {
		t.Error("a night light gamma change does not rebuild the bars")
	}
	icon := config.Defaults()
	icon.Recorder.IconIdle = "other"
	if !barsAffected(base, icon) {
		t.Error("a recorder icon change does not rebuild the bars")
	}
	loc := config.Defaults()
	loc.Bar.Location = config.LocationBottom
	if !barsAffected(base, loc) {
		t.Error("a bar change does not rebuild the bars")
	}
	if !barsAffected(nil, base) {
		t.Error("no previous config must rebuild")
	}
	// The scrubbed fields are scrubbed on copies, not the live configs.
	if capture.Recorder.MicrophoneDevice != "mic0" {
		t.Error("barsAffected mutated its argument")
	}
}

func TestConfigSetterPersists(t *testing.T) {
	dir := t.TempDir()
	svc := config.Load(dir, config.DiscardDiagnostics)
	defer svc.Close()
	w := &configWriter{store: svc}
	set := func(path string, value any) error {
		var got error
		w.set(path, value, func(err error) { got = err })
		w.flush(10 * time.Second)
		return got
	}
	if err := set("modules.recorder.microphone-device", "mic0"); err != nil {
		t.Errorf("a good set reports %v", err)
	}
	if got := svc.Config().Recorder.MicrophoneDevice; got != "mic0" {
		t.Errorf("live value = %q", got)
	}
	body, err := os.ReadFile(filepath.Join(dir, "runtime.yaml"))
	if err != nil || !strings.Contains(string(body), "microphone-device: mic0\n") {
		t.Errorf("runtime.yaml = %q (%v)", body, err)
	}
	if err := set("modules.recorder.microphone-device", true); err == nil || !strings.HasPrefix(err.Error(), "config: set modules.recorder.microphone-device: ") {
		t.Errorf("a value of the wrong type reports %v, want the set naming it", err)
	}
	if got := svc.Config().Recorder.MicrophoneDevice; got != "mic0" {
		t.Errorf("after a bad set = %q", got)
	}
}

func TestSetConfigWithoutAServiceSaysSo(t *testing.T) {
	var got error
	called := false
	ModuleContext{}.setConfig("modules.recorder.microphone", true, func(err error) { got, called = err, true })
	if !called || got == nil || !strings.Contains(got.Error(), "modules.recorder.microphone") {
		t.Errorf("done called %v with %v, want an error naming the path", called, got)
	}
}

func TestSetConfigHandsTheOutcomeToTheLoop(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	failure := errors.New("config: save: disk full")
	for _, want := range []error{nil, failure} {
		ctx.SetConfig = func(_ string, _ any, done func(error)) { go done(want) }
		heard := make(chan error, 1)
		onHeadlessLoop(func() bool {
			ctx.setConfig("modules.nightlight.temperature", int64(3200), func(err error) { heard <- err })
			return true
		})
		if got := apptest.Answer(t, "the outcome on the loop", heard); !errors.Is(got, want) {
			t.Errorf("done heard %v, want %v", got, want)
		}
	}
}

func TestASaveIsHeardAfterItsOwnWriteRetiredTheGeneration(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	failure := errors.New("config: save: disk full")
	ctx.SetConfig = func(_ string, _ any, done func(error)) {
		ctx.gen.retire()
		go done(failure)
	}
	heard := make(chan error, 1)
	onHeadlessLoop(func() bool {
		ctx.setConfig(dockPathPinned, []any{"firefox"}, func(err error) { heard <- err })
		return true
	})
	if got := apptest.Answer(t, "the outcome of a write that remounted the bars", heard); !errors.Is(got, failure) {
		t.Errorf("done heard %v, want %v", got, failure)
	}
	ran := false
	ctx.Invoke(func() { ran = true })
	if ran {
		t.Error("work other than a save's outcome ran for the retired generation")
	}
}

type failingConfigStore struct {
	setErr  map[string]error
	saveErr error
	sets    []string
	saves   int
}

func (s *failingConfigStore) SetByPath(path string, _ any) error {
	if err := s.setErr[path]; err != nil {
		return err
	}
	s.sets = append(s.sets, path)
	return nil
}

func (s *failingConfigStore) Save() error {
	s.saves++
	return s.saveErr
}

func TestConfigWriterReportsEachChangesOutcome(t *testing.T) {
	refused := errors.New("unknown key")
	store := &failingConfigStore{setErr: map[string]error{"bad": refused}}
	w := &configWriter{store: store}
	outcomes := map[string]error{}
	var mu sync.Mutex
	for _, path := range []string{"a", "bad", "b"} {
		w.set(path, 1, func(err error) {
			mu.Lock()
			defer mu.Unlock()
			outcomes[path] = err
		})
	}
	w.flush(10 * time.Second)
	mu.Lock()
	defer mu.Unlock()
	if len(outcomes) != 3 {
		t.Fatalf("outcomes %v, want one for each change", outcomes)
	}
	if outcomes["a"] != nil || outcomes["b"] != nil {
		t.Errorf("saved changes report %v and %v, want nil", outcomes["a"], outcomes["b"])
	}
	if err := outcomes["bad"]; !errors.Is(err, refused) || !strings.HasPrefix(err.Error(), "config: set bad: ") {
		t.Errorf("the refused change reports %v, want the store's refusal naming it", err)
	}
	if !slices.Equal(store.sets, []string{"a", "b"}) || store.saves < 1 {
		t.Errorf("sets %v saves %d, want the good changes saved", store.sets, store.saves)
	}
}

func TestConfigWriterReportsAFailedSaveToEveryChangeItHeld(t *testing.T) {
	full := errors.New("no space left on device")
	store := &failingConfigStore{saveErr: full}
	w := &configWriter{store: store}
	var mu sync.Mutex
	var heard []error
	for _, path := range []string{"a", "b"} {
		w.set(path, 1, func(err error) {
			mu.Lock()
			defer mu.Unlock()
			heard = append(heard, err)
		})
	}
	w.flush(10 * time.Second)
	mu.Lock()
	defer mu.Unlock()
	if len(heard) != 2 {
		t.Fatalf("%d outcomes, want two", len(heard))
	}
	for _, err := range heard {
		if !errors.Is(err, full) || !strings.HasPrefix(err.Error(), "config: save: ") {
			t.Errorf("outcome %v, want the failed save", err)
		}
	}
}

type heldConfigStore struct {
	mu      sync.Mutex
	sets    []string
	saves   int
	entered chan struct{}
	release chan struct{}
}

func (s *heldConfigStore) SetByPath(path string, _ any) error {
	s.entered <- struct{}{}
	<-s.release
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sets = append(s.sets, path)
	return nil
}

func (s *heldConfigStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saves++
	return nil
}

func TestConfigWriterCoalescesSavesAndNeverHoldsItsCaller(t *testing.T) {
	store := &heldConfigStore{entered: make(chan struct{}, 8), release: make(chan struct{})}
	w := &configWriter{store: store}
	saved := func(err error) {
		if err != nil {
			t.Errorf("a held save failed: %v", err)
		}
	}
	w.set("a", 1, saved)
	apptest.Answer(t, "the first set reached the store", store.entered)
	w.set("b", 2, saved)
	close(store.release)
	w.flush(10 * time.Second)
	store.mu.Lock()
	defer store.mu.Unlock()
	if !slices.Equal(store.sets, []string{"a", "b"}) {
		t.Errorf("sets %v, want both in order", store.sets)
	}
	if store.saves != 1 {
		t.Errorf("%d saves for two sets, want one", store.saves)
	}
}
