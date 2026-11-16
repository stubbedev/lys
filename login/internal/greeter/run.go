package greeter

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/desktop/logging"
	"github.com/stubbedev/lys/desktop/outputsize"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/desktop/tokens"
	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/wallclock"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/login/internal/auth"
	"github.com/stubbedev/lys/login/internal/layouts"
	loginscreen "github.com/stubbedev/lys/login/internal/screen"
)

// Run shows the greeter until greetd starts a session (or the window
// closes): one fullscreen window on the kiosk compositor.
func Run(init Init) error {
	// lys-ui draws the cursor client-side from the XCURSOR_* theme: set
	// the resolved one before connecting.
	ApplyCursor(ResolveCursor(init.Config.Login.Greeter, detectLastCursor(init)))

	// The stylesheet's font families and weights resolve through the
	// system store, as in the shell (the clock's 700 weight).
	widget.SetFaceResolver(apptheme.FontResolver)
	if err := installIcons(); err != nil {
		log.Printf("greeter: %v", err)
	}
	app.SetLogger(logging.Library())
	sess, err := app.Connect()
	if err != nil {
		return fmt.Errorf("greeter: connect: %w", err)
	}
	defer sess.Close()
	icons := iconPreload{scales: outputScales(sess.Outputs())}
	defer icons.wait()
	icons.start(preloadButtonIcons)
	a := app.NewApplication(sess)
	scaleText := followTextScale(a)
	a.SetAccessibility(app.A11yOff)
	readerPath, readerFound := screenReaderPath(init.Config.Login.Greeter.ScreenReader)
	var screenReaders readers
	defer screenReaders.wait()

	family := init.Config.General.FontSans
	base, err := app.Font(family, 16)
	if err != nil {
		return fmt.Errorf("greeter: font %q: %w", family, err)
	}
	font := app.FontFallback(base)

	wall, err := wallclock.NewSystemClock(a.Invoke, wallclock.LocaltimePath)
	if err != nil {
		return fmt.Errorf("greeter: %w", err)
	}
	defer func() {
		if err := wall.Close(); err != nil {
			log.Printf("greeter: close the clock: %v", err)
		}
	}()
	var power systemPower
	defer power.close()
	var win *app.Window
	var held widget.Widget
	home := homeOutput(sess.Outputs())
	sheet := Stylesheet(init.Config)
	s := newScreen(init, font, sheet, deps{
		connect: func(start auth.Start) (auth.Conversation, error) {
			return auth.NewGreetdFromEnv(start)
		},
		spawn: func(conv auth.Conversation, username string, onEvent func(auth.Event)) authHandle {
			return auth.Spawn(conv, username, onEvent)
		},
		invoke: a.Invoke,
		focus: func(w widget.Widget) {
			if win == nil {
				return
			}
			if err := win.SetFocus(w); err != nil {
				log.Printf("greeter: cannot focus %T: %v", w, err)
				return
			}
			held = w
		},
		focused:      func() widget.Widget { return held },
		keyboardHeld: func() bool { return sess.KeyboardFocus() != nil },
		quit:         a.Quit,
		whenPresented: func(fn func()) error {
			if win == nil {
				return errors.New("greeter: no window to present")
			}
			return win.WhenPresented(fn)
		},
		after:        time.After,
		wall:         wall,
		now:          wall.Now,
		saveLast:     SaveLast,
		power:        power.press,
		loadImage:    imagedecode.OpenOffHeap,
		outputs:      func() []image.Point { return outputsize.Devices(sess.Outputs()) },
		forget:       a.ForgetImage,
		loadAvatar:   imagedecode.Open,
		outputScale:  func() int { return loginscreen.MaxScale(sess.Outputs()) },
		nextLayout:   layouts.Switcher("greeter"),
		labelLayouts: layouts.LabeledOnSystem,
		splash:       loadSplashFrame,
		windowOutput: func() string { return windowOutput(win, home) },
		a11y: a11yDeps{
			readerFound:  readerFound,
			startReader:  screenReaders.start(a, readerPath),
			textScale:    scaleText,
			highContrast: widget.SetMediaContrast,
			dispatch:     wmDispatch(),
			popover:      openPopover,
		},
	})
	defer s.end()
	icons.start(s.prompt.PreloadIcons)
	win, err = a.NewWindow(windowConfig(s.root, init.OnPresented, s.captureKey, s.outputChanged))
	if err != nil {
		return err
	}
	win.FullscreenOn(home)
	var covers covering
	defer covers.end()
	s.start()
	err = covers.startAfter(win.WhenPresented, s.markPresented, func() func() {
		cover := func(layer func(*app.Output, render.Font, *widget.Stylesheet) app.LayerConfig) func(*app.Output) {
			return func(out *app.Output) {
				if _, err := a.NewLayer(layer(out, font, sheet)); err != nil {
					log.Printf("greeter: cover output %s: %v", out.Name, err)
				}
			}
		}
		return coverOutputs(sess.LayerShell() != nil, sess.Outputs(), sess.WatchOutputs, cover(s.handOverCoverOn), cover(s.coverOn))
	})
	if err != nil {
		return fmt.Errorf("greeter: wait for the first frame: %w", err)
	}
	s.clock.Arm()
	defer s.clock.Stop()
	var stopAnnouncing func()
	stopAnnouncing = a.Every(time.Second, func() {
		s.announceReady()
		if s.announced {
			stopAnnouncing()
		}
	})
	a.OnLocks(func(l app.Locks) { s.setCapsLock(l.Caps) })
	a.OnKeyboardLayout(s.layouts.Set)
	s.applyDebugOps(os.Getenv("LYS_SHELL_GREETER_DEBUG"), func(d time.Duration, fn func()) {
		time.AfterFunc(d, func() { a.Invoke(fn) })
	})
	log.Printf("lys-greeter starting (%d sessions)", len(init.Sessions))
	icons.wait()
	err = a.Run()
	if err == app.ErrClosed { //nolint:errorlint // the loop's sentinel
		return nil
	}
	return err
}

type covering struct {
	stop func()
}

func (c *covering) startAfter(whenPresented func(func()) error, presented func(), start func() (stop func())) error {
	return whenPresented(func() {
		presented()
		c.stop = start()
	})
}

func (c *covering) end() {
	if c.stop != nil {
		c.stop()
	}
}

func coverOutputs(layerShell bool, outputs []*app.Output, watch func(added, removed func(*app.Output)) func(), first, added func(*app.Output)) (stop func()) {
	if !layerShell {
		log.Printf("greeter: the compositor has no layer shell, so only the greeter's window covers its outputs")
		return func() {}
	}
	for _, out := range outputs {
		first(out)
	}
	return watch(added, nil)
}

func homeOutput(outputs []*app.Output) *app.Output {
	if len(outputs) == 0 {
		return nil
	}
	return outputs[0]
}

func windowOutput(win *app.Window, home *app.Output) string {
	if win != nil {
		if on := win.Outputs(); len(on) > 0 {
			return on[0].Name
		}
	}
	if home == nil {
		return ""
	}
	return home.Name
}

func windowConfig(root widget.Widget, onPresented func(), capture func(app.Accel) bool, onOutputs func()) app.WindowConfig {
	cfg := app.WindowConfig{
		KeyCapture: capture,
		OnOutputs:  onOutputs,
		Title:      "lys-greeter",
		AppID:      "dev.stubbe.lys.greeter",
		Root:       root,
		Background: render.RGB(0, 0, 0),
	}
	if onPresented != nil {
		cfg.OnPresented = func() { go onPresented() }
	}
	return cfg
}

// applyDebugOps is the dev-harness hook (LYS_SHELL_GREETER_DEBUG):
// comma-separated "popup" opens the session picker and
// "login=<user>:<pass>" fills the form and submits, each a second in.
// greetd never forwards the variable, so production never sees it.
func (s *screen) applyDebugOps(spec string, after func(time.Duration, func())) {
	if spec == "" {
		return
	}
	for op := range strings.SplitSeq(spec, ",") {
		if op == "popup" {
			after(time.Second, s.drop.Open)
			continue
		}
		login, ok := strings.CutPrefix(op, "login=")
		if !ok {
			continue
		}
		user, pass, ok := strings.Cut(login, ":")
		if !ok {
			continue
		}
		after(time.Second, func() {
			s.prompt.Username.SetText(user)
			s.submit(pass)
		})
	}
}

// Stylesheet is install_css: the compiled stylesheet and the configured
// palette's theme, at the bundle's priority, with lys-ui's theme
// installed from the same palette. The greeter carries no user styles:
// pre-login there is no user.
func Stylesheet(cfg *config.Config) *widget.Stylesheet {
	palette, p, err := resolvePalette(cfg.Styling.ActivePalette())
	if err != nil {
		log.Printf("%v; showing the default lys-shell palette", err)
	}
	apptheme.InstallWidgetTheme(p, cfg)
	return widget.NewStylesheet(styling.BundleCSS(cfg, palette, p), apptheme.Priority)
}

func resolvePalette(configured config.Palette) (config.Palette, *styling.Palette, error) {
	p, err := styling.PaletteFromHex(configured)
	if err == nil {
		return configured, p, nil
	}
	fallback := tokens.LysShell()
	p, fallbackErr := styling.PaletteFromHex(fallback)
	if fallbackErr != nil {
		panic(fmt.Sprintf("greeter: the default lys-shell palette: %v", fallbackErr))
	}
	return fallback, p, fmt.Errorf("greeter: resolve the configured palette: %w", err)
}

// systemPower runs the power buttons' actions through logind, which
// allows each for the active local session, on one system bus
// connection: dialled on the first press, again after a failed dial, and
// dropped by close.
type systemPower struct {
	mu   sync.Mutex
	conn *dbus.Conn
}

func (p *systemPower) press(action logind.PowerAction) {
	if err := p.run(action); err != nil {
		log.Printf("greeter: %v", err)
	}
}

func (p *systemPower) run(action logind.PowerAction) error {
	m, err := p.manager()
	if err != nil {
		return fmt.Errorf("power action %s: %w", action, err)
	}
	if err := m.Power(context.Background(), action); err != nil {
		return fmt.Errorf("power action %s: %w", action, err)
	}
	return nil
}

func (p *systemPower) manager() (logind.Manager, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn == nil {
		conn, err := logind.Dial(context.Background())
		if err != nil {
			return logind.Manager{}, err
		}
		p.conn = conn
	}
	return logind.NewManager(p.conn), nil
}

func (p *systemPower) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}
}
