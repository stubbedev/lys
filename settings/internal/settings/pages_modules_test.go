package settings

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/fakeclock"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestNoSidebarEntryReadsLikeAnotherOrAnotherGroup(t *testing.T) {
	tr := locale.T()
	sections := layout(config.Defaults(), bothParts)
	groups := map[string]string{}
	for _, s := range sections {
		groups[tr.Get(s.key)] = s.key
	}
	pages := map[string]string{}
	for _, s := range sections {
		for _, p := range s.pages {
			text := tr.Get(p.navKey)
			if other, ok := pages[text]; ok {
				t.Errorf("pages %s and %s both read %q", p.id, other, text)
			}
			pages[text] = p.id
			if g, ok := groups[text]; ok && g != s.key {
				t.Errorf("page %s reads %q like the group %s it is not in", p.id, text, g)
			}
		}
	}
}

func TestThePowerMenuListsItsButtonsWithTheirCommandsUnderAdvanced(t *testing.T) {
	p, ok := pageByID(layout(config.Defaults(), bothParts), "power")
	if !ok {
		t.Fatal("no power menu page")
	}
	var buttons, commands *sectionSpec
	for i, s := range p.sections {
		switch s.title {
		case "settings-section-power-menu":
			buttons = &p.sections[i]
		case "settings-section-power-menu-commands":
			commands = &p.sections[i]
		}
	}
	if buttons == nil || commands == nil {
		t.Fatal("the power menu page lacks its buttons or its commands")
	}
	if buttons.advanced || !commands.advanced {
		t.Errorf("buttons advanced %v, commands advanced %v; want only the commands folded away", buttons.advanced, commands.advanced)
	}
	if len(buttons.rows) != len(powerMenuActions) || len(commands.rows) != len(powerMenuActions) {
		t.Fatalf("%d buttons and %d commands, want %d of each", len(buttons.rows), len(commands.rows), len(powerMenuActions))
	}
	for i, r := range commands.rows {
		if r.enabledWhen.path != buttons.rows[i].path {
			t.Errorf("%s follows %q, want its button %s", r.path, r.enabledWhen.path, buttons.rows[i].path)
		}
		if _, ok := config.Field(r.path); !ok {
			t.Errorf("no config field %s", r.path)
		}
	}
}

func TestTheScreenshotPageGroupsWhatHappensAfterACapture(t *testing.T) {
	p, ok := pageByID(layout(config.Defaults(), bothParts), "screenshot")
	if !ok {
		t.Fatal("no screenshot page")
	}
	want := map[string][]string{
		"settings-section-screenshot-save": {"modules.screenshot.output-directory", "modules.screenshot.filename-format"},
		"settings-section-screenshot-after": {
			"modules.screenshot.copy-to-clipboard", "modules.screenshot.notify",
			"modules.screenshot.thumbnail-ms", "modules.screenshot.thumbnail-position",
		},
	}
	for _, s := range p.sections {
		paths, ok := want[s.title]
		if !ok {
			continue
		}
		got := make([]string, len(s.rows))
		for i, r := range s.rows {
			got[i] = r.path
		}
		if !slices.Equal(got, paths) {
			t.Errorf("%s holds %v, want %v", s.title, got, paths)
		}
		delete(want, s.title)
	}
	for title := range want {
		t.Errorf("the screenshot page has no section %s", title)
	}
}

func TestTheSeparatorsThicknessSlidesOverOneToFourPixels(t *testing.T) {
	k := testKit(t, "modules:\n  separator:\n    size: 6\n")
	c, ok := fieldControl(k, k.store, pageRow(t, "separator", "modules.separator.size")).(*softRange)
	if !ok {
		t.Fatal("the separator's thickness is not a pixel slider")
	}
	if c.number.Value() != 6 {
		t.Errorf("a 6 px line reads %v in the number", c.number.Value())
	}
	for v, want := range map[float64]float64{0: 1, 3: 3, 9: 4} {
		c.Knob.SetValue(v)
		if got := c.Knob.Value(); got != want {
			t.Errorf("sliding to %v rests at %v, want %v", v, got, want)
		}
	}
}

func TestDeviceLikeModulePagesLinkToTheirDevicePages(t *testing.T) {
	k := testKit(t, "")
	k.nav = layout(k.store.svc.Config(), bothParts)
	for module, want := range map[string][]string{
		"audio": {soundID, barPageID}, "battery": {powerPageID, barPageID}, "bluetooth": {bluetoothPageID, barPageID}, "clock": {dateTimeID, barPageID},
		"idle-inhibit": {"idle"}, "keybind-mode": {"wm-binds"}, "keyboard-input": {"wm-keyboard"},
		"brightness": {displaysID, barPageID}, "nightlight": {displaysID}, "media": {barPageID},
	} {
		p, ok := pageByID(k.nav, module)
		if !ok {
			t.Fatalf("no %s page", module)
		}
		if opened := relatedOpens(k, p.relatedGroups()); !slices.Equal(opened, want) {
			t.Errorf("the %s page's links opened %q, want %q", module, opened, want)
		}
	}
}

func TestAModuleLinkToAPageNotInstalledIsLeftOut(t *testing.T) {
	k := testKit(t, "")
	k.nav = layout(config.Defaults(), parts{shell: true})
	p, ok := pageByID(layout(config.Defaults(), bothParts), "keyboard-input")
	if !ok {
		t.Fatal("no keyboard-input page")
	}
	if _, ok := relatedSection(k, p.relatedGroups()); ok {
		t.Error("without lys-wm the Keyboard Input page still has related links")
	}
}

func TestSearchFindsKeepAwakeByWhatItDoes(t *testing.T) {
	index := settingIndex(layout(config.Defaults(), bothParts))
	hits := searchSettings(index, "keep awake duration")
	if !slices.ContainsFunc(hits, func(e settingEntry) bool { return e.path == "modules.idle-inhibit.startup-duration" }) {
		t.Errorf("searching keep awake does not find its duration: %v", hits)
	}
	if got := searchSettings(index, "idle inhibit"); slices.ContainsFunc(got, func(e settingEntry) bool { return e.page == "idle-inhibit" }) {
		t.Errorf("the jargon still names the page: %v", got)
	}
}

func TestMediaShowsThePlayerIconsOnlyForTheMappedIconType(t *testing.T) {
	k := testKit(t, "modules:\n  media:\n    icon-type: default\n")
	p := layoutPage(t, k, "media")
	if _, ok := rowOf(t, p, "modules.media.icon-type").ctl.(*choiceCards); !ok {
		t.Fatal("the media icon type is not chosen by picture")
	}
	if _, ok := rowOf(t, p, "modules.media.icon-name").ctl.(*cellGrid); !ok {
		t.Fatal("the media icon and the spinning disc are not one grid of state icons")
	}
	mapped := rowOf(t, p, "modules.media.player-icons")
	if mapped.Visible() {
		t.Fatal("a generic icon shows the player map")
	}
	setRow(t, p, "modules.media.icon-type", string(config.MediaIconTypeApplicationMapped))
	if !mapped.Visible() {
		t.Error("a mapped icon hides the player map")
	}
}

func TestThePrivacyPageLeadsToTheOnlineLyrics(t *testing.T) {
	if err := checkSetting(bothParts, "media", "modules.media.lyrics"); err != nil {
		t.Fatal(err)
	}
	serveGrants(t, &fakeGrants{})
	k := testKit(t, "")
	k.nav = layout(k.store.svc.Config(), bothParts)
	var opened []string
	k.openPage = func(id string) { opened = append(opened, id) }
	spec, _ := pageByID(k.nav, "privacy")
	page := buildPage(k, spec)
	t.Cleanup(page.close)
	for _, row := range treetest.All[*pageLinkRow](page) {
		row.ClickAt(widget.Point{})
	}
	if !slices.Contains(opened, "media") {
		t.Errorf("the Privacy page's links opened %q, not the Media page", opened)
	}
}

func sectionOf(t *testing.T, page, path string) string {
	t.Helper()
	p, ok := pageByID(layout(config.Defaults(), bothParts), page)
	if !ok {
		t.Fatalf("no %s page", page)
	}
	for _, s := range p.sections {
		for _, r := range s.flatRows() {
			if r.path == path {
				return s.title
			}
		}
	}
	t.Fatalf("no %s row on %s", path, page)
	return ""
}

func TestBitwardenSeparatesTheAccountItsSecurityAndItsIntegrations(t *testing.T) {
	for path, want := range map[string]string{
		"modules.bitwarden.server":            "settings-section-bitwarden-account",
		"modules.bitwarden.email":             "settings-section-bitwarden-account",
		"modules.bitwarden.lock-with-session": "settings-section-bitwarden-security",
		"modules.bitwarden.clipboard-clear":   "settings-section-bitwarden-security",
		"modules.bitwarden.network-folder":    "settings-section-bitwarden-integrations",
		"modules.bitwarden.portal-folder":     "settings-section-bitwarden-integrations",
		"modules.bitwarden.icon-locked":       "settings-section-icons",
	} {
		if got := sectionOf(t, "bitwarden", path); got != want {
			t.Errorf("%s is under %s, want %s", path, got, want)
		}
	}
	if got := sectionOf(t, "bitwarden", "modules.bitwarden.format"); got == "settings-section-bitwarden-account" {
		t.Error("the bar label sits with the account")
	}
}

func TestTheNightLightScheduleRowsShowOnlyWithTheSchedule(t *testing.T) {
	const m = "modules.nightlight"
	k := testKit(t, "modules:\n  nightlight:\n    auto-schedule: false\n")
	p := layoutPage(t, k, displaysID)
	shown := func(want map[string]bool) {
		t.Helper()
		for path, visible := range want {
			if got := rowOf(t, p, m+"."+path).Visible(); got != visible {
				t.Errorf("%s shows %v, want %v", path, got, visible)
			}
		}
	}
	shown(map[string]bool{"schedule": false, "custom-hours": false, "transition-duration": false, "location": false})
	setRow(t, p, m+".auto-schedule", true)
	shown(map[string]bool{"schedule": true, "custom-hours": false, "transition-duration": true, "location": true})
	setRow(t, p, m+".schedule", string(config.NightlightScheduleCustom))
	shown(map[string]bool{"schedule": true, "custom-hours": true, "transition-duration": true, "location": false})
	setRow(t, p, m+".auto-schedule", false)
	shown(map[string]bool{"schedule": false, "custom-hours": false, "transition-duration": false, "location": false})
}

func TestRevealingARowHiddenThroughAChainLandsOnTheRowInUse(t *testing.T) {
	const m = "modules.nightlight"
	k := testKit(t, "modules:\n  nightlight:\n    auto-schedule: false\n    schedule: custom\n")
	p := layoutPage(t, k, displaysID)
	hours := rowOf(t, p, m+".custom-hours")
	if got := p.inUse(hours); got.path != m+".auto-schedule" {
		t.Fatalf("revealing the custom hours with the schedule off lands on %s", got.path)
	}
	setRow(t, p, m+".auto-schedule", true)
	if got := p.inUse(hours); got != hours {
		t.Errorf("revealing the shown custom hours lands on %s", got.path)
	}
}

func TestTheCustomHoursArePickedOnTheClockAndNeverMeet(t *testing.T) {
	const path = "modules.nightlight.custom-hours"
	k := testKit(t, "modules:\n  clock:\n    format: \"%H:%M\"\n")
	c := fieldControl(k, k.store, pageRow(t, displaysID, path)).(*clockRange)
	if c.from.hours[c.from.hour.Selected()] != 21 || c.to.hours[c.to.hour.Selected()] != 7 {
		t.Fatalf("the default hours read from %d to %d", c.from.hour.Selected(), c.to.hour.Selected())
	}
	c.from.hour.SetSelected(slices.Index(c.from.hours, 22))
	c.to.minute.SetSelected(slices.Index(c.to.minutes, 30))
	got, _ := k.store.value(path).(map[string]any)
	if got["from"] != "22:00" || got["to"] != "07:30" {
		t.Fatalf("picking 22:00 to 07:30 stored %v", got)
	}
	if k.store.svc.Config().Nightlight.CustomHours.Hours().From().String() != "22:00" {
		t.Errorf("the shell's config reads %s", k.store.svc.Config().Nightlight.CustomHours.Hours().From())
	}
	c.refresh()
	c.to.hour.SetSelected(slices.Index(c.to.hours, 22))
	if slices.Contains(c.to.minutes, 0) {
		t.Errorf("at 22 the end offers minute 0, the start's 22:00: %v", c.to.minutes)
	}
	got, _ = k.store.value(path).(map[string]any)
	if got["from"] == got["to"] {
		t.Errorf("the hours were stored meeting at %v", got["from"])
	}
}

func TestTheNightLightStrengthSlidesInKelvin(t *testing.T) {
	k := testKit(t, "")
	c, ok := fieldControl(k, k.store, pageRow(t, displaysID, "modules.nightlight.temperature")).(*kelvinEditor)
	if !ok {
		t.Fatal("the temperature is not a kelvin slider")
	}
	c.slider.OnCommit(3400.4)
	if v := k.store.value("modules.nightlight.temperature"); v != int64(3400) {
		t.Errorf("sliding to 3400 K stored %v", v)
	}
	if got := sectionOf(t, displaysID, "modules.nightlight.gamma"); got != "settings-section-nightlight-display" {
		t.Errorf("the gamma is under %s", got)
	}
}

func TestModulePagesWithNothingElsewhereHaveNoLinks(t *testing.T) {
	for _, id := range []string{"cava", "custom", "separator"} {
		p, ok := pageByID(layout(config.Defaults(), bothParts), id)
		if !ok {
			t.Fatalf("no %s page", id)
		}
		if len(relatedLinks(p)) > 0 {
			t.Errorf("the %s page has related links", id)
		}
	}
}

func TestASlidersTrailingCommitWaitsForTheKitsClock(t *testing.T) {
	k := testKit(t, "modules:\n  separator:\n    size: 2\n")
	clock := fakeclock.New()
	k.clock = clock
	c, ok := fieldControl(k, k.store, pageRow(t, "separator", "modules.separator.size")).(*softRange)
	if !ok {
		t.Fatal("the separator's thickness is not a pixel slider")
	}
	c.Knob.SetValue(3)
	c.Knob.SetValue(4)
	if got := fmt.Sprint(k.store.value("modules.separator.size")); got != "3" {
		t.Fatalf("before the throttle passed the size is %v, want the first move alone", got)
	}
	clock.Advance(time.Second)
	if got := fmt.Sprint(k.store.value("modules.separator.size")); got != "4" {
		t.Errorf("after the throttle passed the size is %v, want the last move", got)
	}
}

func TestTheResourcePagesAndTheDashboardLinkToTheirOneThresholdSet(t *testing.T) {
	k := testKit(t, "")
	k.nav = layout(k.store.svc.Config(), bothParts)
	for _, module := range []string{"cpu", "ram", "storage", "battery", "dashboard"} {
		p, ok := pageByID(k.nav, module)
		if !ok {
			t.Fatalf("no %s page", module)
		}
		var opened []string
		k.openPage = func(id string) { opened = append(opened, id) }
		for _, l := range pageLinks(p) {
			if row, ok := k.pageLinkRow(l); ok {
				row.ClickAt(widget.Point{})
			}
		}
		if !slices.Contains(opened, thresholdsPageID) {
			t.Errorf("the %s page's links opened %q, not the thresholds page", module, opened)
		}
		for _, s := range p.sections {
			for _, r := range s.rows {
				if strings.Contains(r.path, "threshold") || strings.Contains(r.path, "usage-") || strings.Contains(r.path, "battery-warning") || strings.Contains(r.path, "battery-critical") {
					t.Errorf("the %s page still edits %s", module, r.path)
				}
			}
		}
	}
	p, ok := pageByID(k.nav, thresholdsPageID)
	if !ok {
		t.Fatal("no thresholds page")
	}
	var paths []string
	for _, s := range p.sections {
		for _, r := range s.rows {
			paths = append(paths, r.path)
		}
	}
	if want := []string{"thresholds.cpu", "thresholds.ram", "thresholds.storage", "thresholds.battery"}; !slices.Equal(paths, want) {
		t.Errorf("the thresholds page edits %q, want %q", paths, want)
	}
}

// pageLinks are every link a page shows, in its sections and after them.
func pageLinks(p pageSpec) []pageLink {
	links := relatedLinks(p)
	for _, s := range p.sections {
		links = append(links, s.links...)
	}
	return links
}

func TestOnlyTheResourcePagesLinkToTheThresholds(t *testing.T) {
	pages := layout(config.Defaults(), bothParts)
	for _, id := range []string{"audio", "brightness", notificationBellID, "clock"} {
		p, ok := pageByID(pages, id)
		if !ok {
			t.Fatalf("no %s page", id)
		}
		if slices.Contains(pageLinks(p), thresholdsLink) {
			t.Errorf("the %s page links to the thresholds, which it does not read", id)
		}
	}
	for _, id := range []string{"cpu", "ram", "storage", "battery"} {
		p, _ := pageByID(pages, id)
		if slices.Contains(relatedLinks(p), thresholdsLink) {
			t.Errorf("the %s page links to the thresholds among its related pages, not in its Thresholds section", id)
		}
		i := slices.IndexFunc(p.sections, func(s sectionSpec) bool { return s.title == "settings-section-thresholds" })
		if i < 0 || !slices.Equal(p.sections[i].links, []pageLink{thresholdsLink}) {
			t.Errorf("the %s page's Thresholds section does not link to the warning levels", id)
		}
	}
}

// Every page reads in Title Case, in its sidebar entry and its header,
// the way its siblings read: no word past the first starts lower case
// but the short joining words.
func TestEveryPageTitleReadsInTitleCase(t *testing.T) {
	tr := locale.T()
	joining := map[string]bool{"a": true, "an": true, "and": true, "at": true, "by": true, "for": true, "in": true, "of": true, "on": true, "or": true, "the": true, "to": true, "with": true}
	for _, s := range layout(config.Defaults(), bothParts) {
		for _, p := range s.pages {
			for _, key := range []string{p.navKey, p.header} {
				if key == "" {
					continue
				}
				words := strings.Fields(tr.Get(key))
				for _, w := range words[1:] {
					if first, _ := utf8.DecodeRuneInString(w); unicode.IsLower(first) && !joining[w] {
						t.Errorf("page %s: %s reads %q, %q in lower case", p.id, key, tr.Get(key), w)
					}
				}
			}
		}
	}
}
