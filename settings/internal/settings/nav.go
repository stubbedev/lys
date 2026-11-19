package settings

import (
	"fmt"
	"log"
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
	if p.wmLead && !installed.wm {
		p.lead, p.wmLead = nil, false
	}
	sections := make([]sectionSpec, 0, len(p.sections))
	for _, s := range p.sections {
		rows := make([]rowSpec, 0, len(s.rows))
		for _, r := range s.rows {
			isWM := p.wm || r.wm
			switch {
			case isWM && installed.wm, !isWM && installed.shell:
				rows = append(rows, r)
			case r.wmTwin != "" && installed.wm:
				rows = append(rows, r.asWMTwin())
			}
		}
		if len(rows) > 0 {
			s.rows = rows
			sections = append(sections, s)
		}
	}
	p.sections = sections
	if p.footerOnRows && len(sections) == 0 {
		p.footer = nil
	}
	return p, len(sections) > 0 || p.lead != nil || p.footer != nil
}

func layout(cfg *config.Config, installed parts) []navSection {
	sections := []navSection{
		{key: "settings-nav-home", pages: []pageSpec{overviewPage()}},
		{key: "settings-nav-account", pages: []pageSpec{usersPage(cfg)}},
		{key: "settings-nav-connectivity", pages: []pageSpec{wifiPage(cfg), bluetoothDevicesPage(cfg)}},
		{key: "settings-nav-appearance", pages: []pageSpec{themePage(cfg), wallpaperPage(cfg), fontsPage(cfg), motionPage(cfg)}},
		{key: "settings-nav-displays", pages: []pageSpec{displaysPage()}},
		{key: "settings-nav-sound-section", pages: []pageSpec{soundPage(cfg)}},
		{key: "settings-nav-input", pages: []pageSpec{wmKeyboardPage(), wmPointerPage(), wmBindsPage()}},
		{key: "settings-nav-notifications-focus", pages: []pageSpec{notificationsPage(cfg), osdPage(cfg)}},
		{key: "settings-nav-power", pages: []pageSpec{powerBatteryPage(cfg), idlePage(cfg)}},
		{key: "settings-nav-apps", pages: []pageSpec{defaultAppsPage(cfg), loginItemsPage(cfg), launcherPage(cfg), launcherModesPage(cfg)}},
		{key: "settings-nav-privacy-security", pages: []pageSpec{privacyPage(cfg), passwordsPage(cfg), lockPage(cfg)}},
		{key: "settings-nav-time-language", pages: []pageSpec{dateTimePage(cfg), regionPage(cfg)}},
		{key: "settings-nav-desktop", pages: []pageSpec{
			barPage(cfg), dockPage(cfg),
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

func motionPage(*config.Config) pageSpec {
	return pageSpec{
		id: "motion", navKey: "settings-nav-motion", icon: "ld-zap-symbolic",
		header: "settings-page-motion",
		sections: []sectionSpec{
			{title: "settings-section-general", rows: []rowSpec{
				twinToggle("animations.enabled", "animations.enabled"), twinToggle("animations.reduced-motion", "animations.reduced_motion"),
				animated(field("animations.transition")), animated(field("animations.indicators")),
			}},
			{title: "settings-section-speed", rows: []rowSpec{animated(speedRow())}},
			{title: "settings-section-durations", advanced: true, rows: animatedRows(
				field("animations.duration", animationDuration), field("animations.ui-duration", animationDuration), field("animations.interaction-duration", animationDuration),
			)},
			{title: "settings-section-direction", advanced: true, rows: animatedRows(
				field("animations.enter"), field("animations.exit"),
				field("animations.enter-duration", animationDuration), field("animations.exit-duration", animationDuration),
			)},
			{title: "settings-animations-power", advanced: true, rows: animatedRows(surfaceAnimationRows("animations.power")...)},
			{title: "settings-animations-dialog", advanced: true, rows: animatedRows(surfaceAnimationRows("animations.dialog")...)},
			{title: "settings-animations-file-chooser", advanced: true, rows: animatedRows(surfaceAnimationRows("animations.file-chooser")...)},
			{title: "settings-animations-print", advanced: true, rows: animatedRows(surfaceAnimationRows("animations.print")...)},
			{title: "wm-section-overview", rows: []rowSpec{wmField("overview.blur"), wmField("overview.dim", fraction), wmField("switcher.open_delay"), wmField("showdesktop.peek")}},
			{title: "wm-section-drag", rows: wmFields("drag_preview.enabled", "drag_preview.opacity", "drag_preview.blur", "drag_preview.snap_distance",
				"drag_preview.snap_windows", "drag_preview.drag_switch.width", "drag_preview.drag_switch.frames")},
			{title: "wm-section-hot-corners", rows: append([]rowSpec{hotCornersRow()}, wmFields("hot_corners.pressure", "hot_corners.fullscreen")...)},
		},
	}
}

// animated is a lys-shell animation row, in use only while animations
// are on.
func animated(r rowSpec) rowSpec {
	enabledBy("animations.enabled")(&r)
	return r
}

func animatedRows(rows ...rowSpec) []rowSpec {
	out := make([]rowSpec, len(rows))
	for i, r := range rows {
		out[i] = animated(r)
	}
	return out
}

func (r rowSpec) asWMTwin() rowSpec {
	r.key, r.path, r.wm = r.labelKey(), r.wmTwin, true
	return r
}

// twinToggle is a switch for a lys-shell key and the lys-wm key that means
// the same, writing whichever of the two is installed.
func twinToggle(shellPath, wmPath string) rowSpec {
	return field(shellPath, func(r *rowSpec) {
		r.wmTwin = wmPath
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			var slots []slot
			if k.parts.shell {
				slots = append(slots, s)
			}
			if k.parts.wm {
				w, err := k.wmBackend()
				if err != nil {
					log.Printf("settings: %s: %v", wmPath, err)
				} else {
					f, ok := w.fieldOf(wmPath)
					if !ok {
						log.Panicf("settings: lys-wm has no %s", wmPath)
					}
					slots = append(slots, wmChoiceSlot(f, w.slot(wmPath)))
				}
			}
			return newToggle(k, joinSlots(slots))
		}
	})
}

// joinSlots reads the first slot and writes every one, the first refusal
// stopping the rest.
func joinSlots(slots []slot) slot {
	return slot{
		get: func() any {
			if len(slots) == 0 {
				return nil
			}
			return slots[0].get()
		},
		set: func(v any) error {
			for _, s := range slots {
				if err := s.set(v); err != nil {
					return err
				}
			}
			return nil
		},
		unset: func() {
			for _, s := range slots {
				s.unset()
			}
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
