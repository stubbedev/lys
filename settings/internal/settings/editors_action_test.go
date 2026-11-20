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

func powerCommand(t *testing.T, k *kit, path string) *actionEditor {
	t.Helper()
	c, ok := fieldControl(k, k.store, pageRow(t, "power", path)).(*actionEditor)
	if !ok {
		t.Fatalf("%s is not a command menu", path)
	}
	return c
}

func TestAPowerButtonsCommandIsPickedFromTheSessionActions(t *testing.T) {
	k := testKit(t, "")
	const path = "modules.power.logout-command"
	c := powerCommand(t, k, path)
	tr := locale.T()
	want := []string{tr.Get("settings-command-default"), tr.Get("settings-command-end-session"), tr.Get("settings-command-end-user"), tr.Get("settings-action-custom")}
	if items := c.drop.Items(); !slices.Equal(items, want) {
		t.Fatalf("offers %q, want %q", items, want)
	}
	if c.drop.Selected() != 0 || c.reveal.Revealed() {
		t.Fatalf("the default shows as %d, custom %v", c.drop.Selected(), c.reveal.Revealed())
	}
	c.drop.OnSelect(1)
	if v := k.store.value(path); v != "loginctl terminate-session self" {
		t.Fatalf("stored %v", v)
	}
	c.drop.OnSelect(c.customIndex())
	c.entry.SetText("my-logout")
	if v := k.store.value(path); v != "my-logout" || !c.reveal.Revealed() {
		t.Fatalf("custom stored %v, shown %v", v, c.reveal.Revealed())
	}
}

func TestAPowerButtonsCommandOffersNoNoneAndNoTryingIt(t *testing.T) {
	k := testKit(t, "modules:\n  power:\n    shutdown-command: my-poweroff\n")
	c := powerCommand(t, k, "modules.power.shutdown-command")
	if slices.Contains(c.drop.Items(), locale.T().Get("settings-action-none")) {
		t.Fatalf("a session command offers None: %q", c.drop.Items())
	}
	if c.run != nil {
		t.Fatal("a session command offers to run it")
	}
	if c.drop.Selected() != c.customIndex() || c.entry.Text() != "my-poweroff" {
		t.Fatalf("a custom command shows as %d with %q", c.drop.Selected(), c.entry.Text())
	}
	if slices.Contains(c.drop.Items(), locale.T().Get("settings-command-reboot")) {
		t.Fatalf("shut down offers a reboot: %q", c.drop.Items())
	}
}
