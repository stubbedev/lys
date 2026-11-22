package settings

import (
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/launchermatch"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/launcherview"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const launcherPreviewScreenH = 520

// launcherSample is one sample app the preview lists: its name's message
// and an icon the app ships.
type launcherSample struct {
	key, icon string
}

var launcherSamples = []launcherSample{
	{"settings-launcher-sample-files", "ld-folder-symbolic"},
	{"settings-launcher-sample-terminal", "ld-terminal-symbolic"},
	{"settings-launcher-sample-browser", "ld-globe-symbolic"},
	{"settings-launcher-sample-editor", "ld-file-text-symbolic"},
	{"settings-launcher-sample-calculator", "ld-calculator-symbolic"},
	{"settings-launcher-sample-calendar", "ld-calendar-symbolic"},
	{"settings-launcher-sample-mail", "ld-mail-symbolic"},
	{"settings-launcher-sample-music", "ld-music-symbolic"},
	{"settings-launcher-sample-camera", "ld-camera-symbolic"},
	{"settings-launcher-sample-images", "ld-image-symbolic"},
	{"settings-launcher-sample-settings", "ld-settings-symbolic"},
	{"settings-launcher-sample-chat", "ld-message-circle-symbolic"},
}

// launcherSampleRows are the sample apps query matches as the launcher
// matches with l's matching and case, in the order it ranks them.
func launcherSampleRows(query string, l config.LauncherConfig) []launcherview.Row {
	t := locale.T()
	names := make([]string, len(launcherSamples))
	for i, s := range launcherSamples {
		names[i] = t.Get(s.key)
	}
	engine := launchermatch.NewEngine(launchermatch.OptionsOf(l))
	engine.SetTexts(names, nil)
	engine.SetQuery(query)
	matched := engine.Matched()
	rows := make([]launcherview.Row, len(matched))
	for i, at := range matched {
		rows[i] = launcherview.Row{Text: names[at], Icon: launcherview.IconName(launcherSamples[at].icon)}
	}
	return rows
}

// launcherPreviewState is what the preview keeps across redraws of the
// draft: the test query and the style preset it is drawn with.
type launcherPreviewState struct {
	query  string
	style  string
	redraw func()
}

// launcherPreview is lys-shell's launcher drawn from the config the
// window edits, where launcher.location puts it on a small wallpaper,
// listing the sample apps a test query matches with launcher.matching
// and launcher.case.
func launcherPreview(k *kit) widget.Widget {
	state := &launcherPreviewState{}
	p := newLivePreview(k, "settings-launcher-preview", func(k *kit, cfg *config.Config, font render.Font) widget.Widget {
		return drawLauncherOnScreen(k, cfg, font, state)
	})
	state.redraw = p.configChanged
	return p
}

func drawLauncherOnScreen(k *kit, cfg *config.Config, font render.Font, state *launcherPreviewState) widget.Widget {
	t := locale.T()
	presets := launcherStyleNames(cfg)
	if !slices.Contains(presets, state.style) {
		state.style = ""
	}
	look := launcherview.LookOf(cfg)
	look.Style = cfg.Launcher.Styles[state.style]
	px := launcherview.ScaledPx(cfg)
	variants := app.FontVariantsOf(font)
	st := launcherview.RowStyle{Font: font, Variants: variants, IconPx: px(launcherview.SearchIconSize), ShowIcons: look.ShowIcons, Post: k.invoke}
	rows := launcherSampleRows(state.query, cfg.Launcher)
	column := widget.NewBox(widget.Column, widget.Current().Spacing, 0)
	noMatch := k.note("", "settings-preview-state")
	var session *launcherview.Session
	session = launcherview.NewSession(launcherview.Spec{
		Font: font, Variants: variants, Config: cfg, Look: look,
		Row: func(pos int) widget.Widget { return launcherview.NewRow(rows[pos], pos == 0, st) },
		OnQuery: func(query string) {
			state.query = query
			rows = launcherSampleRows(query, cfg.Launcher)
			session.SetRows(len(rows))
			showNoMatch(noMatch, query, len(rows))
		},
	})
	session.Entry.SetPlaceholder(t.Get("settings-launcher-preview-query"))
	session.Entry.SetAccessibleName(t.Get("settings-launcher-preview-query"))
	session.Entry.SetText(state.query)
	if modes := cfg.Launcher.Modes; len(modes) > 0 {
		session.SetChip(launcherview.ModeName(modes[0], cfg.Launcher.DisplayNames))
		if look.Sidebar {
			session.SetTabs(modes, cfg.Launcher.DisplayNames, 0, func(int) {})
		}
	}
	session.SetRows(len(rows))
	showNoMatch(noMatch, state.query, len(rows))

	still := launcherview.NewStill(session)
	if sheet, ok := look.Stylesheet(apptheme.Priority); ok {
		still.AttachStylesheet(sheet)
	}
	screen := wallpaperScreen(launcherPreviewScreenH)
	screen.Append(still)
	if len(presets) > 0 {
		column.Append(launcherStyleChooser(k, presets, state), false)
	}
	column.Append(screen, false)
	column.Append(noMatch, false)
	return column
}

func showNoMatch(l *widget.Label, query string, rows int) {
	l.SetVisible(rows == 0)
	if rows == 0 {
		l.SetText(locale.T().Get("settings-launcher-preview-no-match", i18n.Str("query", query)))
	}
}

// launcherStyleNames are launcher.styles' presets in order.
func launcherStyleNames(cfg *config.Config) []string {
	names := make([]string, 0, len(cfg.Launcher.Styles))
	for name := range cfg.Launcher.Styles {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// launcherStyleChooser picks the preset the preview draws, or none.
func launcherStyleChooser(k *kit, presets []string, state *launcherPreviewState) widget.Widget {
	t := locale.T()
	labels := append([]string{t.Get("settings-launcher-preview-no-style")}, presets...)
	d := widget.NewDropdown(k.face, k.textPx(), labels, slices.Index(presets, state.style)+1)
	d.SetAccessibleName(t.Get("settings-launcher-preview-style"))
	d.OnSelect = func(i int) {
		state.style = ""
		if i > 0 {
			state.style = presets[i-1]
		}
		state.redraw()
	}
	row := gapBox(widget.Row)
	row.AppendAligned(k.label(t.Get("settings-launcher-preview-style")), true, widget.AlignCenter)
	row.AppendAligned(d, false, widget.AlignCenter)
	return row
}
