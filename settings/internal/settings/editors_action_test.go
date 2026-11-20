package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func everyActionChoice() []actionChoice {
	out := workspaceChoices()
	for _, p := range modulePages(config.Defaults()) {
		out = append(out, actionChoices(p.id)...)
	}
	return out
}

func TestEveryActionPresetIsAMessage(t *testing.T) {
	tr := locale.T()
	for _, ch := range everyActionChoice() {
		if !tr.Has(ch.key) {
			t.Errorf("the preset %q has no message %q", ch.command, ch.key)
		}
	}
}

func TestKeybindModeOffersToLeaveTheMode(t *testing.T) {
	commands := map[string]bool{}
	for _, ch := range actionChoices("keybind-mode") {
		commands[ch.command] = true
	}
	for _, want := range []string{"lys-wmctl dispatch submap reset", "hyprctl dispatch submap reset"} {
		if !commands[want] {
			t.Errorf("keybind-mode does not offer %q", want)
		}
	}
	if slices.ContainsFunc(actionChoices("keyboard-input"), func(ch actionChoice) bool { return strings.Contains(ch.command, "submap") }) {
		t.Error("another module offers to leave a mode")
	}
}

func TestTheActionMenuReadsItsPresetsInTheLanguage(t *testing.T) {
	k := testKit(t, "")
	c := newActionEditor(k, k.store.slot("modules.media.left-click"), actionChoices("media"))
	items := c.drop.Items()
	tr := locale.T()
	want := []string{tr.Get("settings-action-media-play-pause"), tr.Get("settings-action-none"), tr.Get("settings-action-custom")}
	for _, w := range want {
		if !slices.Contains(items, w) {
			t.Errorf("the menu %q lacks %q", items, w)
		}
	}
	for _, it := range items {
		if strings.HasPrefix(it, "settings-") {
			t.Errorf("the menu shows the message id %q", it)
		}
	}
}

func TestACustomClickCommandCanBeRunOnce(t *testing.T) {
	k := testKit(t, "")
	c := newActionEditor(k, k.store.slot("modules.media.left-click"), actionChoices("media"))
	c.selected(c.customIndex())
	if !c.reveal.Revealed() {
		t.Fatal("the custom command is not shown")
	}
	tr := locale.T()
	for command, want := range map[string]string{
		"echo clicked": tr.Get("settings-custom-run-output", i18n.Str("output", "clicked")),
		"true":         tr.Get("settings-command-run-empty"),
		"exit 4":       tr.Get("settings-custom-run-failed", i18n.Str("reason", `run "exit 4": exit status 4`)),
	} {
		c.entry.SetText(command)
		if v := k.store.value("modules.media.left-click"); v != command {
			t.Fatalf("typing %q stored %v", command, v)
		}
		c.run.run()
		if got := c.run.output.Text(); got != want {
			t.Errorf("%q shows %q, want %q", command, got, want)
		}
	}
}
