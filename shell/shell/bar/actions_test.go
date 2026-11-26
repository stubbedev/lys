package bar

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/idleinhibit/inhibitduration"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/logind/logindtest"
	"github.com/stubbedev/lys/lib/logtest"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/wmipc"
	"github.com/stubbedev/lys/ui/app"

	"github.com/stubbedev/lys/shell/internal/notifytest"
	"github.com/stubbedev/lys/shell/service/idleinhibit"
	"github.com/stubbedev/lys/shell/service/recorder"
	"github.com/stubbedev/lys/shell/service/recorder/recordertest"
	"github.com/stubbedev/lys/shell/service/sessionctl"
	"github.com/stubbedev/lys/shell/shell/powermenu"
)

// spawns records what reaches the shell instead of starting it.
func spawns(t *testing.T) func() []string {
	t.Helper()
	var mu sync.Mutex
	var got []string
	old := spawnQuiet
	spawnQuiet = func(line string) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, line)
	}
	t.Cleanup(func() { spawnQuiet = old })
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}
}

// servicedContext is a module context with every service a default
// binding acts on.
func servicedContext(t *testing.T, cfg *config.Config) ModuleContext {
	t.Helper()
	ctx := newTestContext(t, cfg)
	ctx.Screenshot = func(string, string) {}
	ctx.Brightness = &fakeBrightnessSource{ticks: make(chan struct{})}
	ctx.Pulse = &fakePulseSource{err: errors.New("no sink"), ticks: make(chan struct{})}
	ctx.IdleInhibit = idleinhibit.NewState(inhibitduration.Duration{})
	ctx.Notifications = notifytest.NewService(t)
	ctx.Recorder = recorder.NewState(&recordertest.Engine{}, func() config.RecorderConfig { return cfg.Recorder }, recorder.Hooks{}, time.Now)
	t.Cleanup(ctx.Recorder.Stop)
	ctx.PowerProfiles = &fakePPSource{ticks: make(chan struct{})}
	ctx.Nightlight = onHeadlessLoop(func() *nightlightService { return newNightlightService(ctx, (&fakeSunset{}).env(t)) })
	ctx.PowerMenu = powermenu.New(powermenu.Deps{
		Config: func() *config.Config { return cfg },
		Open: func(lc app.LayerConfig) (powermenu.Window, error) {
			return treetest.NewLayer(lc, render.Rect{W: 1600, H: 1000}), nil
		},
		Run:  func(powermenu.Action, config.PowerCommand) {},
		Font: ctx.Font,
	})
	return ctx
}

// dispatch runs a binding of module the way the bar's button does: a
// dropdown opens, the module's own verbs go to it, the rest run.
func dispatch(ctx ModuleContext, module Module, action config.ClickAction) {
	if action.Kind == config.ClickDropdown {
		return
	}
	onHeadlessLoop(func() bool {
		if h, ok := module.(interface{ RunAction(config.ClickAction) }); ok {
			h.RunAction(action)
		} else {
			runClickAction(ctx, action)
		}
		return true
	})
}

// defaultBindingModule builds module name over ctx; keyboard-input, which
// needs a compositor to build, is its bare module, whose own verb waits
// for one.
func defaultBindingModule(ctx ModuleContext, name string) (Module, error) {
	if name == "keyboard-input" {
		return &keyboardLayout{ctx: ctx}, nil
	}
	return createOnHeadlessLoop(name, ctx)
}

func TestNoDefaultBindingStartsAProcess(t *testing.T) {
	logtest.Capture(t)
	spawned := spawns(t)
	cfg := config.Defaults()
	cfg.Recorder.OutputDirectory = t.TempDir()
	ctx := servicedContext(t, cfg)
	for _, name := range config.BuiltinModules {
		clicks := moduleBinding(string(name), cfg)
		actions := []config.ClickAction{clicks.LeftClick, clicks.RightClick, clicks.MiddleClick, clicks.ScrollUp, clicks.ScrollDown}
		if !slices.ContainsFunc(actions, func(a config.ClickAction) bool { return a.Kind != config.ClickNone && a.Kind != config.ClickDropdown }) {
			continue
		}
		module, err := defaultBindingModule(ctx, string(name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, a := range actions {
			dispatch(ctx, module, a)
			if got := spawned(); len(got) != 0 {
				t.Fatalf("%s's default %q started %v", name, a, got)
			}
		}
	}
}

func TestNoDefaultPowerCommandStartsAProcess(t *testing.T) {
	spawned := spawns(t)
	t.Setenv(wmipc.SocketEnv, "")
	fake := logindtest.Start(t, logindtest.Config{Display: "2"})
	r := powerRunner{system: sessionctl.On(fake.Bus.Conn(t)), invoke: func(fn func()) { fn() }, toast: func(label, _ string) {
		t.Errorf("a default power command failed: %s", label)
	}}
	p := config.DefaultsPower()
	for _, a := range []powermenu.Action{powermenu.Lock, powermenu.LogOut, powermenu.Suspend, powermenu.Reboot, powermenu.ShutDown} {
		r.run(a, a.Command(p))
	}
	want := []string{"Manager.LockSession", "Manager.GetSession", "Session.Terminate", "Manager.Suspend", "Manager.Reboot", "Manager.PowerOff"}
	waittest.Until(t, func() bool {
		methods := fake.Methods()
		return !slices.ContainsFunc(want, func(m string) bool { return !slices.Contains(methods, m) })
	}, "logind to hear every default")
	if got := spawned(); len(got) != 0 {
		t.Errorf("the default power commands started %v", got)
	}
}

func TestAUserPowerCommandRunsThroughTheShell(t *testing.T) {
	spawned := spawns(t)
	r := powerRunner{system: sessionctl.On(nil), invoke: func(fn func()) { fn() }}
	r.run(powermenu.Lock, config.ShellPowerCommand("swaylock -f"))
	r.run(powermenu.Lock, config.PowerCommand{})
	if got := spawned(); !slices.Equal(got, []string{"swaylock -f"}) {
		t.Errorf("ran %v, want the user's command alone", got)
	}
}

func TestARefusedBuiltinIsToasted(t *testing.T) {
	logtest.Capture(t)
	t.Setenv(wmipc.SocketEnv, "")
	fake := logindtest.Start(t, logindtest.Config{Display: "2"})
	fake.Refuse("Manager.Reboot", dbus.NewError("org.freedesktop.DBus.Error.AccessDenied", []any{"access denied"}))
	toasts := make(chan [2]string, 1)
	r := powerRunner{system: sessionctl.On(fake.Bus.Conn(t)), invoke: func(fn func()) { fn() }, toast: func(label, icon string) { toasts <- [2]string{label, icon} }}
	r.run(powermenu.Reboot, config.BuiltinPowerCommand(config.PowerBuiltinReboot))
	got := waittest.For(t, toasts, "the refused reboot's toast")
	if want := [2]string{i18n.T("launcher-failed", i18n.Str("what", powermenu.Reboot.Label())), powermenu.Reboot.Icon()}; got != want {
		t.Errorf("toasted %q, want %q", got, want)
	}
	if got := fake.Events.Load(); len(got) != 0 {
		t.Errorf("logind events %v after the refusal", got)
	}
}

func TestTheIdleBuiltinsToggleTheInhibitor(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	ctx.IdleInhibit = idleinhibit.NewState(inhibitduration.Duration{})
	runBuiltin(ctx, config.ShellBuiltinIdleToggleIndefinite)
	if !ctx.IdleInhibit.Active() || !ctx.IdleInhibit.Indefinite() {
		t.Fatalf("indefinite toggle: active %v indefinite %v", ctx.IdleInhibit.Active(), ctx.IdleInhibit.Indefinite())
	}
	runBuiltin(ctx, config.ShellBuiltinIdleToggle)
	if ctx.IdleInhibit.Active() {
		t.Fatal("a second toggle left the inhibitor on")
	}
}

func TestTheDndBuiltinTogglesDoNotDisturb(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	ctx.Notifications = notifytest.NewService(t)
	runBuiltin(ctx, config.ShellBuiltinDndToggle)
	if !ctx.Notifications.DND() {
		t.Fatal("the toggle left do-not-disturb off")
	}
	runBuiltin(ctx, config.ShellBuiltinDndToggle)
	if ctx.Notifications.DND() {
		t.Fatal("a second toggle left do-not-disturb on")
	}
}

func TestTheRecorderBuiltinsDriveTheRecorder(t *testing.T) {
	cfg := config.Defaults()
	cfg.Recorder.OutputDirectory = t.TempDir()
	cfg.Recorder.StartDelayMs = 0
	ctx := newTestContext(t, cfg)
	ctx.Recorder = recorder.NewState(&recordertest.Engine{}, func() config.RecorderConfig { return cfg.Recorder }, recorder.Hooks{}, time.Now)
	t.Cleanup(ctx.Recorder.Stop)
	runBuiltin(ctx, config.ShellBuiltinRecorderToggle)
	waittest.Until(t, func() bool { return ctx.Recorder.Snapshot().Status == recorder.StatusRecording }, "the toggle to start recording")
	runBuiltin(ctx, config.ShellBuiltinRecorderToggle)
	waittest.Until(t, func() bool { return ctx.Recorder.Snapshot().Status == recorder.StatusIdle }, "the toggle to stop recording")
}

func TestABuiltinWithoutItsServiceDoesNothing(t *testing.T) {
	spawned := spawns(t)
	log := logtest.Capture(t)
	ctx := newTestContext(t, config.Defaults())
	for _, b := range config.ShellBuiltins() {
		runBuiltin(ctx, b)
	}
	if got := spawned(); len(got) != 0 {
		t.Errorf("builtins without their service started %v", got)
	}
	for _, b := range config.ShellBuiltins() {
		if !strings.Contains(log.String(), "click: "+b.String()+": no ") {
			t.Errorf("%s left no word of its missing service: %q", b, log.String())
		}
	}
}
