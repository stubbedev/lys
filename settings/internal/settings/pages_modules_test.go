package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
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
	if !slices.Equal(opened, []string{"media"}) {
		t.Errorf("the Privacy page's links opened %q", opened)
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
