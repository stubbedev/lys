package settings

import (
	"cmp"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	defaultAppsID  = "default-apps"
	fileTypesShown = 40
	terminalPath   = "launcher.terminal"
	terminalIcon   = "ld-terminal-symbolic"
	appTileIconPx  = 32
	appTileW       = 168
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
		header:   "settings-page-default-apps",
		sections: []sectionSpec{{title: "settings-section-removable-drives", rows: fields("drives.automount")}},
		footer:   defaultAppsFooter,
		panel:    []panelSetting{{terminalPath, "settings-default-terminal", "settings-section-default-categories"}},
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

func (ix appIndex) terminals() []desktopentry.App {
	var out []desktopentry.App
	for _, a := range ix.all {
		if _, runs := execProgram(a.Exec()); runs && a.ShouldShow() && isTerminal(a) {
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
	*widget.Box
	k       *kit
	ix      appIndex
	cats    *widget.Box
	types   *widget.ItemGroup
	shown   []widget.Widget
	filter  *widget.SearchEntry
	problem *inlineState
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
	d := &defaultApps{Box: sectionStack(), k: k}
	d.problem = k.inlineState()
	d.Append(d.problem, false)
	d.cats = gapBox(widget.Column)
	d.Append(newSection(k, "settings-section-default-categories", d.cats), false)
	d.filter = widget.NewSearchEntry(k.face, k.textPx(), locale.T().Get("settings-default-types-filter"))
	d.filter.SetDelay(0)
	d.filter.OnSearchChanged = func(string) { d.showTypes() }
	d.types = widget.NewItemGroup(newInsetRow(d.filter))
	d.Append(newSection(k, "settings-section-default-types", d.types), false)
	dirs, lists := desktopentry.ApplicationDirs(), desktopentry.MimeAppsLists()
	async(k, func() indexedApps { return withUserAssociations(indexApps(dirs, lists, mimeDescriber())) }, d.reread)
	return d
}

func (d *defaultApps) fail(err error) {
	d.problem.Result(locale.T().Get("settings-default-apps-failed"), err)
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
	tiles := widget.NewWrapBox(choiceCardGap, choiceCardGap, widget.JustifyEqual)
	tiles.AddClass("default-app-tiles")
	t := locale.T()
	for _, c := range appCategories {
		tiles.Append(d.categoryTile(t.Get(c.key), c.types))
	}
	if d.k.parts.shell {
		tiles.Append(d.terminalTile())
	}
	d.cats.Append(newInsetRow(tiles), false)
	d.showTypes()
}

type appTile struct {
	*widget.Box
	button *widget.Button
	title  *widget.Label
	app    *widget.Label
}

func (d *defaultApps) tile(title, icon, app string, pick func(anchor widget.Widget), reset func()) *appTile {
	th := widget.Current()
	face := widget.NewBox(widget.Column, th.Spacing/2, 0)
	face.SetPadding(render.Insets{Top: th.Spacing, Bottom: th.Spacing, Left: th.Padding, Right: th.Padding})
	face.AppendAligned(widget.NewThemeIcon(cmp.Or(icon, appFallbackIcon), appTileIconPx), false, widget.AlignCenter)
	tile := &appTile{Box: widget.NewBox(widget.Column, th.Spacing/2, 0), title: d.k.label(title), app: d.k.label(app, css.Caption, css.DimLabel)}
	for _, l := range []*widget.Label{tile.title, tile.app} {
		l.SetEllipsize(widget.EllipsizeEnd)
		face.AppendAligned(l, false, widget.AlignCenter)
	}
	tile.button = d.k.roleButton(roleTile, face, nil)
	tile.button.SetSizeRequest(appTileW, -1)
	tile.button.SetAccessibleName(title)
	tile.button.SetTooltip(app)
	if pick == nil {
		tile.button.SetEnabled(false)
	} else {
		tile.button.OnClick = func() {
			if d.k.pickers != nil {
				pick(tile.button)
			}
		}
	}
	tile.Append(tile.button, false)
	if reset != nil {
		tile.AppendAligned(d.resetFor(title, reset), false, widget.AlignCenter)
	}
	return tile
}

func (d *defaultApps) resetFor(title string, reset func()) *widget.Button {
	b := d.k.resetButton("settings-default-reset-tooltip", reset).Button
	b.SetAccessibleName(locale.T().Get("settings-default-reset-named", i18n.Str("name", title)))
	return b
}

func (d *defaultApps) typesReset(types []string) func() {
	if !d.ix.user.Overrides(types) {
		return nil
	}
	return func() { d.write(func() error { return resetAssociations(types) }) }
}

func (d *defaultApps) categoryTile(title string, types []string) *appTile {
	t := locale.T()
	reset := d.typesReset(types)
	apps := d.ix.candidates(types)
	if len(apps) == 0 {
		return d.tile(title, "", t.Get("settings-default-none"), nil, reset)
	}
	pick := func(anchor widget.Widget) {
		openAppPicker(d.k, anchor, apps, func(a desktopentry.App) {
			d.write(func() error { return setDefaults(a.ID, types) })
		})
	}
	current, mixed := d.ix.currentOf(types)
	i := slices.IndexFunc(apps, func(a desktopentry.App) bool { return a.ID == current })
	switch {
	case mixed:
		return d.tile(title, "", t.Get("settings-default-mixed"), pick, reset)
	case i < 0:
		return d.tile(title, "", t.Get("settings-default-unset"), pick, reset)
	}
	return d.tile(title, apps[i].Icon(), apps[i].DisplayName(), pick, reset)
}

func (d *defaultApps) terminalTile() *appTile {
	t := locale.T()
	title := t.Get("settings-default-terminal")
	current, _ := d.k.store.value(terminalPath).(string)
	var reset func()
	if !d.k.store.isDefault(terminalPath) {
		reset = func() {
			d.k.store.reset(terminalPath)
			d.fail(nil)
			d.show(d.ix)
		}
	}
	apps := d.ix.terminals()
	var pick func(anchor widget.Widget)
	if len(apps) > 0 {
		pick = func(anchor widget.Widget) {
			openAppPicker(d.k, anchor, apps, func(a desktopentry.App) {
				program, _ := execProgram(a.Exec())
				d.fail(d.k.store.set(terminalPath, program))
				d.show(d.ix)
			})
		}
	}
	if current == "" {
		return d.tile(title, terminalIcon, t.Get("settings-default-terminal-auto"), pick, reset)
	}
	for _, a := range apps {
		if program, _ := execProgram(a.Exec()); program == current {
			return d.tile(title, cmp.Or(a.Icon(), terminalIcon), a.DisplayName(), pick, reset)
		}
	}
	return d.tile(title, terminalIcon, current, pick, reset)
}

func (d *defaultApps) showTypes() {
	for _, w := range d.shown {
		d.types.Remove(w)
	}
	d.shown = d.shown[:0]
	filter := newWordFilter(d.filter.Text())
	for _, mimeType := range d.ix.types {
		name := d.ix.names[mimeType]
		if !filter.matches(mimeType, name) {
			continue
		}
		if len(d.shown) == fileTypesShown {
			d.showType(newInsetRow(d.k.note(locale.T().Get("settings-default-types-more"))))
			break
		}
		d.showType(d.chooser(name, []string{mimeType}))
	}
}

func (d *defaultApps) showType(w widget.Widget) {
	d.types.Append(w)
	d.shown = append(d.shown, w)
}

func (d *defaultApps) chooser(title string, types []string) widget.Widget {
	apps := d.ix.candidates(types)
	row := d.k.row(title, "")
	row.AddClass("default-app")
	if reset := d.typesReset(types); reset != nil {
		row.withActions(d.resetFor(title, reset))
	}
	if len(apps) == 0 {
		row.SetDescription(locale.T().Get("settings-default-none"))
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
	return row.withControl(dd)
}
