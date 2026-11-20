package bar

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/region"
	"github.com/stubbedev/lys/desktop/service/bluetooth"
	"github.com/stubbedev/lys/desktop/service/clipboard"
	"github.com/stubbedev/lys/desktop/service/mpris"
	"github.com/stubbedev/lys/desktop/service/network"
	"github.com/stubbedev/lys/desktop/service/powerprofiles"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/desktop/service/upower"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	timeclock "github.com/stubbedev/lys/shell/internal/clock"
	"github.com/stubbedev/lys/shell/service/brightness"
	"github.com/stubbedev/lys/shell/service/calendar"
	"github.com/stubbedev/lys/shell/service/hyprland"
	"github.com/stubbedev/lys/shell/service/idle"
	"github.com/stubbedev/lys/shell/service/idleinhibit"
	"github.com/stubbedev/lys/shell/service/lyrics"
	"github.com/stubbedev/lys/shell/service/mail"
	"github.com/stubbedev/lys/shell/service/notifications"
	"github.com/stubbedev/lys/shell/service/recorder"
	"github.com/stubbedev/lys/shell/service/sni"
	"github.com/stubbedev/lys/shell/service/treeman"
	"github.com/stubbedev/lys/shell/service/weather"
	"github.com/stubbedev/lys/shell/shell/powermenu"
	"github.com/stubbedev/lys/shell/shell/screenshot"
)

// ModuleContext carries what a module needs at construction time: the
// resolved bar style, the config, and the loop it schedules timers on.
// App is nil only in headless construction (tests): the module then
// builds its tree without scheduling updates. Hyprland is nil when the
// compositor is not Hyprland; Hyprland-only modules error in that case.
type ModuleContext struct {
	Config     *config.Config
	App        *app.Application
	Font       render.Font
	Style      *barStyle
	Hyprland   *hyprland.Connection
	Battery    upower.Source
	Brightness brightness.Source
	Pulse      pulse.Source
	Media      mpris.Source
	Clock      *timeclock.Wall
	// Region is the session locale's units and clock, which settings
	// left to follow the region take.
	Region region.Prefs
	// Calendar holds the vdir calendars the calendar dropdown and the
	// clock read; nil in headless construction.
	Calendar *calendar.Store
	// Lyrics looks up the media dropdown's lyrics; nil when the cache
	// directory is unknown.
	Lyrics        *lyrics.Service
	Bluetooth     bluetooth.Source
	Network       network.Source
	Networking    *network.Service // secret agent, VPNs, wifi; nil without NM
	PowerProfiles powerprofiles.Source
	IdleInhibit   *idleinhibit.State
	// ScreenShares follows the portal's screen shares on the shell's
	// session bus; nil without one.
	ScreenShares shareSource
	// Cameras follows the processes holding a camera open; nil when the
	// video devices cannot be watched.
	Cameras cameraSource
	// Idle is the built-in idle policy, nil when it does not run.
	Idle          *idle.Service
	Treeman       treeman.Source
	Notifications *notifications.Service
	Recorder      *recorder.State
	Mail          *mail.Service
	// BackgroundApps is the portal's background apps, nil without a
	// session bus.
	BackgroundApps backgroundAppSource
	// AutoBrightness is lys-power's automatic brightness, nil without
	// a session bus.
	AutoBrightness autoBrightnessSource
	Nightlight     *nightlightService
	Bitwarden      *bitwardenHost
	SNI            *sni.Store
	Weather        *weather.Service
	// Tasks is the window list the minimized-windows modules and the docks
	// share; nil when the compositor offers no foreign-toplevel list.
	Tasks *taskHub
	// System is the shell's one system bus connection, nil without one;
	// what reads it never closes it.
	System *dbus.Conn
	// SetConfig writes a runtime override and persists it, off the loop,
	// then hands done the error that refused or lost it, or nil once it
	// is saved; nil without a config service. Modules write through
	// setConfig, which brings done back onto the loop.
	SetConfig func(path string, value any, done func(error))
	// Toast shows an OSD toast; nil without an OSD.
	Toast func(label, icon string)
	// Clipboard is the session clipboard history, nil when the
	// compositor has no data-control protocol (the launcher's
	// clipboard mode reads it).
	Clipboard *clipboard.Clipboard
	// Tray drives the tray items and their menus; nil without a host.
	Tray          TrayService
	CustomUpdates *customUpdates
	// PowerMenu is the native power menu the power module's :menu
	// opens and the dashboard's session buttons request; nil without
	// one, and then the dashboard has no session card.
	PowerMenu *powermenu.Menu
	// Dropdowns opens the dropdown:<name> popovers; RunWith owns one
	// registry across outputs.
	Dropdowns *dropdownRegistry
	// Attachers collects modules that need the live layer surface
	// (idle-inhibit binds its inhibitor to it); RunWith calls Attach
	// on each once the layer exists.
	Attachers *[]interface{ Attach(app.Host) }
	// gen is the mount generation the module belongs to; a config
	// reload retires it, so the old modules' scheduled work stops.
	gen *mountGen
	// Connector is the output this bar instance sits on; per-output
	// modules (workspaces) key their state on it.
	Connector string
	// Polls shares each config key's sampling across the bars and holds it
	// back while the session is idle or locked; nil samples alone, unpaused.
	Polls *pollHub
	// Screenshot starts a capture on the in-process screenshot host
	// (the `lys-shell screenshot ...` builtin); nil before the host is up.
	Screenshot func(mode, target string)
	// ReadScreen reads a dragged region's text or QR code (the `lys-shell
	// screenshot text|qr` builtin); nil before the host is up.
	ReadScreen func(kind screenshot.ReadKind)
	// PickColor picks a pixel of the screen and copies its color; nil
	// before the host is up.
	PickColor func()
	// Theme is the bar stylesheet every bar root attaches; nil in
	// headless construction, where the tree builds unstyled.
	Theme *apptheme.Theme
	// Alert replaces showAlert's modal dialog (tests answer it); nil
	// asks through the application.
	Alert func(spec alertSpec, answer func(accepted bool))
}

// headlessLoop stands in for the loop goroutine of a headless context
// (tests): work Invoke would marshal runs inline under it, so widget
// state is touched by one goroutine at a time either way.
var headlessLoop sync.Mutex

// Invoke marshals fn onto the loop goroutine, or for a headless context
// runs it inline under headlessLoop. A retired generation's work is
// dropped either way.
func (c ModuleContext) Invoke(fn func()) {
	c.onLoop(func() {
		if !c.gen.retiredNow() {
			fn()
		}
	})
}

// onLoop marshals fn onto the loop goroutine, or for a headless context
// runs it inline under headlessLoop, whether or not the context's
// generation has retired since.
func (c ModuleContext) onLoop(fn func()) {
	if c.App == nil {
		headlessLoop.Lock()
		defer headlessLoop.Unlock()
		fn()
		return
	}
	c.App.Invoke(fn)
}

// configKey names what a snapshot's module samples: bars of one snapshot share
// it, and a reload starts afresh.
func configKey(kind string, cfg *config.Config) string { return fmt.Sprintf("%s/%p", kind, cfg) }

// Every schedules fn on the application loop; a no-op when the context
// is headless.
func (c ModuleContext) Every(d time.Duration, fn func()) {
	if c.App == nil {
		return
	}
	c.App.Every(d, func() {
		if !c.gen.retiredNow() {
			fn()
		}
	})
}

// mountGen is one generation of mounted bars. A config reload retires
// it before mounting the next: the old modules' Invoke and Every work
// stops touching their (closed) trees, and their followed
// subscriptions end.
type mountGen struct {
	retired atomic.Bool
	once    sync.Once
	done    chan struct{}
	life    context.Context
	cancel  context.CancelFunc
	// releases run when the generation retires, before retire returns.
	releases []func()
	work     *sync.WaitGroup
}

func newMountGen() *mountGen { return newMountGenIn(&sync.WaitGroup{}) }

func newMountGenIn(work *sync.WaitGroup) *mountGen {
	life, cancel := context.WithCancel(context.Background())
	return &mountGen{done: make(chan struct{}), life: life, cancel: cancel, work: work}
}

func (g *mountGen) next() *mountGen { return newMountGenIn(g.work) }

// Life is the module's lifetime: canceled when its generation
// retires (a config reload), never for a context outside one. Module
// loops and subscriptions run under it.
func (c ModuleContext) Life() context.Context {
	if c.gen == nil {
		return context.Background()
	}
	return c.gen.life
}

// Go runs fn on a goroutine of the module's generation, which must end
// once the generation retires. The generations a runtime mounts share
// what they run, which waitWork waits for.
func (c ModuleContext) Go(fn func()) { c.gen.work.Go(fn) }

func (g *mountGen) waitWork() { g.work.Wait() }

func (g *mountGen) retiredNow() bool { return g != nil && g.retired.Load() }

// retire ends the generation.
func (g *mountGen) retire() {
	g.once.Do(func() {
		g.retired.Store(true)
		close(g.done)
		g.cancel()
		for _, release := range g.releases {
			release()
		}
	})
}

// OnRetire runs release as the module's generation retires, before retire
// returns and so before the next generation mounts: for what only one
// holder can have, which the next generation's module would be refused.
// Call it on the loop, as mounting does. Outside a generation nothing retires.
func (c ModuleContext) OnRetire(release func()) {
	if c.gen != nil {
		c.gen.releases = append(c.gen.releases, release)
	}
}

// ended is closed once the generation retires; nil (never) for a
// context outside any generation.
func (g *mountGen) ended() <-chan struct{} {
	if g == nil {
		return nil
	}
	return g.done
}

func drain[T any](ctx ModuleContext, ch <-chan T, stop func(), fn func(T)) {
	ctx.Go(func() {
		if stop != nil {
			defer stop()
		}
		for {
			select {
			case <-ctx.gen.ended():
				return
			case v, ok := <-ch:
				if !ok {
					return
				}
				fn(v)
			}
		}
	})
}

// follow runs fn through Invoke with the values from ch until the module's
// generation retires or ch closes, then stops the subscription. One call waits
// on the loop at a time, with the latest value: values a stalled loop did not
// get to coalesce, so no feed follows it may need every value.
func follow[T any](ctx ModuleContext, ch <-chan T, stop func(), fn func(T)) {
	latest := latestOnLoop[T]{post: ctx.Invoke, show: fn}
	drain(ctx, ch, stop, latest.give)
}

// latestOnLoop hands values to the loop latest-only: one call waits on post at
// a time, and it takes whatever is newest when it runs.
type latestOnLoop[T any] struct {
	post func(func())
	show func(T)

	mu      sync.Mutex
	latest  T
	waiting bool
}

func (l *latestOnLoop[T]) give(v T) {
	l.mu.Lock()
	l.latest = v
	queue := !l.waiting
	l.waiting = true
	l.mu.Unlock()
	if !queue {
		return
	}
	l.post(func() {
		l.mu.Lock()
		next := l.latest
		l.waiting = false
		l.mu.Unlock()
		l.show(next)
	})
}

// Module is one bar module: a live widget tree plus whatever timers
// keep it current. The interface grows update/message plumbing as
// services land.
type Module interface {
	Root() widget.Widget
}

// Factory builds one module instance.
type Factory func(ctx ModuleContext) (Module, error)

var factories = map[string]Factory{
	"audio":             newAudio,
	"battery":           newBattery,
	"brightness":        newBrightness,
	"keyboard-input":    newKeyboardLayout,
	"media":             newMedia,
	"minimized-windows": newMinimizedWindows,
	"bluetooth":         newBluetooth,
	"network":           newNetwork,
	"window-title":      newWindowTitle,
	"cpu":               newCpu,
	"ram":               newRam,
	"storage":           newStorage,
	"weather":           newWeather,
	"world-clock":       newWorldClock,
	"netstat":           newNetstat,
	"mail":              newMail,
	"bitwarden":         newBitwarden,
	"power":             newPower,
	"dashboard":         newDashboard,
	"keybind-mode":      newKeybindMode,
	"power-profiles":    newPowerProfiles,
	"nightlight":        newNightlight,
	"idle-inhibit":      newIdleInhibit,
	"treeman":           newTreeman,
	"notifications":     newNotification,
	"recorder":          newRecorder,
	"systray":           newSystray,
	"clock":             newClock,
	"cava":              newCava,
	"workspaces":        newWorkspaces,
	"background-apps":   newBackgroundApps,
	"screenshot":        newScreenshot,
	"separator":         newSeparator,
}

// Create builds the module named by a layout entry. Custom modules
// (custom-*) resolve through the [modules.custom] definitions; a name
// outside both is an error, logged by the caller while the rest of the
// bar builds; a schema module without one (updates) is "not
// implemented".
func Create(name string, ctx ModuleContext) (Module, error) {
	if def, ok := barview.CustomDefinition(name, ctx.Config); ok {
		return newCustomByID(ctx, def.Id)
	}
	factory, ok := factories[name]
	if !ok {
		return nil, fmt.Errorf("bar: module %q not implemented", name)
	}
	return factory(ctx)
}

// appendItems builds every item of one layout section into a bar zone
// (barview.FillZone). A module that cannot be built (its service is
// unavailable on this system, its compositor is not running) is logged
// and left out: one missing daemon never takes the bar down.
func appendItems(zone *widget.Box, items []config.BarItem, ctx ModuleContext) {
	barview.FillZone(zone, ctx.Config, items, moduleAppender(ctx))
}

func moduleAppender(ctx ModuleContext) barview.ModuleAppender {
	return func(row *widget.Box, item config.BarItem) {
		if err := appendModule(row, item, ctx); err != nil {
			log.Printf("bar: module %q will not appear: %v", item.Module, err)
		}
	}
}

func appendModule(row *widget.Box, item config.BarItem, ctx ModuleContext) error {
	module, err := Create(string(item.Module), ctx)
	if err != nil {
		return err
	}
	if a, ok := module.(interface{ Attach(app.Host) }); ok && ctx.Attachers != nil {
		*ctx.Attachers = append(*ctx.Attachers, a)
	}
	root := module.Root()
	// Components that are not bar buttons (the workspace rows) carry
	// their own chrome and input handling.
	if _, ok := module.(interface{ ownsChrome() }); ok {
		barview.OwnChrome(root, item, ctx.Config)
		row.Append(root, false)
		return nil
	}
	var own []string
	if t, ok := module.(interface{ typeClass() string }); ok {
		own = append(own, t.typeClass())
	}
	wrap := barview.ModuleBox(item, own...)
	if c, ok := module.(interface{ setChrome(moduleChrome) }); ok {
		c.setChrome(wrap)
	}
	var handler interface {
		RunAction(config.ClickAction)
	}
	if h, ok := module.(interface {
		RunAction(config.ClickAction)
	}); ok {
		handler = h
	}
	btn := asBarButton(ctx, root)
	if item.Module == "dashboard" && ctx.Dropdowns != nil {
		ctx.Dropdowns.setDashboardAnchor(ctx.Connector, btn)
	}
	follower, _ := module.(interface {
		followAction(action config.ClickAction, scroll bool)
	})
	btn.configure(barview.ModuleTitle(string(item.Module), ctx.Config), barview.ModuleButton(string(item.Module), ctx.Config), moduleBinding(string(item.Module), ctx.Config), func(action config.ClickAction, scroll bool) {
		switch {
		case action.Kind == config.ClickDropdown && ctx.Dropdowns != nil:
			// Dropdown bindings anchor to this module's own button; the
			// registry toggles the popover on the connector's host.
			_ = ctx.Dropdowns.open(ctx.Connector, action.Dropdown, btn, openedByAnchor)
		case handler != nil:
			handler.RunAction(action)
		default:
			runClickAction(ctx, action)
		}
		// A module that reacts after the binding (the custom module's
		// on-action) hears every one.
		if follower != nil {
			follower.followAction(action, scroll)
		}
	})
	wrap.Append(btn, false)
	row.Append(wrap, false)
	return nil
}

// asBarButton returns the module root as a bar button: the one the
// module assembled, a bare icon as the button's icon (the label-less
// icon buttons: power, dashboard), or any other content wrapped in the
// button chrome.
func asBarButton(ctx ModuleContext, root widget.Widget) *barButton {
	switch r := root.(type) {
	case *barButton:
		return r
	case *widget.Icon:
		return newBarButton(ctx, r, "")
	}
	return newBarButtonAround(ctx, root)
}

// moduleBinding is the module's click and scroll bindings.
func moduleBinding(name string, cfg *config.Config) config.ClickConfig {
	if clicks, ok := cfg.ModuleClicks(name); ok {
		return clicks
	}
	if def, ok := barview.CustomDefinition(name, cfg); ok {
		return def.Clicks()
	}
	return config.ClickConfig{}
}
