package bar

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/geo"
	"github.com/stubbedev/lys/desktop/identity"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/xdg"

	"github.com/stubbedev/lys/shell/internal/clockwatch"
	"github.com/stubbedev/lys/shell/service/nightlight"
)

const (
	nightlightScheduleTick  = time.Minute
	nightlightLocationEach  = 6 * time.Hour
	nightlightLocationRetry = 5 * time.Minute
	nightlightLookupTimeout = 30 * time.Second
)

const nightlightTemperaturePath = "modules.nightlight.temperature"

type colourFilter interface {
	Protocol() string
	EaseStep() time.Duration
	Apply(nightlight.Setting) error
	Clear() error
	Refused() error
	Close()
}

type sunsetEnv struct {
	open       func(changed func()) (colourFilter, error)
	now        func() time.Time
	stateDir   string
	locate     func(ctx context.Context) (geo.Coordinates, error)
	clockJumps func(ctx context.Context, onJump func()) error
}

func liveSunsetEnv(vars xdg.Env, system *dbus.Conn) (sunsetEnv, error) {
	dir, ok := vars.StateDir(identity.Name)
	if !ok {
		return sunsetEnv{}, errors.New("nightlight: no state directory: neither XDG_STATE_HOME nor HOME is an absolute path")
	}
	return sunsetEnv{
		open: func(changed func()) (colourFilter, error) {
			f, err := nightlight.Open(changed)
			if err != nil {
				return nil, err
			}
			return f, nil
		},
		now:      time.Now,
		stateDir: dir,
		locate: func(ctx context.Context) (geo.Coordinates, error) {
			if system == nil {
				return geo.Coordinates{}, errors.New("geoclue: the shell has no system bus connection")
			}
			return nightlight.QueryLocation(ctx, system)
		},
		clockJumps: clockwatch.Watch,
	}, nil
}

type nightlightService struct {
	ctx    ModuleContext
	env    sunsetEnv
	cfg    config.NightlightConfig
	filter colourFilter

	unavailable error
	connected   bool

	night    nightlight.Setting
	nightErr error

	savedTemp uint32
	saveErr   error
	saves     uint64

	enabled         bool
	live            nightlight.Setting
	refused         error
	autoPhase       nightlight.Phase
	manualOverride  bool
	heldThroughEase bool
	stepping        bool

	location    *geo.Coordinates
	locationErr error
	pinned      bool
	lookupErr   string

	watchers  []nightlightWatcher
	nextWatch int

	life   context.Context
	cancel context.CancelFunc
	timer  *time.Timer
}

type nightlightWatcher struct {
	id     int
	change func()
}

func startNightlight(ctx ModuleContext) *nightlightService {
	env, err := liveSunsetEnv(xdg.OS, ctx.System)
	if err != nil {
		log.Print(err)
		return &nightlightService{ctx: ctx, cfg: ctx.Config.Nightlight, unavailable: err}
	}
	return newNightlightService(ctx, env)
}

func newNightlightService(ctx ModuleContext, env sunsetEnv) *nightlightService {
	cfg := ctx.Config.Nightlight
	s := &nightlightService{ctx: ctx, env: env, cfg: cfg, savedTemp: cfg.Temperature}
	ctx.OnRetire(s.stop)
	if s.night, s.nightErr = nightlight.NewSetting(int(cfg.Temperature), int(cfg.Gamma)); s.nightErr != nil {
		log.Print(s.nightErr)
		s.refused = s.nightErr
	}
	if place, pinned := cfg.Location.Pinned(); cfg.AutoSchedule && pinned {
		at := place.Coordinates()
		s.pinned, s.location = true, &at
	}
	s.onOwner(func() {
		filter, err := env.open(func() { s.onOwner(s.readRefusal) })
		if err != nil {
			log.Print(err)
			s.onLoop(func() {
				s.unavailable = err
				s.notify()
			})
			return
		}
		s.filter = filter
		var remembered *geo.Coordinates
		if cfg.AutoSchedule && !s.pinned {
			remembered = s.readRememberedLocation()
		}
		restored, hasRestored := nightlight.LoadOverride(env.stateDir, env.now())
		s.onLoop(func() {
			s.connected = true
			if remembered != nil && s.location == nil {
				s.location = remembered
			}
			if hasRestored {
				s.autoPhase, s.manualOverride = restored.Phase, true
				if restored.Enabled {
					s.startFilter()
				}
			}
			s.notify()
			if ctx.App != nil {
				s.run()
			}
		})
	})
	return s
}

func (s *nightlightService) watch(change func()) (stop func()) {
	s.nextWatch++
	id := s.nextWatch
	s.watchers = append(s.watchers, nightlightWatcher{id: id, change: change})
	return func() {
		for i, w := range s.watchers {
			if w.id == id {
				s.watchers = append(s.watchers[:i:i], s.watchers[i+1:]...)
				return
			}
		}
	}
}

func (s *nightlightService) notify() {
	for _, w := range s.watchers {
		w.change()
	}
}

func (s *nightlightService) showing() bool { return s.enabled && s.refused == nil }

func (s *nightlightService) problem() error {
	if s.unavailable != nil {
		return s.unavailable
	}
	return s.refused
}

var nightlightOwner serialWorker

func (s *nightlightService) onOwner(job func()) {
	if s.ctx.App == nil {
		job()
		return
	}
	nightlightOwner.do(job)
}

func (s *nightlightService) onLoop(fn func()) {
	if s.ctx.App == nil {
		fn()
		return
	}
	s.ctx.Invoke(fn)
}

type serialWorker struct {
	mu      sync.Mutex
	queue   []func()
	running bool
}

func (w *serialWorker) do(job func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.queue = append(w.queue, job)
	if !w.running {
		w.running = true
		go w.drain()
	}
}

func (w *serialWorker) drain() {
	for {
		w.mu.Lock()
		if len(w.queue) == 0 {
			w.running = false
			w.mu.Unlock()
			return
		}
		job := w.queue[0]
		w.queue = w.queue[1:]
		w.mu.Unlock()
		job()
	}
}

func (s *nightlightService) readRememberedLocation() *geo.Coordinates {
	remembered, err := nightlight.LoadLocation(s.env.stateDir)
	switch {
	case err == nil:
		return &remembered
	case !errors.Is(err, fs.ErrNotExist):
		log.Print(err)
	}
	return nil
}

func (s *nightlightService) run() {
	s.life, s.cancel = context.WithCancel(s.ctx.Life())
	s.ctx.Invoke(s.tick)
	if !s.cfg.AutoSchedule {
		return
	}
	go s.followClock(s.life)
	if !s.pinned {
		go s.followLocation(s.life)
	}
}

func (s *nightlightService) followClock(ctx context.Context) {
	err := s.env.clockJumps(ctx, func() { s.ctx.Invoke(s.tick) })
	if err != nil {
		log.Printf("nightlight: a resume can lag by up to %s: %v", nightlightScheduleTick, err)
	}
}

func (s *nightlightService) followLocation(ctx context.Context) {
	for {
		lookup, done := context.WithTimeout(ctx, nightlightLookupTimeout)
		loc, err := s.env.locate(lookup)
		done()
		if ctx.Err() != nil {
			return
		}
		wait := nightlightLocationEach
		if err != nil {
			wait = nightlightLocationRetry
			s.ctx.Invoke(func() { s.lookupFailed(err) })
		} else {
			s.ctx.Invoke(func() { s.setLocation(loc) })
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

func (s *nightlightService) tick() {
	if s.life.Err() != nil {
		return
	}
	s.evaluate()
	s.armTick()
}

func (s *nightlightService) armTick() {
	if s.life == nil || s.life.Err() != nil || !s.cfg.AutoSchedule || s.location == nil {
		return
	}
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(s.nextTick(), func() { s.ctx.Invoke(s.tick) })
}

func (s *nightlightService) nextTick() time.Duration {
	if s.stepping && s.filter != nil {
		return s.filter.EaseStep()
	}
	now := s.env.now()
	next, ok := nightlight.NextEase(now, *s.location, s.cfg.TransitionDuration.Transition())
	if !ok {
		return nightlightScheduleTick
	}
	return min(nightlightScheduleTick, next.Sub(now))
}

func (s *nightlightService) stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.timer != nil {
		s.timer.Stop()
	}
	s.onOwner(func() {
		if s.filter != nil {
			s.filter.Close()
		}
	})
}

func (s *nightlightService) readRefusal() {
	if s.filter == nil {
		return
	}
	refused := s.filter.Refused()
	s.onLoop(func() { s.refreshRefusal(refused) })
}

func (s *nightlightService) refreshRefusal(refused error) {
	if !s.enabled {
		return
	}
	if errors.Is(refused, s.refused) {
		return
	}
	if refused != nil {
		log.Print(refused)
	}
	s.refused = refused
	s.notify()
}

func (s *nightlightService) setLocation(loc geo.Coordinates) {
	s.lookupErr, s.locationErr = "", nil
	if s.location != nil && *s.location == loc {
		return
	}
	s.location = &loc
	log.Printf("nightlight: the location changed, following GeoClue")
	s.onOwner(func() {
		if err := nightlight.SaveLocation(s.env.stateDir, loc); err != nil {
			log.Print(err)
		}
	})
	s.evaluate()
	s.armTick()
	s.notify()
}

func (s *nightlightService) lookupFailed(err error) {
	if text := err.Error(); text != s.lookupErr {
		s.lookupErr = text
		log.Printf("nightlight: the location lookup failed: %v", err)
	}
	s.locationErr = err
	s.notify()
}

func (s *nightlightService) noLocation() error {
	if !s.cfg.AutoSchedule || s.location != nil {
		return nil
	}
	return s.locationErr
}

func (s *nightlightService) evaluate() {
	if !s.cfg.AutoSchedule {
		s.autoPhase, s.manualOverride = 0, false
		return
	}
	s.stepping = false
	if s.location == nil || s.nightErr != nil {
		return
	}
	target := nightlight.TargetAt(s.env.now(), *s.location, s.cfg.TransitionDuration.Transition(), s.night)
	switch {
	case s.manualOverride && s.autoPhase == 0:
		s.autoPhase = target.Phase
	case s.autoPhase != target.Phase:
		s.heldThroughEase = s.manualOverride && s.enabled == (target.Phase == nightlight.PhaseNight)
		s.autoPhase = target.Phase
		s.manualOverride = false
		s.onOwner(func() {
			if err := nightlight.ClearOverride(s.env.stateDir); err != nil {
				log.Printf("%v: a restart can replay the ended toggle", err)
			}
		})
	}
	if s.manualOverride {
		return
	}
	if s.heldThroughEase && target.Easing {
		return
	}
	s.heldThroughEase = false
	s.stepping = target.Easing
	s.show(target.Setting)
}

func (s *nightlightService) show(target nightlight.Setting) {
	switch {
	case target.IsIdentity():
		if s.enabled {
			s.stopFilter()
		}
	case !s.enabled || (s.refused == nil && s.live != target):
		s.applyFilter(target)
	}
}

func (s *nightlightService) toggle() { s.switchTo(!s.enabled) }

func (s *nightlightService) setEnabled(on bool) {
	if on == s.showing() {
		return
	}
	s.switchTo(on)
}

func (s *nightlightService) switchTo(on bool) {
	if s.cfg.AutoSchedule {
		s.manualOverride, s.stepping = true, false
		if s.autoPhase == 0 && s.location != nil {
			s.autoPhase = nightlight.TargetAt(s.env.now(), *s.location, s.cfg.TransitionDuration.Transition(), s.night).Phase
		}
	}
	override, at := nightlight.Override{Phase: s.autoPhase, Enabled: on}, s.env.now()
	s.onOwner(func() {
		if err := nightlight.SaveOverride(s.env.stateDir, override, at); err != nil {
			log.Printf("%v: the toggle will not survive a restart", err)
		}
	})
	if on {
		s.startFilter()
		return
	}
	s.stopFilter()
}

func (s *nightlightService) setTemperature(kelvin uint32) error {
	night, err := nightlight.NewSetting(int(kelvin), int(s.cfg.Gamma))
	if err != nil {
		return err
	}
	s.saveErr = nil
	s.applyTemperature(kelvin, night)
	s.saves++
	save := s.saves
	s.ctx.setConfig(nightlightTemperaturePath, int64(kelvin), func(err error) { s.temperatureSaved(save, kelvin, err) })
	return nil
}

func (s *nightlightService) temperatureSaved(save uint64, kelvin uint32, err error) {
	if err == nil {
		s.savedTemp = kelvin
		return
	}
	log.Print(err)
	if save != s.saves {
		return
	}
	s.saveErr = err
	if s.cfg.Temperature != kelvin || s.savedTemp == kelvin {
		s.notify()
		return
	}
	restore := s.cfg
	restore.Temperature = s.savedTemp
	s.reconfigure(restore)
}

func (s *nightlightService) reconfigure(cfg config.NightlightConfig) {
	if cfg.Temperature == s.cfg.Temperature {
		return
	}
	s.savedTemp = cfg.Temperature
	night, err := nightlight.NewSetting(int(cfg.Temperature), int(cfg.Gamma))
	if err == nil {
		s.applyTemperature(cfg.Temperature, night)
		return
	}
	log.Print(err)
	s.cfg.Temperature, s.nightErr = cfg.Temperature, err
	if s.enabled {
		s.stopFilter()
		return
	}
	s.refused = err
	s.notify()
}

func (s *nightlightService) applyTemperature(kelvin uint32, night nightlight.Setting) {
	s.cfg.Temperature = kelvin
	s.night, s.nightErr = night, nil
	switch {
	case !s.enabled:
		s.refused = nil
	case !s.stepping:
		s.applyFilter(night)
		return
	}
	s.notify()
}

func (s *nightlightService) startFilter() {
	if s.nightErr != nil {
		s.enabled, s.refused = true, s.nightErr
		s.notify()
		return
	}
	s.applyFilter(s.night)
}

func (s *nightlightService) applyFilter(setting nightlight.Setting) {
	wasOn := s.enabled
	s.enabled, s.refused, s.live = true, nil, setting
	s.notify()
	s.onOwner(func() {
		if s.filter == nil {
			return
		}
		err := s.filter.Apply(setting)
		if err == nil {
			err = s.filter.Refused()
		}
		if err != nil {
			log.Print(err)
			s.onLoop(func() {
				s.refused = err
				s.notify()
			})
			return
		}
		if !wasOn {
			log.Printf("nightlight: on at %dK %d%% through %s", setting.Temp(), setting.Gamma(), s.filter.Protocol())
		}
	})
}

func (s *nightlightService) stopFilter() {
	s.onOwner(func() {
		if s.filter == nil {
			return
		}
		if err := s.filter.Clear(); err != nil {
			log.Print(err)
		} else {
			log.Printf("nightlight: off")
		}
	})
	s.enabled, s.refused = false, s.nightErr
	s.notify()
}
