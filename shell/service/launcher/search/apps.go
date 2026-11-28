package search

import (
	"context"
	"strings"

	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shlex"

	"github.com/stubbedev/lys/shell/service/launcher"
)

// appIcon is an entry's named icon or file.
func appIcon(raw, fallback string) launcher.Icon {
	switch {
	case raw == "":
		return launcher.IconName(fallback)
	case strings.HasPrefix(raw, "/"):
		return launcher.IconFile(raw)
	}
	return launcher.IconName(raw)
}

const appFallbackIcon = "ld-app-window-symbolic"

// appWeight lifts apps over an equal match elsewhere: most searches are
// for an app.
const appWeight = 12

// Apps offers the desktop entries, links included.
type Apps struct {
	env  Env
	dirs []string
}

// NewApps searches dirs (desktopentry.ApplicationDirs).
func NewApps(env Env, dirs []string) *Apps { return &Apps{env: env, dirs: dirs} }

// Spec implements Provider.
func (*Apps) Spec() Spec { return Spec{Section: SectionApps, Weight: appWeight} }

// appData is what a row launches: an app, or a Type=Link's URL.
type appData struct {
	app *desktopentry.App
	url string
}

// Whole implements Whole.
func (*Apps) Whole() {}

// Load implements Provider.
func (a *Apps) Load(_ context.Context, publish func([]Entry)) {
	apps := desktopentry.AllApps(a.dirs)
	links := desktopentry.Links(a.dirs)
	entries := make([]Entry, 0, len(apps)+len(links))
	for i := range apps {
		app := &apps[i]
		if !app.ShouldShow() {
			continue
		}
		id := "app:" + app.ID
		subtitle := app.Comment()
		if subtitle == "" {
			subtitle = app.GenericName()
		}
		var exec string
		if argv, ok := shlex.Split(app.Exec()); ok && len(argv) > 0 {
			exec = argv[0][strings.LastIndexByte(argv[0], '/')+1:]
		}
		entries = append(entries, Entry{
			ID: id, Learn: id, Title: app.DisplayName(), Subtitle: subtitle,
			Keywords: strings.Join(append(app.Keywords(), app.GenericName(), exec), " "),
			Icon:     appIcon(app.Icon(), appFallbackIcon),
			Data:     appData{app: app},
		})
	}
	for _, l := range links {
		id := "link:" + l.ID
		entries = append(entries, Entry{
			ID: id, Learn: id, Title: l.Name, Subtitle: l.URL, Icon: appIcon(l.Icon, "ld-link-symbolic"),
			Data: appData{url: l.URL},
		})
	}
	publish(entries)
}

// launch runs app, in the env's terminal when it asks for one, or in it
// regardless when the action says so.
func (a *Apps) launch(app *desktopentry.App, terminal bool) Outcome {
	if !terminal || app.Terminal() {
		if err := app.Launch(launcher.AppContext(a.env.Terminal, "")); err != nil {
			return failed(app.DisplayName(), err)
		}
		return Close{}
	}
	argv, err := app.ExpandExec(app.Exec())
	if err != nil {
		return failed(app.DisplayName(), err)
	}
	return a.env.inTerminal(app.DisplayName(), argv...)
}

// Actions implements Provider.
func (a *Apps) Actions(e Entry) []Action {
	d, _ := e.Data.(appData)
	if d.app == nil {
		return []Action{
			{Title: i18n.T("launcher-action-open"), Icon: "ld-external-link-symbolic", Run: func(context.Context) Outcome {
				return a.env.open(i18n.T("launcher-action-open"), d.url)
			}},
			copyAction(i18n.T("launcher-action-copy-url"), d.url),
		}
	}
	app := d.app
	out := []Action{{Title: i18n.T("launcher-action-open"), Icon: "ld-external-link-symbolic", Run: func(context.Context) Outcome {
		return a.launch(app, false)
	}}}
	for _, action := range app.Actions() {
		out = append(out, Action{Title: app.ActionName(action), Icon: "ld-play-symbolic", Run: func(context.Context) Outcome {
			if err := app.LaunchAction(action, launcher.AppContext(a.env.Terminal, "")); err != nil {
				return failed(app.ActionName(action), err)
			}
			return Close{}
		}})
	}
	if !app.Terminal() {
		out = append(out, Action{Title: i18n.T("launcher-action-run-in-terminal"), Icon: "ld-terminal-symbolic", Run: func(context.Context) Outcome {
			return a.launch(app, true)
		}})
	}
	return append(out, copyAction(i18n.T("launcher-action-copy-name"), app.DisplayName()))
}

// copyAction copies text and closes.
func copyAction(title, text string) Action {
	return Action{Title: title, Icon: "ld-copy-symbolic", Run: func(context.Context) Outcome { return Copy{Text: text} }}
}
