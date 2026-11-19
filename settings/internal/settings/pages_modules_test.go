package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"

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
