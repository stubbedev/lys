package settings

import (
	"cmp"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	defaultAppsID  = "default-apps"
	fileTypesShown = 40
)

type appCategory struct {
	key   string
	types []string
}

var appCategories = []appCategory{
	{"settings-default-web", []string{"x-scheme-handler/https", "x-scheme-handler/http", "text/html"}},
	{"settings-default-mail", []string{"x-scheme-handler/mailto"}},
	{"settings-default-calendar", []string{"text/calendar"}},
	{"settings-default-music", []string{"audio/mpeg", "audio/flac", "audio/ogg"}},
	{"settings-default-video", []string{"video/mp4", "video/x-matroska", "video/webm"}},
	{"settings-default-images", []string{"image/png", "image/jpeg", "image/webp"}},
	{"settings-default-pdf", []string{"application/pdf"}},
	{"settings-default-text", []string{"text/plain"}},
	{"settings-default-files", []string{"inode/directory"}},
}

func defaultAppsPage(*config.Config) pageSpec {
	return pageSpec{
		id: defaultAppsID, navKey: "settings-nav-default-apps", icon: "ld-app-window-symbolic",
		header: "settings-page-default-apps",
		sections: []sectionSpec{
			{title: "settings-section-default-terminal", rows: []rowSpec{field("launcher.terminal", terminalRow)}},
		},
		footer: defaultAppsFooter,
	}
}

var scanApps = desktopentry.AllApps

type appIndex struct {
	all    []desktopentry.App
	byType map[string][]desktopentry.App
	types  []string
	mime   desktopentry.MimeApps
	user   desktopentry.UserAssociations
	names  map[string]string
}

func indexApps(dirs, lists []string, describe func(mimeType string) string) appIndex {
	all := scanApps(dirs)
	ix := appIndex{byType: map[string][]desktopentry.App{}, mime: desktopentry.ReadMimeApps(lists), names: map[string]string{}}
	for _, a := range all {
		if !a.ShouldShow() && len(a.MimeTypes()) == 0 {
			continue
		}
		ix.all = append(ix.all, a)
		for _, t := range a.MimeTypes() {
			ix.byType[t] = append(ix.byType[t], a)
		}
	}
	for t := range ix.byType {
		ix.types = append(ix.types, t)
		ix.names[t] = cmp.Or(describe(t), t)
	}
	slices.SortFunc(ix.types, func(a, b string) int {
		return cmp.Or(cmp.Compare(strings.ToLower(ix.names[a]), strings.ToLower(ix.names[b])), cmp.Compare(a, b))
	})
	return ix
}

func mimeDescriber() func(string) string {
	db := mime.Load(mime.DataDirs())
	var langs []string
	for _, l := range locale.T().Languages() {
		langs = append(langs, l.String())
	}
	return func(mimeType string) string { return db.CommentIn(mimeType, langs) }
}

func (ix appIndex) candidates(types []string) []desktopentry.App {
	var out []desktopentry.App
	for _, a := range ix.all {
		mimes := a.MimeTypes()
		if slices.ContainsFunc(types, func(t string) bool { return slices.Contains(mimes, t) }) {
			out = append(out, a)
		}
	}
	slices.SortFunc(out, byDisplayName)
	return out
}

func (ix appIndex) current(mimeType string) string {
	if a, ok := ix.mime.Default(ix.all, mimeType); ok {
		return a.ID
	}
	return ""
}

func (ix appIndex) currentOf(types []string) (id string, mixed bool) {
	id = ix.current(types[0])
	for _, t := range types[1:] {
		if ix.current(t) != id {
			return "", true
		}
	}
	return id, false
}

type defaultApps struct {
	k       *kit
	ix      appIndex
	cats    *widget.Box
	types   *widget.Box
	filter  *widget.SearchEntry
	problem *widget.Label
}

var (
	setDefaults          = desktopentry.SetDefaults
	resetAssociations    = desktopentry.ResetTypeAssociations
	readUserAssociations = desktopentry.ReadUserAssociations
)

type indexedApps struct {
	ix  appIndex
	err error
}

func withUserAssociations(ix appIndex) indexedApps {
	user, err := readUserAssociations()
	ix.user = user
	return indexedApps{ix, err}
}

func defaultAppsFooter(k *kit) widget.Widget {
	d := &defaultApps{k: k}
	content := sectionStack()
	d.problem = wrapped(k.label("", "setting-problem", css.Caption, css.Error))
	d.problem.SetVisible(false)
	content.Append(d.problem, false)
	d.cats = gapBox(widget.Column)
	content.Append(newSection(k, "settings-section-default-categories", d.cats), false)
	typesBody := widget.NewBox(widget.Column, 0, 0)
	d.filter = widget.NewSearchEntry(k.face, k.textPx(), locale.T().Get("settings-default-types-filter"))
	d.filter.SetDelay(0)
	d.filter.OnSearchChanged = func(string) { d.showTypes() }
	d.types = gapBox(widget.Column)
	typesBody.Append(insetRow(d.filter), false)
	typesBody.Append(widget.NewSeparator(widget.Horizontal), false)
	typesBody.Append(d.types, false)
	content.Append(newSection(k, "settings-section-default-types", typesBody), false)
	dirs, lists := desktopentry.ApplicationDirs(), desktopentry.MimeAppsLists()
	async(k, func() indexedApps { return withUserAssociations(indexApps(dirs, lists, mimeDescriber())) }, d.reread)
	return content
}

func (d *defaultApps) fail(err error) {
	d.problem.SetVisible(err != nil)
	if err != nil {
		d.problem.SetText(err.Error())
	}
}

func (d *defaultApps) reread(r indexedApps) {
	d.fail(r.err)
	d.show(r.ix)
}

func (d *defaultApps) write(change func() error) {
	ix := d.ix
	async(d.k, func() indexedApps {
		if err := change(); err != nil {
			return indexedApps{ix, err}
		}
		ix.mime = desktopentry.ReadMimeApps(desktopentry.MimeAppsLists())
		return withUserAssociations(ix)
	}, d.reread)
}

func (d *defaultApps) show(ix appIndex) {
	d.ix = ix
	d.cats.Clear()
	rows := widget.NewItemGroup()
	t := locale.T()
	for _, c := range appCategories {
		rows.Append(d.chooser(t.Get(c.key), c.types))
	}
	d.cats.Append(rows, false)
	d.showTypes()
}

func (d *defaultApps) showTypes() {
	d.types.Clear()
	filter := newWordFilter(d.filter.Text())
	rows := widget.NewItemGroup()
	d.types.Append(rows, false)
	shown := 0
	for _, mimeType := range d.ix.types {
		name := d.ix.names[mimeType]
		if !filter.matches(mimeType, name) {
			continue
		}
		if shown == fileTypesShown {
			rows.Append(widget.NewItem(d.k.face, "", locale.T().Get("settings-default-types-more")))
			break
		}
		rows.Append(d.chooser(name, []string{mimeType}))
		shown++
	}
}

func (d *defaultApps) chooser(title string, types []string) widget.Widget {
	apps := d.ix.candidates(types)
	row := gapBox(widget.Row)
	row.SetElement("item")
	row.AddClass(css.Small, "setting-row", "default-app")
	row.AppendAligned(d.k.label(title, "setting-label"), true, widget.AlignCenter)
	if d.ix.user.Overrides(types) {
		reset := d.k.button(d.k.label(locale.T().Get("settings-default-reset")), func() {
			d.write(func() error { return resetAssociations(types) })
		}, css.Small, css.Flat)
		reset.SetAccessibleName(locale.T().Get("settings-default-reset-named", i18n.Str("name", title)))
		reset.SetTooltip(locale.T().Get("settings-default-reset-tooltip"))
		row.AppendAligned(reset, false, widget.AlignCenter)
	}
	if len(apps) == 0 {
		row.AppendAligned(d.k.label(locale.T().Get("settings-default-none"), css.DimLabel), false, widget.AlignCenter)
		return row
	}
	current, mixed := d.ix.currentOf(types)
	items := make([]widget.DropdownRow, 0, len(apps)+1)
	selected := 0
	switch {
	case mixed:
		items = append(items, widget.DropdownRow{Label: locale.T().Get("settings-default-mixed")})
	case !slices.ContainsFunc(apps, func(a desktopentry.App) bool { return a.ID == current }):
		items = append(items, widget.DropdownRow{Label: locale.T().Get("settings-default-unset")})
	}
	for _, a := range apps {
		if a.ID == current {
			selected = len(items)
		}
		icon := a.Icon()
		items = append(items, widget.DropdownRow{Label: a.DisplayName(), Leading: func() widget.Widget { return widget.NewThemeIcon(icon, appIconPx) }})
	}
	offset := len(items) - len(apps)
	dd := widget.NewDropdownRows(d.k.face, d.k.textPx(), items, selected)
	dd.SetAccessibleName(title)
	dd.OnSelect = func(i int) {
		if i < offset {
			return
		}
		id := apps[i-offset].ID
		d.write(func() error { return setDefaults(id, types) })
	}
	row.AppendAligned(dd, false, widget.AlignCenter)
	return row
}
