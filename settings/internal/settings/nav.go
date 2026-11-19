package settings

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/identity"
	"github.com/stubbedev/lys/desktop/settingspage"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

// initialPage is the page the window opens on (INITIAL_PAGE_ID), or
// the first page while that one is not ported.
const initialPage = overviewID

// navSection is NavSectionLayout: a titled sidebar group of pages.
type navSection struct {
	key   string
	pages []pageSpec
}

// layout is the sidebar's sections in order,
// each page built from the config (a page may list what the config
// holds).
type parts struct{ shell, wm bool }

func installedParts() parts {
	_, err := exec.LookPath(identity.Name)
	return parts{shell: err == nil, wm: wmconfig.Installed()}
}

func (p pageSpec) on(installed parts) (pageSpec, bool) {
	if p.wm && !installed.wm {
		return pageSpec{}, false
	}
	sections := make([]sectionSpec, 0, len(p.sections))
	for _, s := range p.sections {
		rows := make([]rowSpec, 0, len(s.rows))
		for _, r := range s.rows {
			if (p.wm || r.wm) && installed.wm || !(p.wm || r.wm) && installed.shell {
				rows = append(rows, r)
			}
		}
		if len(rows) > 0 {
			sections = append(sections, sectionSpec{title: s.title, rows: rows})
		}
	}
	p.sections = sections
	return p, len(sections) > 0 || p.lead != nil || p.footer != nil
}

func layout(cfg *config.Config, installed parts) []navSection {
	sections := []navSection{
		{key: "settings-nav-home", pages: []pageSpec{overviewPage()}},
		{key: "settings-nav-account", pages: []pageSpec{usersPage(cfg)}},
		{key: "settings-nav-appearance", pages: []pageSpec{themePage(cfg), wallpaperPage(cfg), generalPage(cfg), animationsPage(cfg), wmMotionPage()}},
		{key: "settings-nav-displays", pages: []pageSpec{displaysPage()}},
		{key: "settings-nav-sound-section", pages: []pageSpec{soundPage(cfg)}},
		{key: "settings-nav-input", pages: []pageSpec{wmInputPage(), wmBindsPage()}},
		{key: "settings-nav-notifications-focus", pages: []pageSpec{notificationsPage(cfg), focusPage(cfg), osdPage(cfg)}},
		{key: "settings-nav-power", pages: []pageSpec{idlePage(cfg)}},
		{key: "settings-nav-apps", pages: []pageSpec{defaultAppsPage(cfg), loginItemsPage(cfg), launcherPage(cfg), launcherModesPage(cfg)}},
		{key: "settings-nav-privacy-security", pages: []pageSpec{privacyPage(cfg), passwordsPage(cfg), lockPage(cfg), greeterPage(cfg), sharePickerPage(cfg)}},
		{key: "settings-nav-time-language", pages: []pageSpec{dateTimePage(cfg), regionPage(cfg)}},
		{key: "settings-nav-desktop", pages: []pageSpec{
			barGeneralPage(cfg), barButtonPage(cfg), barDropdownPage(cfg), dropdownsPage(cfg), dockPage(cfg),
			wmWindowsPage(), wmEffectsPage(), wmTilingPage(), wmSessionPage(),
		}},
	}
	sections = append(sections, barItemSections(cfg)...)
	out := sections[:0]
	for _, s := range sections {
		pages := s.pages[:0]
		for _, p := range s.pages {
			if p, ok := p.on(installed); ok {
				pages = append(pages, p)
			}
		}
		for i := range pages {
			pages[i].section = s.key
		}
		if len(pages) > 0 {
			out = append(out, navSection{key: s.key, pages: pages})
		}
	}
	return out
}

// pageByID finds a page in the layout.
func pageByID(sections []navSection, id string) (pageSpec, bool) {
	for _, s := range sections {
		for _, p := range s.pages {
			if p.id == id {
				return p, true
			}
		}
	}
	return pageSpec{}, false
}

// firstPage is the page to open: initialPage, else the first listed.
func firstPage(sections []navSection) string {
	if _, ok := pageByID(sections, initialPage); ok {
		return initialPage
	}
	if len(sections) > 0 && len(sections[0].pages) > 0 {
		return sections[0].pages[0].id
	}
	return ""
}

// barDropdownPage is the bar dropdown page.
func barDropdownPage(*config.Config) pageSpec {
	return pageSpec{
		id: "bar-dropdown", navKey: "settings-nav-bar-dropdown", icon: "ld-panel-bottom-symbolic",
		header: "settings-page-bar-dropdown",
		sections: []sectionSpec{
			{title: "settings-section-behavior", rows: fields("bar.dropdown-shadow", "bar.dropdown-autohide", "bar.dropdown-freeze-label")},
			{title: "settings-section-appearance", rows: []rowSpec{field("bar.dropdown-opacity", percentage)}},
		},
	}
}

// animationsPage is pages/animations.
func animationsPage(*config.Config) pageSpec {
	return pageSpec{
		id: "animations", navKey: "settings-nav-animations", icon: "ld-zap-symbolic",
		header: "settings-page-animations",
		sections: []sectionSpec{
			{title: "settings-section-general", rows: append(fields("animations.enabled", "animations.reduced-motion", "animations.transition"), field("animations.duration", animationDuration))},
			{title: "settings-section-direction", rows: []rowSpec{
				field("animations.enter"), field("animations.exit"),
				field("animations.enter-duration", animationDuration), field("animations.exit-duration", animationDuration),
			}},
			{title: "settings-animations-power", rows: surfaceAnimationRows("animations.power")},
			{title: "settings-animations-dialog", rows: surfaceAnimationRows("animations.dialog")},
			{title: "settings-animations-file-chooser", rows: surfaceAnimationRows("animations.file-chooser")},
			{title: "settings-animations-print", rows: surfaceAnimationRows("animations.print")},
			{title: "settings-section-speed", rows: []rowSpec{
				field("animations.ui-duration", animationDuration), field("animations.interaction-duration", animationDuration), field("animations.indicators"),
			}},
		},
	}
}

// sharePickerPage is pages/share_picker.
func sharePickerPage(*config.Config) pageSpec {
	return pageSpec{
		id: "share-picker", navKey: "settings-nav-share-picker", icon: "ld-app-window-symbolic",
		header: "settings-page-share-picker",
		sections: []sectionSpec{
			{title: "settings-section-general", rows: fields("share-picker.default-page", "share-picker.hide-token-restore")},
			{title: "settings-section-display", rows: []rowSpec{
				field("share-picker.width", sizeBase(config.SharePickerWidthBaseRem)),
				field("share-picker.height", sizeBase(config.SharePickerHeightBaseRem)),
				field("share-picker.resize-size"),
				field("share-picker.widget-size", sizeBase(config.SharePickerWidgetBaseRem)),
			}},
			{title: "settings-section-share-picker-windows", rows: []rowSpec{
				field("share-picker.windows-spacing", sizeBase(config.SharePickerWindowsSpacingBaseRem)),
				field("share-picker.windows-min-per-row"), field("share-picker.windows-max-per-row"),
			}},
			{title: "settings-section-share-picker-outputs", rows: []rowSpec{
				field("share-picker.outputs-spacing", sizeBase(config.SharePickerOutputsSpacingBaseRem)),
				field("share-picker.outputs-show-label"), field("share-picker.outputs-respect-scaling"),
			}},
			{title: "settings-section-animation", rows: surfaceAnimationRows("animations.share-picker")},
		},
	}
}

func pages(cfg *config.Config, installed parts) []settingspage.Page {
	t := locale.T()
	sections := layout(cfg, installed)
	out := []settingspage.Page{}
	at := map[string]int{}
	for _, s := range sections {
		for _, p := range s.pages {
			at[p.id] = len(out)
			out = append(out, settingspage.Page{ID: p.id, Title: t.Get(p.navKey), Icon: p.icon})
		}
	}
	listed := map[string]bool{}
	for _, e := range settingIndex(sections) {
		if e.path == "" || listed[e.path] {
			continue
		}
		listed[e.path] = true
		page := &out[at[e.page]]
		page.Settings = append(page.Settings, settingspage.Setting{Path: e.path, Title: e.label, Keywords: e.listedKeywords()})
	}
	return out
}

// ListedPages is the page list for a launcher, without a session and without
// touching the config: which pages exist does not depend on it.
func ListedPages() []settingspage.Page { return pages(config.Defaults(), installedParts()) }

type launch struct {
	page, setting string
}

func parseLaunch(args []string) launch {
	return launch{page: flagValue(args, settingspage.PageFlag), setting: flagValue(args, settingspage.SettingFlag)}
}

func flagValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
		if v, ok := strings.CutPrefix(a, flag+"="); ok {
			return v
		}
	}
	return ""
}

func CheckSetting(page, path string) error { return checkSetting(installedParts(), page, path) }

func checkSetting(installed parts, page, path string) error {
	hit, ok := findSetting(settingIndex(layout(config.Defaults(), installed)), path)
	if !ok {
		return fmt.Errorf("no setting %q", path)
	}
	if page != "" && page != hit.page {
		return fmt.Errorf("setting %q is on page %q, not %q", path, hit.page, page)
	}
	return nil
}
