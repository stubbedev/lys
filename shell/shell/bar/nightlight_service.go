package bar

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/geo"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/identity"
	"github.com/stubbedev/lys/desktop/tzdb"
	"github.com/stubbedev/lys/lib/xdg"

	"github.com/stubbedev/lys/shell/internal/clockwatch"
	"github.com/stubbedev/lys/shell/service/nightlight"
)

const (
	nightlightScheduleTick = time.Minute
	nightlightZoneCheck    = 5 * time.Minute
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
	after      func(d time.Duration, fn func()) (stop func() bool)
	stateDir   string
	locate     func() (geo.Coordinates, error)
	clockJumps func(ctx context.Context, onJump func()) error
}

func liveSunsetEnv(vars xdg.Env) (sunsetEnv, error) {
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
		now:        time.Now,
		after:      func(d time.Duration, fn func()) func() bool { return time.AfterFunc(d, fn).Stop },
		stateDir:   dir,
		locate:     func() (geo.Coordinates, error) { return nightlight.Locate(tzdb.System) },
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

	previews    uint64
	stopPreview func() bool

	location    *geo.Coordinates
	locationErr error
	pinned      bool

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
	env, err := liveSunsetEnv(xdg.OS)
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
	if place, pinned := cfg.Location.Pinned(); followsTheSun(cfg) && pinned {
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
		restored, hasRestored := nightlight.LoadOverride(env.stateDir, env.now())
		s.onLoop(func() {
			s.connected = true
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
	if s.ctx.NightlightBus != nil {
		s.ctx.NightlightBus.Show(s.showing())
	}
	for _, w := range s.watchers {
		w.change()
	}
}

func (s *nightlightService) showing() bool { return s.enabled && s.refused == nil }

// refusal is what keeps the filter off, in the shell's words: a
// configured strength out of range, or a display that refused it.
func (s *nightlightService) refusal() string {
	if s.nightErr != nil && errors.Is(s.refused, s.nightErr) {
		return i18n.T("dropdown-brightness-nightlight-invalid")
	}
	return i18n.T("bar-nightlight-refused")
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

func (s *nightlightService) run() {
	s.life, s.cancel = context.WithCancel(s.ctx.Life())
	s.ctx.Invoke(s.tick)
	if !s.cfg.AutoSchedule {
		return
	}
	go s.followClock(s.life)
	if followsTheSun(s.cfg) && !s.pinned {
		go s.followLocation(s.life)
	}
}

func followsTheSun(cfg config.NightlightConfig) bool {
	return cfg.AutoSchedule && cfg.Schedule == config.NightlightScheduleSun
}

func (s *nightlightService) daylight() (nightlight.Daylight, bool) {
	if s.cfg.Schedule == config.NightlightScheduleCustom {
		return nightlight.HoursOf(s.cfg.CustomHours.Hours()), true
	}
	if s.location == nil {
		return nil, false
	}
	return nightlight.SunAt(*s.location), true
}

func (s *nightlightService) followClock(ctx context.Context) {
	err := s.env.clockJumps(ctx, func() { s.ctx.Invoke(s.tick) })
	if err != nil {
		log.Printf("nightlight: a resume can lag by up to %s: %v", nightlightScheduleTick, err)
	}
}

func (s *nightlightService) followLocation(ctx context.Context) {
	check := time.NewTicker(nightlightZoneCheck)
	defer check.Stop()
	for {
		loc, err := s.env.locate()
		switch {
		case ctx.Err() != nil:
			return
		case err != nil:
			s.ctx.Invoke(func() { s.lookupFailed(err) })
		default:
			s.ctx.Invoke(func() { s.setLocation(loc) })
		}
		select {
		case <-ctx.Done():
			return
		case <-check.C:
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
	if s.life == nil || s.life.Err() != nil || !s.cfg.AutoSchedule {
		return
	}
	days, ok := s.daylight()
	if !ok {
		return
	}
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(s.nextTick(days), func() { s.ctx.Invoke(s.tick) })
}

func (s *nightlightService) nextTick(days nightlight.Daylight) time.Duration {
	if s.stepping && s.filter != nil {
		return s.filter.EaseStep()
	}
	now := s.env.now()
	next, ok := nightlight.NextEase(now, days, s.cfg.TransitionDuration.Transition())
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
	if s.stopPreview != nil {
		s.stopPreview()
	}
	s.onOwner(func() {
		if s.filter != nil {
			s.filter.Close()
		}
	})
}

// preview shows temp at the configured gamma for PreviewLength, then
// the display goes back to what the night light shows; a later preview
// restarts the wait.
func (s *nightlightService) preview(temp int) {
	setting, err := nightlight.NewSetting(temp, int(s.cfg.Gamma))
	if err != nil {
		log.Printf("nightlight: preview: %v", err)
		return
	}
	if s.stopPreview != nil {
		s.stopPreview()
	}
	s.previews++
	n := s.previews
	s.onOwner(func() {
		if s.filter == nil {
			return
		}
		if err := s.filter.Apply(setting); err != nil {
			log.Printf("nightlight: preview %dK: %v", temp, err)
		}
	})
	s.stopPreview = s.env.after(nightlight.PreviewLength, func() { s.onLoop(func() { s.endPreview(n) }) })
}

// endPreview puts back what the night light shows, unless a later
// preview took over.
func (s *nightlightService) endPreview(n uint64) {
	if n != s.previews {
		return
	}
	s.stopPreview = nil
	on, live := s.showing(), s.live
	s.onOwner(func() {
		if s.filter == nil {
			return
		}
		apply := s.filter.Clear
		if on {
			apply = func() error { return s.filter.Apply(live) }
		}
		if err := apply(); err != nil {
			log.Printf("nightlight: end the preview: %v", err)
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
	s.locationErr = nil
	if s.location != nil && *s.location == loc {
		return
	}
	s.location = &loc
	log.Printf("nightlight: following the sun at the time zone's place, %s", loc)
	s.evaluate()
	s.armTick()
	s.notify()
}

func (s *nightlightService) lookupFailed(err error) {
	if !sameErrorText(err, s.locationErr) {
		log.Printf("nightlight: the location lookup failed: %v", err)
	}
	s.locationErr = err
	s.notify()
}

func (s *nightlightService) noLocation() error {
	if !followsTheSun(s.cfg) || s.location != nil {
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
	days, ok := s.daylight()
	if !ok || s.nightErr != nil {
		return
	}
	target := nightlight.TargetAt(s.env.now(), days, s.cfg.TransitionDuration.Transition(), s.night)
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
		if days, ok := s.daylight(); s.autoPhase == 0 && ok {
			s.autoPhase = nightlight.TargetAt(s.env.now(), days, s.cfg.TransitionDuration.Transition(), s.night).Phase
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
