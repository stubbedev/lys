package viewer

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/fileops"
	"github.com/stubbedev/lys/desktop/fileuri"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/logging"
	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/desktop/thumbnail"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/transfer"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/viewer/internal/locale"
)

const (
	windowWidth, windowHeight = 1024, 768
	fontPx                    = 14
	toastLife                 = 5 * time.Second
)

func Run(paths []string) error {
	var reopen func(paths []string)
	ins, primary, err := app.ClaimInstance(app.InstanceConfig{
		AppID:  AppID,
		OnOpen: func(paths []string, _ string) { reopen(paths) },
	}, app.Invocation{Open: paths, Token: os.Getenv("XDG_ACTIVATION_TOKEN")})
	switch {
	case err != nil:
		log.Printf("viewer: single instance: %v", err)
	case !primary:
		return nil
	}
	bin, err := fileops.SystemBin()
	if err != nil {
		return fmt.Errorf("viewer: %w", err)
	}
	svc, err := config.Open()
	if err != nil {
		return fmt.Errorf("viewer: load the desktop config: %w", err)
	}
	defer svc.Close()
	app.SetLogger(logging.Library())
	sess, err := app.Connect()
	if err != nil {
		return fmt.Errorf("viewer: connect: %w", err)
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
		return fmt.Errorf("viewer: font %q: %w", cfg.General.FontSans, err)
	}
	face := app.FontFallback(typeface)
	db := mime.System()
	sources := preview.Sources{MIME: db, Thumbnails: thumbnail.New(thumbnail.Config{}), Pictures: preview.NewPictures(preview.PictureBudget)}
	h := &host{app: application, db: db, face: face}
	defer h.work.Wait()
	v := New(Config{
		Font: face, MIME: db, Bin: bin, Host: h,
		Load: func(ctx context.Context, req preview.Request) preview.Content { return preview.Load(ctx, sources, req) },
	})
	defer v.Close()
	theme.Attach(v.Root())
	h.win, err = application.NewWindow(app.WindowConfig{
		Title: "lys-viewer", AppID: AppID, Width: windowWidth, Height: windowHeight,
		Root: v.Root(), Background: theme.RenderPalette().Surface,
		ContentType: app.ContentPhoto, KeyCapture: v.Key,
	})
	if err != nil {
		return fmt.Errorf("viewer: window: %w", err)
	}
	stop := svc.Subscribe(func(_, next *config.Config) {
		application.Invoke(func() { theme.SetConfig(next) })
	})
	defer stop()
	opens := newOpens(db)
	show := func(paths []string) {
		c, err := Open(opens, paths)
		if err != nil {
			log.Printf("viewer: %v", err)
		}
		v.Open(c)
	}
	show(paths)
	if ins != nil {
		reopen = func(paths []string) {
			show(paths)
			h.win.RequestAttention()
		}
		ins.Bind(application)
	}
	return app.Ended(application.Run())
}

func shipIcons() {
	exe, err := os.Executable()
	if err != nil {
		log.Printf("viewer: icons: find this program: %v", err)
		return
	}
	dir := filepath.Join(filepath.Dir(exe), "..", "share", "lys-viewer", "icons")
	widget.SetIconSearchPaths(append(widget.IconSearchPaths(), dir))
}

type host struct {
	app  *app.Application
	win  *app.Window
	db   *mime.Database
	face render.Font
	work sync.WaitGroup
}

func (h *host) Invoke(fn func()) { h.app.Invoke(fn) }

func (h *host) Spawn(fn func()) { h.work.Go(fn) }

func (h *host) SetTitle(title string) { h.win.SetTitle(title) }

func (h *host) Toast(text string, undo func()) {
	toast, err := h.app.ShowToast(text, toastLife, &app.ToastConfig{Host: h.win, CloseOnPress: true})
	if err != nil {
		log.Printf("viewer: toast %q: %v", text, err)
		return
	}
	if undo != nil {
		toast.SetAction(locale.T().Get("viewer-undo"), undo)
	}
}

func (h *host) Copy(content transfer.Content) error {
	clip := h.app.Clipboard()
	if clip == nil {
		return app.ErrClipboardUnavailable
	}
	return clip.Write(content)
}

func (h *host) SetFullscreen(on bool) {
	if on {
		h.win.Fullscreen()
		return
	}
	h.win.Unfullscreen()
}

func (h *host) OpenWith(anchor widget.Boundser, path string) {
	h.Spawn(func() {
		mimeType, err := h.db.TypeOfFile(path)
		if err != nil {
			log.Printf("viewer: the type of %s, taken from its name: %v", path, err)
			mimeType = h.db.TypeByName(path)
		}
		apps := otherApps(desktopentry.ApplicationDirs(), desktopentry.MimeAppsLists(), mimeType)
		h.Invoke(func() { h.openWithMenu(anchor, path, apps) })
	})
}

func (h *host) openWithMenu(anchor widget.Boundser, path string, apps []desktopentry.App) {
	items := make([]widget.MenuItem, 0, max(len(apps), 1))
	for _, a := range apps {
		items = append(items, widget.MenuItem{Label: a.DisplayName(), OnClick: func() { h.launch(a, path) }})
	}
	if len(items) == 0 {
		items = append(items, widget.MenuItem{Label: locale.T().Get("viewer-no-apps"), Disabled: true})
	}
	if _, err := h.app.OpenMenuPopover(h.win, app.MenuPopoverConfig{Anchor: anchor, Face: h.face, Items: items}); err != nil {
		log.Printf("viewer: open with: %v", err)
	}
}

func (h *host) launch(a desktopentry.App, path string) {
	h.app.RequestActivationToken(func(token string, err error) {
		if err != nil {
			log.Printf("viewer: open %s in %s without an activation token: %v", path, a.ID, err)
		}
		h.Spawn(func() {
			err := a.Open(fileuri.FromPath(path), desktopentry.LaunchContext{ActivationToken: token})
			if err == nil {
				return
			}
			log.Printf("viewer: %v", err)
			h.Invoke(func() {
				h.Toast(locale.T().Get("viewer-open-failed", i18n.Str("app", a.DisplayName()), i18n.Str("name", filepath.Base(path))), nil)
			})
		})
	})
}
