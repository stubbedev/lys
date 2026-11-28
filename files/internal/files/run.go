package files

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/dochelper"
	"github.com/stubbedev/lys/desktop/fileops"
	"github.com/stubbedev/lys/desktop/fileuri"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/logging"
	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/spawn"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/xdg"
	"github.com/stubbedev/lys/shell/service/launcher"
	"github.com/stubbedev/lys/shell/service/udisks"
	"github.com/stubbedev/lys/shell/shell/filebrowser"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/files/internal/filemanager1"
	"github.com/stubbedev/lys/files/internal/locale"
)

const (
	windowWidth, windowHeight = 1100, 720
	fontPx                    = 14
	toastLife                 = 5 * time.Second
	busTimeout                = 5 * time.Second
)

// ServiceArg is the argument the D-Bus activation file starts the file
// manager with: it serves org.freedesktop.FileManager1 and opens a
// window once asked to show something.
const ServiceArg = "--service"

// Run shows paths in tabs, or with service, waits to be asked over
// org.freedesktop.FileManager1, until the window closes. A launch while
// a file manager runs hands its paths to that one.
func Run(paths []string, service bool) error {
	var forwarded func(paths []string, args []string)
	inv := app.Invocation{Open: paths, Token: os.Getenv("XDG_ACTIVATION_TOKEN")}
	if service {
		inv.Args = []string{ServiceArg}
	}
	ins, primary, err := app.ClaimInstance(app.InstanceConfig{
		AppID:         AppID,
		OnOpen:        func(paths []string, _ string) { forwarded(paths, nil) },
		OnCommandLine: func(args []string, _ string) { forwarded(nil, args) },
	}, inv)
	switch {
	case err != nil:
		log.Printf("files: single instance: %v", err)
	case !primary:
		return nil
	}
	bin, err := fileops.SystemBin()
	if err != nil {
		return fmt.Errorf("files: %w", err)
	}
	svc, err := config.Open()
	if err != nil {
		return fmt.Errorf("files: load the desktop config: %w", err)
	}
	defer svc.Close()
	app.SetLogger(logging.Library())
	sess, err := app.Connect()
	if err != nil {
		return fmt.Errorf("files: connect: %w", err)
	}
	defer sess.Close()

	application := app.NewApplication(sess)
	shipIcons()
	defer apptheme.Setup(application)()
	cfg := svc.Config()
	theme := apptheme.New(cfg)
	theme.WatchUserStyles(application)
	typeface, err := app.Font(cfg.General.FontSans, fontPx)
	if err != nil {
		return fmt.Errorf("files: font %q: %w", cfg.General.FontSans, err)
	}
	stop := svc.Subscribe(func(_, next *config.Config) {
		application.Invoke(func() { theme.SetConfig(next) })
	})
	defer stop()
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("files: find the home folder: %w", err)
	}
	system := dialSystem()
	if system != nil {
		defer func() { _ = system.Close() }()
	}
	h := &host{app: application, db: mime.System(), svc: svc}
	defer h.work.Wait()
	w := &window{app: application, theme: theme, host: h}
	w.cfg = Config{
		Host: h, Bin: bin, KeySym: application.KeySym, Sheet: theme.Sheet,
		Browser: filebrowser.Deps{
			Invoke:         application.Invoke,
			Font:           app.FontFallback(typeface),
			MIME:           h.db,
			Home:           home,
			Places:         func() []filebrowser.Place { return userPlaces(home) },
			Locations:      filebrowser.SystemLocations(home),
			WatchLocations: filebrowser.WatchSystemLocations,
			Drives: func() *udisks.Client {
				if system == nil {
					return nil
				}
				return udisks.New(system)
			},
			ViewFile:      viewFile(),
			BookmarksFile: filebrowser.BookmarksFile(xdg.OS),
			RecentFile:    filebrowser.RecentFile(),
			TrashDirs:     filebrowser.SystemTrashDirs(home),
			ForgetImage:   application.ForgetImage,
			OpenFile:      h.OpenFile,
			Animations:    func() config.AnimationsConfig { return h.svc.Config().Animations },
			Documents:     dochelper.Opener(""),
		},
	}
	defer w.close()

	release := func() {}
	if service && len(paths) == 0 {
		release = application.Hold()
	} else if err := w.show(func(f *Files) { f.Open(paths) }, inv.Token); err != nil {
		return err
	}
	w.release = release
	if stopBus := serveFileManager(w); stopBus != nil {
		defer stopBus()
	}
	if ins != nil {
		forwarded = func(paths, args []string) {
			if len(paths) == 0 && slices.Contains(args, ServiceArg) {
				return
			}
			if err := w.show(func(f *Files) { f.Open(paths) }, ""); err != nil {
				log.Printf("files: %v", err)
			}
		}
		ins.Bind(application)
	}
	return app.Ended(application.Run())
}

// window is the one window a run shows, mapped once asked to.
type window struct {
	app     *app.Application
	theme   *apptheme.Theme
	host    *host
	cfg     Config
	files   *Files
	release func()
}

// show maps the window when it is not yet, then does what it was asked
// on it and raises it with token.
func (w *window) show(do func(*Files), token string) error {
	if w.files == nil {
		f := New(w.cfg)
		win, err := w.app.NewWindow(app.WindowConfig{
			Title: locale.T().Get("files-title"), AppID: AppID, Width: windowWidth, Height: windowHeight,
			Root: f.Root(), Background: w.theme.RenderPalette().Surface,
			OnKey: f.OnKey(), OnPress: f.OnPress,
		})
		if err != nil {
			return fmt.Errorf("files: window: %w", err)
		}
		win.SetCloseRequest(f.CloseRequest)
		w.host.win, w.files = win, f
		if w.release != nil {
			w.release()
		}
	}
	do(w.files)
	switch {
	case token != "":
		w.host.win.Activate(token)
	default:
		w.host.win.RequestAttention()
	}
	return nil
}

// close ends the file manager after the loop.
func (w *window) close() {
	if w.files != nil {
		w.files.Close()
	}
}

// serveFileManager serves org.freedesktop.FileManager1 on the session
// bus; without a bus, or with the name another file manager holds, it
// says so and serves nothing.
func serveFileManager(w *window) (stop func()) {
	conn, err := dbusx.DialSession(context.Background(), busTimeout)
	if err != nil {
		log.Printf("files: FileManager1: %v", err)
		return nil
	}
	release, err := filemanager1.NewDaemon(fileManager{show: func(do func(*Files), token string) {
		w.app.Invoke(func() {
			if err := w.show(do, token); err != nil {
				log.Printf("files: %v", err)
			}
		})
	}}).Export(conn)
	if err != nil {
		log.Printf("files: FileManager1: %v", err)
		_ = conn.Close()
		return nil
	}
	return func() {
		release()
		_ = conn.Close()
	}
}

// fileManager is FileManager1's host: show does each request, raising
// the window with the caller's token.
type fileManager struct {
	show func(do func(*Files), token string)
}

func (m fileManager) ShowFolders(paths []string, token string) {
	m.show(func(f *Files) { f.ShowFolders(paths) }, token)
}

func (m fileManager) ShowItems(paths []string, token string) {
	m.show(func(f *Files) { f.ShowItems(paths) }, token)
}

func (m fileManager) ShowItemProperties(paths []string, token string) {
	m.show(func(f *Files) { f.ShowItemProperties(paths) }, token)
}

// dialSystem connects to the system bus the drives are listed over;
// without one the mounts alone are.
func dialSystem() *dbus.Conn {
	conn, err := dbusx.DialSystem(context.Background(), busTimeout)
	if err != nil {
		log.Printf("files: drives: %v", err)
		return nil
	}
	return conn
}

// userPlaces are the sidebar's places, logging what could not be read.
func userPlaces(home string) []filebrowser.Place {
	places, err := filebrowser.UserPlaces(xdg.OS, home)
	if err != nil {
		log.Printf("files: places: %v", err)
	}
	return places
}

// viewFile keeps the sort and layout across runs, "" without a state
// directory.
func viewFile() string {
	dir, ok := xdg.OS.StateDir("lys-files")
	if !ok {
		log.Print("files: no state directory; the view is not kept")
		return ""
	}
	return filepath.Join(dir, "view")
}

func shipIcons() {
	exe, err := os.Executable()
	if err != nil {
		log.Printf("files: icons: find this program: %v", err)
		return
	}
	dir := filepath.Join(filepath.Dir(exe), "..", "share", "lys-files", "icons")
	widget.SetIconSearchPaths(append(widget.IconSearchPaths(), dir))
}

// host is the file manager's Host over the application and its window.
type host struct {
	app  *app.Application
	win  *app.Window
	db   *mime.Database
	svc  *config.Service
	work sync.WaitGroup
}

func (h *host) Invoke(fn func()) { h.app.Invoke(fn) }

func (h *host) Spawn(fn func()) { h.work.Go(fn) }

func (h *host) Focus(w widget.Widget) error { return h.win.SetFocus(w) }

func (h *host) SetTitle(title string) { h.win.SetTitle(title) }

func (h *host) Close() { h.win.Close() }

func (h *host) HeldMods() app.Mods { return h.app.HeldMods() }

func (h *host) Clipboard() (Clipboard, error) {
	if c := h.app.Clipboard(); c != nil {
		return c, nil
	}
	return nil, app.ErrClipboardUnavailable
}

func (h *host) Ask(q Question) (dismiss func()) {
	answered := false
	d, err := h.app.NewDialog(h.win, app.DialogConfig{
		Title: q.Title, Description: q.Body, Content: q.Content, Buttons: q.Buttons,
		DefaultResponse: q.Default, CancelResponse: q.Cancel, Modal: true,
		OnResponse: func(response string) {
			answered = true
			q.Answer(response)
		},
		OnClosed: func() {
			if !answered {
				answered = true
				q.Answer(q.Cancel)
			}
		},
	})
	if err != nil {
		log.Printf("files: dialog %q: %v", q.Title, err)
		q.Answer(q.Cancel)
		return func() {}
	}
	return d.Close
}

func (h *host) Toast(text string, undo func()) {
	toast, err := h.app.ShowToast(text, toastLife, &app.ToastConfig{Host: h.win, CloseOnPress: true})
	if err != nil {
		log.Printf("files: toast %q: %v", text, err)
		return
	}
	if undo != nil {
		toast.SetAction(locale.T().Get("files-undo"), undo)
	}
}

func (h *host) OpenFile(path string) {
	h.app.OpenPath(path, func(err error) {
		if err == nil {
			return
		}
		log.Printf("files: open %s: %v", path, err)
		h.Toast(locale.T().Get("files-open-failed", i18n.Str("name", filepath.Base(path))), nil)
	})
}

func (h *host) Launch(a desktopentry.App, paths []string) {
	h.app.RequestActivationToken(func(token string, err error) {
		if err != nil {
			log.Printf("files: open in %s without an activation token: %v", a.ID, err)
		}
		terminal := h.svc.Config().Launcher.Terminal
		ctx := desktopentry.LaunchContext{ActivationToken: token, Terminal: func() (string, error) { return launcher.DetectTerminal(terminal) }}
		uris := make([]string, len(paths))
		for i, p := range paths {
			uris[i] = fileuri.FromPath(p)
		}
		h.Spawn(func() {
			if err := a.OpenAll(uris, ctx); err != nil {
				log.Printf("files: %v", err)
				h.Invoke(func() {
					h.Toast(locale.T().Get("files-open-with-failed", i18n.Str("app", a.DisplayName()), i18n.Int("count", int64(len(paths))), i18n.Str("name", filepath.Base(paths[0]))), nil)
				})
			}
		})
	})
}

func (h *host) OpenTerminal(dir string) {
	terminal, err := launcher.DetectTerminal(h.svc.Config().Launcher.Terminal)
	if err != nil {
		log.Printf("files: open a terminal in %s: %v", dir, err)
		h.Toast(locale.T().Get("files-terminal-failed"), nil)
		return
	}
	h.app.RequestActivationToken(func(token string, err error) {
		if err != nil {
			log.Printf("files: open %s without an activation token: %v", terminal, err)
		}
		cmd := exec.Command(terminal)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "XDG_ACTIVATION_TOKEN="+token)
		if err := spawn.Detached(cmd, terminal); err != nil {
			log.Printf("files: open %s in %s: %v", terminal, dir, err)
			h.Toast(locale.T().Get("files-terminal-failed"), nil)
		}
	})
}

func (h *host) Apps(mimeType string) []desktopentry.App {
	return slices.DeleteFunc(desktopentry.RecommendedFor(desktopentry.ApplicationDirs(), desktopentry.MimeAppsLists(), mimeType), func(a desktopentry.App) bool {
		return a.ID == desktopentry.FileID(AppID)
	})
}
