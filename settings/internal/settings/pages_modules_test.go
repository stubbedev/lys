package settings

import (
	"fmt"
	"slices"
	"testing"
	"time"

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
		"settings-section-screenshot-text": {"modules.screenshot.ocr-languages"},
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
		c.slider.Knob.SetValue(v)
		if got := c.slider.Knob.Value(); got != want {
			t.Errorf("sliding to %v rests at %v, want %v", v, got, want)
		}
	}
}

func TestDeviceLikeModulePagesLinkToTheirDevicePages(t *testing.T) {
	k := testKit(t, "")
	k.nav = layout(k.store.svc.Config(), bothParts)
	for module, want := range map[string]string{
		"audio": soundID, "battery": powerPageID, "bluetooth": bluetoothPageID, "clock": dateTimeID,
		"idle-inhibit": "idle", "keybind-mode": "wm-binds", "keyboard-input": "wm-keyboard",
	} {
		p, ok := pageByID(k.nav, module)
		if !ok {
			t.Fatalf("no %s page", module)
		}
		if p.footer == nil {
			t.Errorf("the %s page links nowhere", module)
			continue
		}
		var opened []string
		k.openPage = func(id string) { opened = append(opened, id) }
		for _, b := range treetest.All[*widget.Button](p.footer(k)) {
			b.OnClick()
		}
		if !slices.Equal(opened, []string{want}) {
			t.Errorf("the %s page's links opened %q, want %s", module, opened, want)
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
	if labels := treetest.Labels(p.footer(k)); len(labels) != 0 {
		t.Errorf("without lys-wm the Keyboard Input page links to %q", labels)
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

func TestMediaShowsOnlyTheIconRowsOfTheChosenIconType(t *testing.T) {
	k := testKit(t, "modules:\n  media:\n    icon-type: default\n")
	p := layoutPage(t, k, "media")
	if _, ok := rowOf(t, p, "modules.media.icon-type").ctl.(*choiceCards); !ok {
		t.Fatal("the media icon type is not chosen by picture")
	}
	disc, mapped, fallback := rowOf(t, p, "modules.media.spinning-disc-icon"), rowOf(t, p, "modules.media.player-icons"), rowOf(t, p, "modules.media.icon-name")
	if disc.Visible() || mapped.Visible() || !fallback.Visible() {
		t.Fatalf("a generic icon shows the disc %v, the player map %v, the icon %v", disc.Visible(), mapped.Visible(), fallback.Visible())
	}
	setRow(t, p, "modules.media.icon-type", string(config.MediaIconTypeSpinningDisc))
	if !disc.Visible() || mapped.Visible() || !fallback.Visible() {
		t.Errorf("a spinning disc shows the disc %v, the player map %v, the icon %v", disc.Visible(), mapped.Visible(), fallback.Visible())
	}
	setRow(t, p, "modules.media.icon-type", string(config.MediaIconTypeApplicationMapped))
	if disc.Visible() || !mapped.Visible() {
		t.Errorf("a mapped icon shows the disc %v, the player map %v", disc.Visible(), mapped.Visible())
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
	footer := grantsFooter(k)
	t.Cleanup(footer.(*grantsList).close)
	for _, b := range treetest.All[*widget.Button](footer) {
		b.OnClick()
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
		"modules.bitwarden.icon-locked":       "settings-section-bitwarden-look",
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
	k := testKit(t, "modules:\n  nightlight:\n    auto-schedule: false\n")
	p := layoutPage(t, k, "nightlight")
	rows := []*settingRow{rowOf(t, p, "modules.nightlight.transition-duration"), rowOf(t, p, "modules.nightlight.location")}
	for _, r := range rows {
		if r.Visible() {
			t.Errorf("%s shows without the schedule", r.path)
		}
	}
	setRow(t, p, "modules.nightlight.auto-schedule", true)
	for _, r := range rows {
		if !r.Visible() {
			t.Errorf("%s stays hidden with the schedule on", r.path)
		}
	}
}

func TestTheNightLightStrengthSlidesInKelvin(t *testing.T) {
	k := testKit(t, "")
	c, ok := fieldControl(k, k.store, pageRow(t, "nightlight", "modules.nightlight.temperature")).(*slider)
	if !ok {
		t.Fatal("the temperature is not a slider")
	}
	c.OnCommit(3400.4)
	if v := k.store.value("modules.nightlight.temperature"); v != int64(3400) {
		t.Errorf("sliding to 3400 K stored %v", v)
	}
	if got := sectionOf(t, "nightlight", "modules.nightlight.gamma"); got != "settings-section-nightlight-display" {
		t.Errorf("the gamma is under %s", got)
	}
}

func TestModulePagesWithNothingElsewhereHaveNoLinks(t *testing.T) {
	for _, id := range []string{"cava", "custom", "media"} {
		p, ok := pageByID(layout(config.Defaults(), bothParts), id)
		if !ok {
			t.Fatalf("no %s page", id)
		}
		if p.footer != nil {
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
	c.slider.Knob.SetValue(3)
	c.slider.Knob.SetValue(4)
	if got := fmt.Sprint(k.store.value("modules.separator.size")); got != "3" {
		t.Fatalf("before the throttle passed the size is %v, want the first move alone", got)
	}
	clock.Advance(time.Second)
	if got := fmt.Sprint(k.store.value("modules.separator.size")); got != "4" {
		t.Errorf("after the throttle passed the size is %v, want the last move", got)
	}
}
