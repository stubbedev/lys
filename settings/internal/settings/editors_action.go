package settings

import (
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// actionChoice is ActionChoice: a preset command and its label's message
// with its arguments.
type actionChoice struct {
	key, command string
	args         []i18n.Arg
}

func (c actionChoice) label() string { return locale.T().Get(c.key, c.args...) }

// commandMenu is what an actionEditor offers beside its presets.
type commandMenu int

const (
	// clickMenu offers None, and a custom command that can be run once.
	clickMenu commandMenu = iota
	// sessionMenu offers neither: a session command always runs
	// something, and trying one would lock or end the session.
	sessionMenu
)

// actionEditor is ActionControl over a command string (a click action,
// which ClickAction and WorkspaceClickAction both encode as one, or a
// session command): the presets, None where the menu has it, and a
// custom command whose entry follows the menu and writes as it is
// typed.
type actionEditor struct {
	*widget.Box
	k       *kit
	slot    slot
	menu    commandMenu
	choices []actionChoice
	pick    *customDropdown
	entry   *widget.Entry
	run     *commandRun
	custom  bool
	syncing bool
}

func newActionEditor(k *kit, s slot, choices []actionChoice) *actionEditor {
	return newCommandMenu(k, s, choices, clickMenu)
}

func newCommandMenu(k *kit, s slot, choices []actionChoice, menu commandMenu) *actionEditor {
	c := &actionEditor{Box: gapBox(widget.Column), k: k, slot: s, menu: menu, choices: choices}
	t := locale.T()
	labels := make([]string, 0, len(choices)+1)
	for _, ch := range choices {
		labels = append(labels, ch.label())
	}
	if menu == clickMenu {
		labels = append(labels, t.Get("settings-action-none"))
	}
	c.entry = k.entry()
	c.entry.SetPlaceholder(t.Get("settings-action-placeholder"))
	c.entry.OnChanged = func(text string) {
		if c.custom && !c.syncing {
			_ = c.slot.set(text)
		}
	}
	c.pick = newCustomDropdown(k, labels, t.Get("settings-action-custom"), c.entry)
	c.pick.OnSelect = c.selected
	c.Append(c.pick, false)
	if menu == clickMenu {
		c.run = newCommandRun(k, s, "settings-command-run-empty")
		c.Append(c.run, false)
	}
	command := c.command()
	c.custom = command != "" && c.choiceIndex(command) < 0
	c.refresh()
	return c
}

// fit stretches the menu over the card's control column, so the menus
// of one card start and end at one x whatever each shows.
func (c *actionEditor) fit() widget.ItemControlFit { return widget.ItemControlColumn }

func (c *actionEditor) command() string {
	s, _ := c.slot.get().(string)
	return s
}

func (c *actionEditor) noneIndex() int {
	if c.menu != clickMenu {
		return -1
	}
	return len(c.choices)
}

func (c *actionEditor) customIndex() int { return c.pick.customIndex() }

func (c *actionEditor) choiceIndex(command string) int {
	return slices.IndexFunc(c.choices, func(ch actionChoice) bool { return ch.command == command })
}

// selected is ActionMsg::Selected: a preset or None writes it; Custom
// writes what the entry holds.
func (c *actionEditor) selected(i int) {
	c.custom = i == c.customIndex()
	switch {
	case i < len(c.choices):
		_ = c.slot.set(c.choices[i].command)
	case i == c.noneIndex():
		_ = c.slot.set("")
	default:
		if text := c.entry.Text(); text != "" {
			_ = c.slot.set(text)
		}
	}
	c.showRun()
}

// refresh is ActionMsg::Refresh: a preset or empty command leaves
// custom mode (unless custom was just chosen with nothing typed yet).
func (c *actionEditor) refresh() {
	command := c.command()
	if (c.choiceIndex(command) >= 0 || command == "") && (!c.custom || command != "") {
		c.custom = false
	}
	i := c.choiceIndex(command)
	switch {
	case i >= 0:
	case command == "" && !c.custom && c.menu == clickMenu:
		i = c.noneIndex()
	default:
		i = c.customIndex()
	}
	c.custom = i == c.customIndex()
	c.pick.show(i)
	if c.custom && c.entry.Text() != command {
		c.syncing = true
		c.entry.SetText(command)
		c.syncing = false
	}
	c.showRun()
}

// showRun shows the custom command's run while custom is chosen.
func (c *actionEditor) showRun() {
	if c.run != nil {
		c.run.SetVisible(c.custom)
	}
}

// workspaceChoices is workspace_choices: the focus actions the
// workspace modules add (WorkspaceClickAction).
func workspaceChoices() []actionChoice {
	return []actionChoice{
		{key: "settings-action-focus-workspace", command: config.WorkspaceClickAction{Kind: config.WorkspaceClickFocusThis}.String()},
		{key: "settings-action-focus-next", command: config.WorkspaceClickAction{Kind: config.WorkspaceClickFocusNext}.String()},
		{key: "settings-action-focus-previous", command: config.WorkspaceClickAction{Kind: config.WorkspaceClickFocusPrevious}.String()},
		{key: "settings-action-focus-last", command: config.WorkspaceClickAction{Kind: config.WorkspaceClickFocusLast}.String()},
	}
}

// actionChoices is choices_for: each module's preset click actions.
func actionChoices(module config.BarModule) []actionChoice {
	shell := func(key, command string) actionChoice { return actionChoice{key: key, command: command} }
	dropdown := func(key, id string) actionChoice { return actionChoice{key: key, command: "dropdown:" + id} }
	switch module {
	case "recorder":
		return []actionChoice{
			shell("settings-action-recording-toggle", "lys-shell recorder toggle"), shell("settings-action-recording-start", "lys-shell recorder start"),
			shell("settings-action-recording-stop", "lys-shell recorder stop"), shell("settings-action-recording-pause", "lys-shell recorder pause"),
			shell("settings-action-recording-resume", "lys-shell recorder resume"), dropdown("settings-action-open-recorder", "recorder"),
		}
	case "screenshot":
		out := make([]actionChoice, len(screenshotCaptures))
		for i, c := range screenshotCaptures {
			out[i] = shell(c.key, c.command())
		}
		return out
	case "background-apps":
		return []actionChoice{dropdown("settings-action-open-background-apps", "background-apps")}
	case "battery":
		return []actionChoice{dropdown("settings-action-open-battery", "battery")}
	case "bitwarden":
		return []actionChoice{dropdown("settings-action-open-vault", "bitwarden")}
	case "bluetooth":
		return []actionChoice{dropdown("settings-action-open-bluetooth", "bluetooth")}
	case "brightness":
		return []actionChoice{
			{key: "settings-action-brightness-up", command: "brightness:5"},
			{key: "settings-action-brightness-down", command: "brightness:-5"},
			{key: "settings-action-brightness-blackout", command: "brightness:toggle"},
			dropdown("settings-action-open-brightness", "brightness"),
		}
	case "network":
		return []actionChoice{dropdown("settings-action-open-network", "network")}
	case "media":
		return []actionChoice{
			shell("settings-action-media-play-pause", "lys-shell media play-pause"), shell("settings-action-media-next", "lys-shell media next"),
			shell("settings-action-media-previous", "lys-shell media previous"), dropdown("settings-action-open-media", "media"),
		}
	case "notifications":
		return []actionChoice{shell("settings-action-dnd-toggle", "lys-shell notify dnd"), dropdown("settings-action-open-notifications", "notification")}
	case "mail":
		return []actionChoice{dropdown("settings-action-open-mail", "mail")}
	case "weather":
		return []actionChoice{dropdown("settings-action-open-weather", "weather")}
	case "treeman":
		return []actionChoice{dropdown("settings-action-open-treeman", "treeman")}
	case "dashboard", "cpu", "ram", "storage", "netstat":
		return []actionChoice{dropdown("settings-action-open-dashboard", "dashboard")}
	case "clock":
		return []actionChoice{dropdown("settings-action-open-calendar", "calendar"), dropdown("settings-action-open-weather", "weather")}
	case "world-clock":
		return []actionChoice{dropdown("settings-action-open-calendar", "calendar")}
	case "nightlight":
		return []actionChoice{shell("settings-action-nightlight-toggle", ":toggle")}
	case "power-profiles":
		return []actionChoice{shell("settings-action-profile-cycle", ":cycle")}
	case "power":
		return []actionChoice{shell("settings-action-open-power-menu", ":menu")}
	case "keybind-mode":
		return []actionChoice{
			shell("settings-action-leave-mode-lys-wm", "lys-wmctl dispatch submap reset"),
			shell("settings-action-leave-mode-hyprland", "hyprctl dispatch submap reset"),
		}
	case "keyboard-input":
		return []actionChoice{shell("settings-action-layout-next", config.KeyboardNextLayoutAction)}
	case "idle-inhibit":
		return []actionChoice{
			shell("settings-action-keep-awake-toggle", "lys-shell idle toggle"),
			shell("settings-action-keep-awake-toggle-indefinite", "lys-shell idle toggle --indefinite"),
		}
	case "audio":
		return []actionChoice{
			shell("settings-action-speaker-mute", "lys-shell audio output-mute"), shell("settings-action-speaker-up", "lys-shell audio output-volume +5"),
			shell("settings-action-speaker-down", "lys-shell audio output-volume -5"), shell("settings-action-mic-mute", "lys-shell audio input-mute"),
			shell("settings-action-mic-up", "lys-shell audio input-volume +5"), shell("settings-action-mic-down", "lys-shell audio input-volume -5"),
			dropdown("settings-action-open-audio", "audio"),
		}
	case "cava":
		return []actionChoice{dropdown("settings-action-open-audio", "audio")}
	case "workspaces":
		return workspaceChoices()
	}
	return nil
}

// hostActionChoices are the presets a module's clicks get from what is
// installed, listed as its row opens: an installed system monitor for
// the memory module.
func hostActionChoices(h host, module config.BarModule) []actionChoice {
	if module != config.ModuleRAM {
		return nil
	}
	monitors := h.systemMonitors()
	out := make([]actionChoice, len(monitors))
	for i, m := range monitors {
		out[i] = actionChoice{key: "settings-action-open-monitor", command: m.value, args: []i18n.Arg{i18n.Str("app", m.label)}}
	}
	return out
}
