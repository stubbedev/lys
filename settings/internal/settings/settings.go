package settings

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/logging"
	"github.com/stubbedev/lys/desktop/service/clipboard"
	"github.com/stubbedev/lys/desktop/service/powersupply"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/lib/hibernate"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/accounts"
	"github.com/stubbedev/lys/settings/internal/locale"
)

// appID is the window's application id (dev.stubbe.lys.settings, the
// desktop entry's name).
const appID = "dev.stubbe.lys.settings"

// Run loads the config and shows the settings window until it closes.
// A config that cannot load is an error: the window edits
// it, so there is nothing to show without it.
func Run(page, setting string) error {
	// raise is set once the window exists; forwarded launches buffer
	// until Bind, after it.
	var raise func(args []string)
	ins, primary, err := claimInstance(app.OSInvocation(os.Args[1:]), func(args []string) { raise(args) })
	switch {
	case err != nil:
		// No socket for the guard: run anyway, as GApplication does
		// without a bus.
		log.Printf("settings: single instance: %v", err)
	case !primary:
		// The running window was raised in this launch's stead.
		return nil
	}
	installed := installedParts()
	svc, err := config.Open()
	if err != nil {
		return fmt.Errorf("cannot load config: %w", err)
	}
	defer svc.Close()
	if page != "" {
		if _, ok := pageByID(layout(svc.Config(), installed), page); !ok {
			return fmt.Errorf("settings: no page %q", page)
		}
	}
	app.SetLogger(logging.Library())
	sess, err := app.Connect()
	if err != nil {
		return fmt.Errorf("settings: connect: %w", err)
	}
	defer sess.Close()

	application := app.NewApplication(sess)
	shipIcons()
	defer apptheme.Setup(application)()
	cfg := svc.Config()
	theme := apptheme.New(cfg)
	theme.WatchUserStyles(application)
	face, err := app.Font(cfg.General.FontSans, 14)
	if err != nil {
		return fmt.Errorf("settings: font %q: %w", cfg.General.FontSans, err)
	}
	// The code views are GtkTextView's monospace (the family alias).
	mono, err := app.Font("monospace", 14)
	if err != nil {
		log.Printf("settings: monospace font: %v", err)
		mono = face
	}
	k := &kit{
		face: app.FontFallback(face), mono: app.FontFallback(mono), monoVariants: app.FontVariants(mono),
		store: store{svc}, invoke: application.Invoke, parts: installed,
		background:    func(fn func()) { go fn() },
		systemBus:     dialSystemBus,
		network:       func() (netControl, error) { return openNetwork(dialSystemBus) },
		statuses:      tileStatuses,
		power:         func() (powerControl, error) { return openPower(dialSystemBus, powersupply.New(powersupply.Dir)) },
		pausing:       func() (pauseControl, error) { return openPausing(dialSystemBus, dialSessionBus) },
		bluetooth:     func() (btControl, error) { return openBluetooth(dialSystemBus) },
		dnd:           func() (dndControl, error) { return openDND(dialSessionBus) },
		nightlight:    func() (nightlightControl, error) { return openNightlight(dialSessionBus) },
		nightLocation: timeZonePlace,
		region:        sessionRegion,
		brightness:    func() (brightnessControl, error) { return openBrightness(dialSessionBus) },
		vault:         func() (vaultControl, error) { return openVault(dialSessionBus) },
		cameras:       watchCameras,
		screenShares:  func() (shareSource, func(), error) { return followShares(dialSessionBus) },
		notify:        sendNotification,
		shellCommand:  runShellCommand,
		checkPAM:      checkPAMService,
		readAccounts:  accounts.ReadDatabase,
		changeAccount: changeAccount,
		openURI:       openWithDefaultApp,
		i2c:           systemHost.i2cAccess,
		hibernation:   func() ([]hibernate.Blocker, error) { return hibernate.Blockers(os.DirFS("/")) },
		mailCount:     countMail,
		barFont:       barview.Font,
		familyFont:    loadFamily,
		fontFile:      app.FontFile,
		fileFamily:    readFontFamily,
		forgetImage:   application.ForgetImage,
		clock:         wallClock{},
	}
	w, win, stop, err := openSettingsWindow(application, sess.Outputs, k, theme)
	if err != nil {
		return err
	}
	defer stop()
	w.request(launch{page: page, setting: setting})
	w.focusPage()
	if err := application.AddAccel("ctrl+f", widget.NewAction("settings.search", w.focusSearch)); err != nil {
		return fmt.Errorf("settings: search shortcut: %w", err)
	}
	if ins != nil {
		// A second launch raises this window (connect_activate's
		// present): its activation token focuses it when the launcher
		// gave one, else the window asks for attention.
		raise = func(args []string) {
			w.request(parseLaunch(args))
			win.RequestAttention()
		}
		ins.Bind(application)
	}
	return application.Run()
}

// openSettingsWindow builds the settings window over application for k,
// styled by theme: the window, its pickers, toasts, output overlays,
// clipboard and reset-all wired to it, and a change to the config from
// any source - a row, the reset buttons, the config file, another
// lys-shell - restyling it and refreshing its page. stop ends that
// following.
func openSettingsWindow(application *app.Application, outputs func() []*app.Output, k *kit, theme *apptheme.Theme) (w *window, win *app.Window, stop func(), err error) {
	cfg := k.store.svc.Config()
	after := func(d time.Duration, fn func()) {
		time.AfterFunc(d, func() { application.Invoke(fn) })
	}
	w = newWindow(k, cfg, after)
	k.onWMChange = w.refresh
	theme.Attach(w.root)
	win, err = application.NewWindow(app.WindowConfig{
		Title: locale.T().Get("settings-title"), AppID: appID,
		Width: windowWidth, Height: windowHeight,
		Root: w.root, Background: theme.RenderPalette().Surface,
		OnPresented: w.presentedFirst,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("settings: window: %w", err)
	}
	w.onClose = win.Close
	w.focus = func(target widget.Widget) {
		app.Trace("settings", "focus %T", target)
		if err := win.SetFocus(target); err != nil {
			log.Printf("settings: focus %T: %v", target, err)
		}
	}
	k.pickers = windowPickers{app: application, win: win, theme: theme}
	k.toast = func(text string, life time.Duration) (*widget.Toast, error) {
		return application.ShowToast(text, life, &app.ToastConfig{Host: win, Over: w.stack, CloseOnPress: true})
	}
	k.overlays = func(output string, content widget.Widget, at overlayPlacement) (func(), error) {
		return outputOverlay(application, outputs(), theme, output, content, at)
	}
	clip := func() (*app.Clipboard, error) {
		if c := application.Clipboard(); c != nil {
			return c, nil
		}
		return nil, app.ErrClipboardUnavailable
	}
	k.copySecret = func(text string) error {
		c, err := clip()
		if err != nil {
			return err
		}
		return c.WriteExpiring(clipboard.SecretContent(text), copiedSecretLife)
	}
	k.copyText = func(text string) error {
		c, err := clip()
		if err != nil {
			return err
		}
		return c.WriteText(text)
	}
	w.onResetAll = func() { confirmResetAll(application, win, k) }
	stop = k.store.svc.Subscribe(func(_, next *config.Config) {
		application.Invoke(func() {
			theme.SetConfig(next)
			w.refresh()
		})
	})
	return w, win, stop, nil
}

// shipIcons puts the icons this app installs on the search path: they live in
// its own share directory, so it needs no lys-shell and owns no file of its.
func shipIcons() {
	exe, err := os.Executable()
	if err != nil {
		log.Printf("settings: icons: find this program: %v", err)
		return
	}
	dir := filepath.Join(filepath.Dir(exe), "..", "share", "lys-settings", "icons")
	widget.SetIconSearchPaths(append(widget.IconSearchPaths(), dir))
}

// confirmResetAll asks, then drops every runtime override
// (ConfirmResetAll, ExecuteResetAll).
func confirmResetAll(application *app.Application, parent *app.Window, k *kit) {
	if _, err := application.NewDialog(parent, resetAllDialog(k)); err != nil {
		log.Printf("settings: reset-all dialog: %v", err)
	}
}

// claimInstance is the dev.stubbe.lys.settings single-instance guard
// (set_application_id): the first launch is the primary, a later one
// forwards its invocation and exits.
// onRelaunch runs on the loop for each later launch.
func claimInstance(inv app.Invocation, onRelaunch func(args []string)) (*app.Instance, bool, error) {
	return app.ClaimInstance(app.InstanceConfig{
		AppID:         appID,
		OnCommandLine: func(args []string, _ string) { onRelaunch(args) },
		OnOpen:        func([]string, string) { onRelaunch(nil) },
	}, inv)
}

// windowPickers opens the editors' popovers and dialogs over the
// settings window, styled by its stylesheet.
type windowPickers struct {
	app   *app.Application
	win   *app.Window
	theme *apptheme.Theme
}

// popover opens content beside anchor, focus (its search entry)
// taking the keys as it opens, so typing filters at once.
func (p windowPickers) popover(anchor, content, focus widget.Widget) (closePopover func()) {
	bound, ok := anchor.(widget.Boundser)
	if !ok {
		return func() {}
	}
	p.style(content)
	pop, err := p.app.OpenPopover(p.win, pickerPopoverConfig(bound, content, focus))
	if err != nil {
		log.Printf("settings: popover: %v", err)
		return func() {}
	}
	return pop.Dismiss
}

// style puts the app's stylesheet on content, which a popover hosts
// outside the window's tree; a dialog takes it from the window itself.
func (p windowPickers) style(content widget.Widget) {
	if root, ok := content.(interface{ AttachStylesheet(*widget.Stylesheet) }); ok {
		p.theme.Attach(root)
	}
}

func (p windowPickers) dialog(cfg app.DialogConfig) (closeDialog func()) {
	d, err := p.app.NewDialog(p.win, cfg)
	if err != nil {
		log.Printf("settings: dialog %q: %v", cfg.Title, err)
		return func() {}
	}
	return d.Close
}

func loadFamily(family string) (render.Font, error) {
	face, err := app.Font(family, fontSamplePx)
	if err != nil {
		return nil, err
	}
	return app.FontFallback(face), nil
}

// font opens lys-ui's font dialog for families alone (FontButton's
// family-only level), monospace ones only when monospace.
func (p windowPickers) font(family string, monospace bool, fn func(string)) {
	if _, err := p.app.FontChooserDialog(p.win, app.FontChooserConfig{
		Family: family, FamilyOnly: true, MonospaceOnly: monospace,
		OnFont: func(picked string, _ float64) { fn(picked) },
	}); err != nil {
		log.Printf("settings: font dialog: %v", err)
	}
}

// openFile asks lys-ui's file picker for one file: a settings app runs where no
// portal does, on lys-wm without the shell.
func (p windowPickers) openFile(fn func(path string)) {
	if _, err := p.app.OpenFileDialog(p.win, app.FileDialogConfig{
		OnOpen: func(paths []string) {
			if len(paths) > 0 {
				fn(paths[0])
			}
		},
	}); err != nil {
		log.Printf("settings: file chooser: %v", err)
	}
}

func (p windowPickers) openFolder(fn func(path string)) {
	if _, err := p.app.OpenFolderDialog(p.win, app.FileDialogConfig{
		OnOpen: func(paths []string) {
			if len(paths) > 0 {
				fn(paths[0])
			}
		},
	}); err != nil {
		log.Printf("settings: folder chooser: %v", err)
	}
}

// pickerPopoverConfig is an editor's popover: it opens from the editor
// at lys-ui's popover gap, focus inside it taking the keys.
func pickerPopoverConfig(anchor widget.Boundser, content, focus widget.Widget) app.PopoverConfig {
	return app.PopoverConfig{Anchor: anchor, Content: content, Focus: focus, Gap: widget.PopoverGap}
}

// overlayCornerMargin is how far a corner overlay sits in from its
// output's edges, in logical pixels.
const overlayCornerMargin int32 = 32

// overlayPlacement is where an overlay sits on its output and whether it
// takes the keys.
type overlayPlacement uint8

const (
	// overlayCorner is a label naming the output, in its top-left corner
	// clear of the windows centred on it, taking no keys.
	overlayCorner overlayPlacement = iota
	// overlayCentred is a question asked on the output, centred, taking
	// the keys when clicked.
	overlayCentred
)

// layer is the anchor, the margins and the keyboard mode of an overlay
// layer placed at p.
func (p overlayPlacement) layer() (app.Anchor, app.Margins, app.KeyboardMode) {
	switch p {
	case overlayCorner:
		return app.AnchorTop | app.AnchorLeft, app.Margins{Top: overlayCornerMargin, Left: overlayCornerMargin}, app.KeyboardNone
	case overlayCentred:
		return 0, app.Margins{}, app.KeyboardOnDemand
	}
	log.Panicf("settings: no overlay placement %d", p)
	return 0, app.Margins{}, app.KeyboardNone
}

// outputOverlay opens content as an overlay layer on the output named,
// placed at, styled by the window's stylesheet; the returned close may
// run more than once.
func outputOverlay(application *app.Application, outputs []*app.Output, theme *apptheme.Theme, name string, content widget.Widget, at overlayPlacement) (func(), error) {
	i := slices.IndexFunc(outputs, func(o *app.Output) bool { return o.Name == name })
	if i < 0 {
		return nil, fmt.Errorf("no output %s", name)
	}
	anchor, margins, keys := at.layer()
	root := widget.NewBox(widget.Column, 0, widget.Current().Padding*2)
	root.AddClass("display-overlay")
	root.Append(content, false)
	theme.Attach(root)
	layer, err := application.NewLayer(app.LayerConfig{
		Output: outputs[i], Layer: app.LayerOverlay, Namespace: "lys-settings-displays",
		Anchor: anchor, Margin: margins, Keyboard: keys, Root: root, Background: theme.RenderPalette().Surface,
	})
	if err != nil {
		return nil, fmt.Errorf("open an overlay on %s: %w", name, err)
	}
	return func() {
		if !layer.Closed() {
			layer.Close()
		}
	}, nil
}
