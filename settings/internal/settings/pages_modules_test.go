package settings

import (
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
