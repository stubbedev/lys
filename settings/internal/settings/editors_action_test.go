package settings

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func everyActionChoice() []actionChoice {
	var out []actionChoice
	for _, m := range config.BuiltinModules {
		out = append(out, actionChoices(m)...)
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
	items := c.pick.pick.Items()
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
	pickChoice(c.pick.pick, c.customIndex())
	if !c.entry.Visible() {
		t.Fatal("the custom command is not shown")
	}
	tr := locale.T()
	for command, want := range map[string]string{
		"echo clicked": tr.Get("settings-custom-run-output", i18n.Str("output", "clicked")),
		"true":         tr.Get("settings-command-run-empty"),
	} {
		c.entry.SetText(command)
		if v := k.store.value("modules.media.left-click"); v != command {
			t.Fatalf("typing %q stored %v", command, v)
		}
		c.run.run()
		if got := c.run.output.Text(); got != want || !c.run.output.Visible() || c.run.problem.Visible() {
			t.Errorf("%q shows %q, shown %v, problem %q", command, got, c.run.output.Visible(), c.run.problem.Text())
		}
	}
	c.entry.SetText("exit 4")
	c.run.run()
	if c.run.output.Visible() || !failedWith(c.run.problem, tr.Get("settings-custom-run-failed"), `run "exit 4": exit status 4`) {
		t.Errorf("a failed command shows output %v, says %q over %q", c.run.output.Visible(), c.run.problem.Text(), c.run.problem.detail.Text())
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
	if items := c.pick.pick.Items(); !slices.Equal(items, want) {
		t.Fatalf("offers %q, want %q", items, want)
	}
	if c.pick.pick.Selected() != 0 || c.entry.Visible() {
		t.Fatalf("the default shows as %d, custom %v", c.pick.pick.Selected(), c.entry.Visible())
	}
	c.pick.pick.OnSelect(1)
	if v := k.store.value(path); v != "loginctl terminate-session self" {
		t.Fatalf("stored %v", v)
	}
	c.pick.pick.OnSelect(c.customIndex())
	c.entry.SetText("my-logout")
	if v := k.store.value(path); v != "my-logout" || !c.entry.Visible() {
		t.Fatalf("custom stored %v, shown %v", v, c.entry.Visible())
	}
}

func TestAPowerButtonsCommandOffersNoNoneAndNoTryingIt(t *testing.T) {
	k := testKit(t, "modules:\n  power:\n    shutdown-command: my-poweroff\n")
	c := powerCommand(t, k, "modules.power.shutdown-command")
	if slices.Contains(c.pick.pick.Items(), locale.T().Get("settings-action-none")) {
		t.Fatalf("a session command offers None: %q", c.pick.pick.Items())
	}
	if c.run != nil {
		t.Fatal("a session command offers to run it")
	}
	if c.pick.pick.Selected() != c.customIndex() || c.entry.Text() != "my-poweroff" {
		t.Fatalf("a custom command shows as %d with %q", c.pick.pick.Selected(), c.entry.Text())
	}
	if slices.Contains(c.pick.pick.Items(), locale.T().Get("settings-command-reboot")) {
		t.Fatalf("shut down offers a reboot: %q", c.pick.pick.Items())
	}
}

func monitorHost(t *testing.T) host {
	t.Helper()
	data := t.TempDir()
	h := fakeHost(t, map[string]string{"XDG_DATA_HOME": data, "XDG_DATA_DIRS": t.TempDir()})
	put(t, filepath.Join(data, "applications/missioncenter.desktop"), "[Desktop Entry]\nName=Mission Center\nExec=missioncenter\nCategories=GTK;System;Monitor;\n")
	put(t, filepath.Join(data, "applications/htop.desktop"), "[Desktop Entry]\nName=Htop\nExec=htop\nTerminal=true\nCategories=System;Monitor;ConsoleOnly;\n")
	put(t, filepath.Join(data, "applications/nemo.desktop"), "[Desktop Entry]\nName=Nemo\nExec=nemo %U\nCategories=System;FileManager;\n")
	put(t, filepath.Join(data, "applications/gnome-system-monitor.desktop"), "[Desktop Entry]\nName=System Monitor\nExec=gnome-system-monitor\nCategories=GNOME;GTK;System;Monitor;\n")
	return h
}

func TestTheSystemMonitorsAreTheWindowedMonitorApps(t *testing.T) {
	got := monitorHost(t).systemMonitors()
	if !slices.Equal(values(got), []string{"missioncenter", "gnome-system-monitor"}) {
		t.Fatalf("system monitors %+v, want Mission Center and System Monitor without htop or Nemo", got)
	}
	if got := fakeHost(t, map[string]string{"XDG_DATA_HOME": t.TempDir(), "XDG_DATA_DIRS": t.TempDir()}).systemMonitors(); len(got) != 0 {
		t.Errorf("with no apps installed the system monitors are %+v", got)
	}
}

func TestTheMemoryModuleOffersToOpenEachSystemMonitor(t *testing.T) {
	h := monitorHost(t)
	presets := hostActionChoices(h, config.ModuleRAM)
	labels := make([]string, len(presets))
	for i, p := range presets {
		labels[i] = p.label()
	}
	want := locale.T().Get("settings-action-open-monitor", i18n.Str("app", "Mission Center"))
	if len(presets) != 2 || !slices.Contains(labels, want) {
		t.Fatalf("the memory module's host presets are %q, want one per monitor with %q", labels, want)
	}
	if got := hostActionChoices(h, config.ModuleCPU); got != nil {
		t.Errorf("the CPU module got host presets %+v", got)
	}
	before := systemHost
	systemHost = h
	t.Cleanup(func() { systemHost = before })
	k := testKit(t, "")
	m, ok := fieldControl(k, k.store, pageRow(t, "ram", ramPath+".left-click")).(*mouseActions)
	if !ok {
		t.Fatal("the RAM page has no mouse actions")
	}
	if !slices.ContainsFunc(m.choices, func(c actionChoice) bool { return c.command == "missioncenter" && c.label() == want }) {
		t.Errorf("the RAM page's click presets %+v miss %q", m.choices, want)
	}
}

// Every click a bar item does by default is one of its choices, read by
// name: a default never reads as a custom command.
func TestEveryDefaultClickIsANamedChoice(t *testing.T) {
	modules := config.DefaultsModules()
	for _, m := range config.BuiltinModules {
		clicks, ok := modules.ModuleClicks(string(m))
		if !ok {
			continue
		}
		var commands []string
		for _, c := range actionChoices(m) {
			commands = append(commands, c.command)
		}
		for _, a := range []config.ClickAction{clicks.LeftClick, clicks.RightClick, clicks.MiddleClick, clicks.ScrollUp, clicks.ScrollDown} {
			if s := a.String(); s != "" && !slices.Contains(commands, s) {
				t.Errorf("%s clicks %q by default, which is none of its choices %q", m, s, commands)
			}
		}
	}
}

// Each power command's menu took its own presets' width, so Log Out's,
// whose presets read longest, started 63px left of the others. The menus
// of one card share its control column.
func TestTheSessionCommandsShareTheControlColumn(t *testing.T) {
	k := testKit(t, "")
	group := widget.NewItemGroup()
	var menus []*actionEditor
	for _, a := range powerMenuActions {
		path := "modules.power." + a + "-command"
		c := fieldControl(k, k.store, field(path, sessionCommandRow(path))).(*actionEditor)
		menus = append(menus, c)
		group.Append(k.row(a, "").withControl(c))
	}
	box := widget.NewBox(widget.Column, 0, 0)
	box.Append(group, false)
	styledLayOut(k, box, 1000)
	first := menus[0].Bounds()
	for i, m := range menus {
		if b := m.Bounds(); b.X != first.X || b.W != first.W {
			t.Errorf("%s's menu spans %d..%d, %s's %d..%d", powerMenuActions[i], b.X, b.X+b.W, powerMenuActions[0], first.X, first.X+first.W)
		}
	}
}
