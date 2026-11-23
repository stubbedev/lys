package settings

import (
	"slices"

	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// appListSpec is what an app list edits: the apps its config names and
// the controls each app's row holds.
type appListSpec struct {
	// addKey is the add button's tooltip.
	addKey string
	// emptyIcon and emptyKey say the list names no app.
	emptyIcon, emptyKey string
	// rowClass is each row's class.
	rowClass string
	// extra, when set, sits beside the add button: another way to add.
	extra widget.Widget
	// stored is every app the config names, in the config's order.
	stored func() []string
	// controls are app's row's controls in list, between its face and its
	// remove button.
	controls func(list *appList, app string) []widget.Widget
	// show sets the rows' controls to what the config holds.
	show func(rows []string)
	// drop takes app out of the config, returning what puts it back as it
	// was.
	drop func(app string) (restore func())
}

// appList is a list of apps, one row per app: its face, its controls and
// its remove button, and an add button picking an installed app. An app
// added keeps its row, though the config does not name it yet, until it
// is removed.
type appList struct {
	*widget.Box
	k       *kit
	spec    appListSpec
	apps    installedApps
	pending []string
	rows    []string
	list    *widget.Box
	state   *cardState
	add     *widget.Button
	// syncing is set while show sets the controls, so their handlers
	// write nothing.
	syncing bool
}

func newAppList(k *kit, spec appListSpec) *appList {
	c := &appList{Box: gapBox(widget.Column), k: k, spec: spec, list: gapBox(widget.Column), state: k.cardState()}
	c.add = k.addButton(spec.addKey, c.pickApp)
	controls := gapBox(widget.Row)
	controls.AppendAligned(c.add, false, widget.AlignCenter)
	if spec.extra != nil {
		controls.AppendAligned(spec.extra, true, widget.AlignCenter)
	}
	c.Append(c.state.lone(), false)
	c.Append(c.list, false)
	c.Append(controls, false)
	k.withInstalledApps(func(ix installedApps) {
		c.apps = ix
		c.rebuild()
	})
	c.rebuild()
	return c
}

// named is every app the config names or that was added here, those
// with a row first in their rows' order, then the rest as added, then as
// the config names them.
func (c *appList) named() []string {
	all := slices.Concat(c.pending, c.spec.stored())
	out := slices.DeleteFunc(slices.Clone(c.rows), func(app string) bool { return !slices.Contains(all, app) })
	for _, app := range all {
		if !slices.Contains(out, app) {
			out = append(out, app)
		}
	}
	return out
}

// pickApp offers the installed apps the list does not name yet.
func (c *appList) pickApp() {
	named := c.named()
	apps := slices.DeleteFunc(slices.Clone(c.apps.shown), func(a desktopentry.App) bool { return slices.Contains(named, a.AppID()) })
	openAppPicker(c.k, c.add, apps, func(a desktopentry.App) { c.addApp(a.AppID()) })
}

// addApp gives app a row, unless it has one.
func (c *appList) addApp(app string) {
	if slices.Contains(c.named(), app) {
		return
	}
	c.keep(app)
	c.rebuild()
}

// keep holds app's row while the config does not name it.
func (c *appList) keep(app string) {
	if !slices.Contains(c.pending, app) {
		c.pending = append(c.pending, app)
	}
}

// remove takes app's row away and app out of the config, its Undo
// putting both back.
func (c *appList) remove(app string) {
	at := slices.Index(c.rows, app)
	c.pending = slices.DeleteFunc(c.pending, func(p string) bool { return p == app })
	restore := c.spec.drop(app)
	c.rebuild()
	c.k.offerUndo(c.name(app), func() {
		restore()
		c.keep(app)
		c.rows = insertAt(slices.DeleteFunc(slices.Clone(c.rows), func(r string) bool { return r == app }), at, app)
		c.rebuild()
	})
}

// name is what app is called: its installed app's name, else its id.
func (c *appList) name(app string) string {
	if a, ok := c.apps.byID[app]; ok && a.DisplayName() != "" {
		return a.DisplayName()
	}
	return app
}

func (c *appList) face(app string) *widget.Box {
	if a, ok := c.apps.byID[app]; ok {
		return installedFace(c.k, a)
	}
	return appFace(c.k, "", app, app)
}

func (c *appList) rebuild() {
	c.list.Clear()
	c.rows = c.named()
	if len(c.rows) == 0 {
		c.state.Empty(c.spec.emptyIcon, locale.T().Get(c.spec.emptyKey))
	} else {
		c.state.Clear()
	}
	for _, app := range c.rows {
		row := gapBox(widget.Row)
		row.AddClass(c.spec.rowClass)
		row.AppendAligned(c.face(app), true, widget.AlignCenter)
		for _, w := range c.spec.controls(c, app) {
			row.AppendAligned(w, false, widget.AlignCenter)
		}
		row.AppendAligned(c.k.removeButton("settings-list-remove", func() { c.remove(app) }), false, widget.AlignCenter)
		c.list.Append(row, false)
	}
	c.refresh()
}

func (c *appList) refresh() {
	if !slices.Equal(c.named(), c.rows) {
		c.rebuild()
		return
	}
	c.syncing = true
	defer func() { c.syncing = false }()
	c.spec.show(c.rows)
}
