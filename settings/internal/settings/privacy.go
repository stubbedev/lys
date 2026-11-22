package settings

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/portal/permissions"
	"github.com/stubbedev/lys/desktop/settingspage"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

var windowsPerRow = rangeSpec{
	axis:  linearAxis(1, 12, 1),
	unit:  "settings-range-windows",
	names: [2]string{"settings-share-picker-windows-min-per-row", "settings-share-picker-windows-max-per-row"},
}

func privacyPage(*config.Config) pageSpec {
	return pageSpec{
		id: settingspage.PrivacyID, navKey: "settings-nav-privacy", icon: "ld-eye-symbolic",
		header: "settings-page-privacy",
		sections: []sectionSpec{
			{title: "settings-section-screen-share", rows: []rowSpec{field("portal.screen-cast.reminder-days", durationChips(inDays, after(7*day), after(30*day), after(90*day), zeroAs(never)))}},
			{title: "settings-section-share-picker", preview: sharePickerTry, rows: []rowSpec{
				field("share-picker.default-page", pictureChoice(sharePickerPagePictures)),
				field("share-picker.hide-token-restore", invertedSwitch("settings-share-picker-offer-restore")),
			}},
			{title: "settings-section-share-picker-layout", advanced: true, rows: []rowSpec{
				field("share-picker.width", sizeBase(config.SharePickerWidthBaseRem)),
				field("share-picker.height", sizeBase(config.SharePickerHeightBaseRem)),
				field("share-picker.widget-size", sizeBase(config.SharePickerWidgetBaseRem)),
				field("share-picker.resize-size", countChips(inVideoLines, 360, 480, 720, 1080)),
			}},
			{title: "settings-section-share-picker-windows", advanced: true, rows: []rowSpec{
				field("share-picker.windows-spacing", sizeBase(config.SharePickerWindowsSpacingBaseRem)),
				rangeRow("share-picker.windows-min-per-row", "share-picker.windows-max-per-row", "settings-share-picker-windows-per-row", windowsPerRow),
			}},
			{title: "settings-section-share-picker-outputs", advanced: true, rows: []rowSpec{
				field("share-picker.outputs-spacing", sizeBase(config.SharePickerOutputsSpacingBaseRem)),
				field("share-picker.outputs-show-label"), field("share-picker.outputs-respect-scaling"),
			}},
			{title: "settings-section-lockdown", rows: fields("portal.lockdown.disable-camera", "portal.lockdown.disable-microphone",
				"portal.lockdown.disable-location", "portal.lockdown.disable-sound-output", "portal.lockdown.disable-printing",
				"portal.lockdown.disable-save-to-disk", "portal.lockdown.disable-application-handlers")},
			{title: "settings-section-app-launchers", rows: []rowSpec{field("portal.dynamic-launcher.install-token-apps", stringList)}},
			{title: "settings-section-clipboard", rows: fields("clipboard.persist", "clipboard.history-mib", "clipboard.pinned-mib")},
		},
		footer:   grantsFooter,
		related:  linksTo(privacyLinks...),
		animates: "animations.share-picker",
	}
}

var sharePickerPreviewArgs = []string{"portal", "show", "screen-cast-preview"}

func sharePickerTry(k *kit) widget.Widget {
	return newTryButton(k, "settings-share-picker-preview", func() error { return k.shellCommand(sharePickerPreviewArgs...) })
}

type grantStore interface {
	List(ctx context.Context) ([]permissions.App, error)
	Revoke(ctx context.Context, app string, table permissions.Table, id string) (int, error)
	Set(ctx context.Context, app string, table permissions.Table, id string, perms []string) error
	Watch(ctx context.Context, changed func(), failed func(error)) (*dbusx.Watch, error)
}

const grantsTimeout = 5 * time.Second

var (
	switchedTables   = []permissions.Table{permissions.Background, permissions.Notifications}
	readOnlyTables   = []permissions.Table{permissions.GameMode, permissions.Realtime}
	permissionYes    = []string{"yes"}
	permissionNo     = []string{"no"}
	grantsByResource = 0
	grantsByApp      = 1
)

var openGrants = func() (grantStore, func(), error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, nil, fmt.Errorf("connect to the session bus: %w", err)
	}
	return permissions.Store{Conn: conn, ShortcutsPath: permissions.ShortcutsPath()}, func() { _ = conn.Close() }, nil
}

func withStore[T any](store grantStore, fn func(context.Context, grantStore) (T, error)) (T, error) {
	ctx, cancel := context.WithTimeout(context.Background(), grantsTimeout)
	defer cancel()
	return fn(ctx, store)
}

type grantsListing struct {
	apps []permissions.App
	err  error
}

type grantsList struct {
	*widget.Box
	k         *kit
	store     grantStore
	release   func()
	watch     *dbusx.Watch
	rows      *widget.ItemGroup
	state     *cardState
	view      *choicePick
	apps      []permissions.App
	installed installedApps
	opened    map[privacyResource]bool
	inUse     *inUseStrip
	// failedKey and failure are the change to a grant that failed last,
	// shown with the next render.
	failedKey string
	failure   error
	// unreadable is why the grants cannot be listed, nil while they can.
	unreadable error
}

// unidentifiedApp is the app ID xdg-desktop-portal records for a caller
// whose ID it could not read.
const unidentifiedApp = "[Invalid UTF-8]"

var privacyLinks = []pageLink{
	{"lock", "settings-privacy-link-lock-detail"},
	{"launcher-modes", "settings-privacy-link-launcher-detail"},
	{"media", "settings-privacy-link-media-detail"},
}

func grantsFooter(k *kit) widget.Widget {
	t := locale.T()
	l := &grantsList{Box: sectionStack(), k: k, rows: widget.NewItemGroup(), state: k.cardState(), opened: map[privacyResource]bool{}}
	l.view = newChoicePick(k, []string{t.Get("settings-privacy-by-permission"), t.Get("settings-privacy-by-app")}, grantsByResource)
	l.view.SetAccessibleName(t.Get("settings-privacy-view"))
	l.view.OnSelect = func(int) { l.render() }
	l.inUse = newInUseStrip(l)
	l.Append(l.inUse, false)
	body := widget.NewItemGroup(k.row(t.Get("settings-privacy-view"), "").withControl(l.view), l.rows)
	l.Append(newSection(k, "settings-section-app-permissions", body), false)
	k.withInstalledApps(func(ix installedApps) {
		l.installed = ix
		l.render()
		l.inUse.render()
	})
	store, release, err := openGrants()
	if err != nil {
		l.unreadable = err
		l.render()
		return l
	}
	l.store, l.release = store, release
	watch, err := store.Watch(context.Background(), func() { k.invoke(l.reload) }, func(err error) {
		log.Printf("settings: a permission store signal: %v", err)
	})
	if err != nil {
		log.Printf("settings: follow the permission store: %v", err)
	}
	l.watch = watch
	l.reload()
	return l
}

func (l *grantsList) close() {
	l.inUse.close()
	if l.watch != nil {
		if err := l.watch.Stop(); err != nil {
			log.Printf("settings: stop following the permission store: %v", err)
		}
		l.watch = nil
	}
	if l.release != nil {
		l.release()
		l.release = nil
	}
	l.store = nil
}

func (l *grantsList) reload() {
	if l.store == nil {
		return
	}
	store := l.store
	async(l.k, func() grantsListing {
		apps, err := withStore(store, func(ctx context.Context, s grantStore) ([]permissions.App, error) { return s.List(ctx) })
		return grantsListing{apps, err}
	}, func(r grantsListing) {
		l.apps, l.unreadable = r.apps, r.err
		if r.err != nil {
			log.Printf("settings: list the apps' grants: %v", r.err)
		}
		l.render()
	})
}

func (l *grantsList) render() {
	t := locale.T()
	l.rows.Clear()
	l.rows.Append(l.state)
	group := l.rows
	switch {
	case l.unreadable != nil:
		l.state.Unavailable("ld-shield-check-symbolic", t.Get("settings-privacy-unavailable"), l.unreadable)
		return
	case l.failure != nil:
		l.state.Error(t.Get(l.failedKey), l.failure, nil)
		l.failure = nil
	case len(l.apps) == 0:
		l.state.Empty("ld-shield-check-symbolic", t.Get("settings-privacy-none"))
		return
	default:
		l.state.Clear()
	}
	if l.view.Selected() == grantsByResource {
		l.renderByResource(group)
		return
	}
	for _, app := range l.apps {
		group.Append(l.appHeader(app.ID))
		for _, g := range app.Grants {
			group.Append(l.grantRow(app.ID, g, t.Get("settings-privacy-table-"+string(g.Table))+" · "+g.ID))
		}
	}
}

// appTitle names the app an ID stands for: its installed name, the apps
// outside a sandbox for the empty ID, or the ID itself.
func (l *grantsList) appTitle(id string) string {
	t := locale.T()
	switch id {
	case "":
		return t.Get("settings-privacy-host-apps")
	case unidentifiedApp:
		return t.Get("settings-privacy-unknown-app")
	}
	if a, ok := l.installed.byID[id]; ok && a.DisplayName() != "" {
		return a.DisplayName()
	}
	return id
}

func (l *grantsList) appHeader(id string) *settingRow {
	title := l.appTitle(id)
	detail := ""
	if title != id && id != unidentifiedApp {
		detail = id
	}
	header := l.k.row(title, detail)
	header.AddClass("settings-privacy-app")
	header.SetMedia(widget.NewThemeIcon(l.appIcon(id), appIconPx), true)
	return header
}

func (l *grantsList) appIcon(id string) string {
	if a, ok := l.installed.byID[id]; ok && a.Icon() != "" {
		return a.Icon()
	}
	return appFallbackIcon
}

func (l *grantsList) renderByResource(group *widget.ItemGroup) {
	t := locale.T()
	for _, r := range privacyResources {
		held := resourceGrants(l.apps, r)
		if len(held) == 0 && !r.core {
			continue
		}
		header := l.k.row(t.Get(r.key), t.Get("settings-privacy-resource-apps", i18n.Int("count", countApps(held))))
		header.AddClass("settings-privacy-app")
		header.SetMedia(l.k.icon(r.icon), false)
		group.Append(header)
		if len(held) == 0 {
			continue
		}
		l.k.rowDisclosure(header.Item, l.opened[r], func(open bool) {
			if open != l.opened[r] {
				l.opened[r] = open
				l.render()
			}
		})
		if !l.opened[r] {
			continue
		}
		for _, h := range held {
			row := l.grantRow(h.app, h.grant, l.appTitle(h.app))
			row.SetMedia(widget.NewThemeIcon(l.appIcon(h.app), appIconPx), true)
			group.Append(row)
		}
	}
}

func (l *grantsList) grantRow(app string, g permissions.Grant, title string) *settingRow {
	t := locale.T()
	row := l.k.row(title, grantWords(g))
	row.AddClass("settings-privacy-grant")
	switch {
	case slices.Contains(readOnlyTables, g.Table):
		row.withControl(l.k.label(t.Get("settings-privacy-read-only"), css.Caption, css.DimLabel))
	case slices.Contains(switchedTables, g.Table):
		sw := widget.NewSwitch(slices.Contains(strings.Split(g.Value, ","), "yes"))
		sw.SetAccessibleName(app + " " + t.Get("settings-privacy-table-"+string(g.Table)))
		sw.OnChanged = func(on bool) {
			perms := permissionNo
			if on {
				perms = permissionYes
			}
			l.act(func(ctx context.Context, s grantStore) error { return s.Set(ctx, app, g.Table, g.ID, perms) }, "settings-privacy-set-failed")
		}
		row.withControl(sw)
	default:
		var revoke *widget.Button
		revoke = l.k.removeButton("settings-privacy-revoke", func() { l.confirmRevoke(revoke, app, g) })
		row.withControl(revoke)
	}
	return row
}

// confirmRevoke asks before revoking app's grant g, revoke going inert
// while the revocation is on its way.
func (l *grantsList) confirmRevoke(revoke *widget.Button, app string, g permissions.Grant) {
	what := fmt.Sprintf("revoke the %s grant %s of %q", g.Table, g.ID, app)
	err := l.k.confirm(what, "settings-privacy-revoke-grant", func() {
		revoke.SetEnabled(false)
		l.act(func(ctx context.Context, s grantStore) error {
			_, err := s.Revoke(ctx, app, g.Table, g.ID)
			return err
		}, "settings-privacy-revoke-failed")
	}, i18n.Str("app", l.appTitle(app)), i18n.Str("permission", locale.T().Get("settings-privacy-table-"+string(g.Table))))
	if err != nil {
		l.failedKey, l.failure = "settings-privacy-revoke-failed", err
		l.render()
	}
}

func grantWords(g permissions.Grant) string {
	if g.Table == permissions.Shortcuts {
		return g.Value
	}
	t := locale.T()
	tokens := strings.Split(g.Value, ",")
	words := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if at, err := strconv.ParseInt(token, 10, 64); err == nil && g.Table == permissions.Location {
			words = append(words, t.Get("settings-privacy-last-used", i18n.Str("date", time.Unix(at, 0).Format(time.DateOnly))))
			continue
		}
		words = append(words, t.Get("settings-privacy-value", i18n.Str("value", strings.ToLower(token))))
	}
	return strings.Join(words, ", ")
}

func (l *grantsList) act(change func(context.Context, grantStore) error, failedKey string) {
	store := l.store
	async(l.k, func() error {
		_, err := withStore(store, func(ctx context.Context, s grantStore) (struct{}, error) { return struct{}{}, change(ctx, s) })
		return err
	}, func(err error) {
		if err != nil {
			log.Printf("settings: change a grant: %v", err)
			l.failedKey, l.failure = failedKey, err
		}
		l.reload()
	})
}

func wrapped(l *widget.Label) *widget.Label {
	l.SetWrap(true)
	return l
}
