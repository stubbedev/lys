package settings

import (
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// actionChoice is ActionChoice: a preset command and its label's message.
type actionChoice struct{ key, command string }

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
// custom command whose entry slides in and writes as it is typed.
type actionEditor struct {
	*widget.Box
	k       *kit
	slot    slot
	menu    commandMenu
	choices []actionChoice
	drop    *widget.Dropdown
	entry   *widget.Entry
	run     *commandRun
	reveal  *widget.Revealer
	custom  bool
	syncing bool
}

func newActionEditor(k *kit, s slot, choices []actionChoice) *actionEditor {
	return newCommandMenu(k, s, choices, clickMenu)
}

func newCommandMenu(k *kit, s slot, choices []actionChoice, menu commandMenu) *actionEditor {
	c := &actionEditor{Box: gapBox(widget.Column), k: k, slot: s, menu: menu, choices: choices}
	t := locale.T()
	labels := make([]string, 0, len(choices)+2)
	for _, ch := range choices {
		labels = append(labels, t.Get(ch.key))
	}
	if menu == clickMenu {
		labels = append(labels, t.Get("settings-action-none"))
	}
	labels = append(labels, t.Get("settings-action-custom"))
	c.drop = widget.NewDropdown(k.face, k.textPx(), labels, 0)
	c.drop.OnSelect = c.selected
	c.entry = k.entry()
	c.entry.SetPlaceholder(t.Get("settings-action-placeholder"))
	c.entry.OnChanged = func(text string) {
		if c.custom && !c.syncing {
			_ = c.slot.set(text)
		}
	}
	custom := gapBox(widget.Column)
	custom.Append(c.entry, false)
	if menu == clickMenu {
		c.run = newCommandRun(k, s, "settings-command-run-empty")
		custom.Append(c.run, false)
	}
	c.reveal = widget.NewRevealer(custom)
	c.reveal.SetTransition(widget.RevealSlideDown)
	c.reveal.SetCollapse(true)
	c.Append(c.drop, false)
	c.Append(c.reveal, false)
	command := c.command()
	c.custom = command != "" && c.choiceIndex(command) < 0
	c.refresh()
	return c
}

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

func (c *actionEditor) customIndex() int {
	if c.menu != clickMenu {
		return len(c.choices)
	}
	return len(c.choices) + 1
}

func (c *actionEditor) choiceIndex(command string) int {
	return slices.IndexFunc(c.choices, func(ch actionChoice) bool { return ch.command == command })
}

// selected is ActionMsg::Selected: a preset or None writes it and hides
// the entry; Custom shows the entry and writes what it holds.
func (c *actionEditor) selected(i int) {
	if c.syncing {
		return
	}
	switch {
	case i < len(c.choices):
		c.custom = false
		c.reveal.SetRevealed(false)
		_ = c.slot.set(c.choices[i].command)
	case i == c.noneIndex():
		c.custom = false
		c.reveal.SetRevealed(false)
		_ = c.slot.set("")
	default:
		c.custom = true
		c.reveal.SetRevealed(true)
		if text := c.entry.Text(); text != "" {
			_ = c.slot.set(text)
		}
	}
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
	c.syncing = true
	c.drop.SetSelected(i)
	reveal := i == c.customIndex()
	c.reveal.SetRevealed(reveal)
	if reveal && c.entry.Text() != command {
		c.entry.SetText(command)
	}
	c.syncing = false
}

// workspaceChoices is workspace_choices: the focus actions the
// workspace modules add (WorkspaceClickAction).
func workspaceChoices() []actionChoice {
	return []actionChoice{
		{"settings-action-focus-workspace", config.WorkspaceClickAction{Kind: config.WorkspaceClickFocusThis}.String()},
		{"settings-action-focus-next", config.WorkspaceClickAction{Kind: config.WorkspaceClickFocusNext}.String()},
		{"settings-action-focus-previous", config.WorkspaceClickAction{Kind: config.WorkspaceClickFocusPrevious}.String()},
		{"settings-action-focus-last", config.WorkspaceClickAction{Kind: config.WorkspaceClickFocusLast}.String()},
	}
}

// actionChoices is choices_for: each module's preset click actions.
func actionChoices(module string) []actionChoice {
	shell := func(key, command string) actionChoice { return actionChoice{key, command} }
	dropdown := func(key, id string) actionChoice { return actionChoice{key, "dropdown:" + id} }
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
	case "battery":
		return []actionChoice{dropdown("settings-action-open-battery", "battery")}
	case "bitwarden":
		return []actionChoice{dropdown("settings-action-open-vault", "bitwarden")}
	case "bluetooth":
		return []actionChoice{dropdown("settings-action-open-bluetooth", "bluetooth")}
	case "brightness":
		return []actionChoice{
			{"settings-action-brightness-up", "brightness:5"},
			{"settings-action-brightness-down", "brightness:-5"},
			{"settings-action-brightness-blackout", "brightness:toggle"},
			dropdown("settings-action-open-brightness", "brightness"),
		}
	case "network":
		return []actionChoice{dropdown("settings-action-open-network", "network")}
	case "media":
		return []actionChoice{
			shell("settings-action-media-play-pause", "lys-shell media play-pause"), shell("settings-action-media-next", "lys-shell media next"),
			shell("settings-action-media-previous", "lys-shell media previous"), dropdown("settings-action-open-media", "media"),
		}
	case "notifications", "notification":
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
	}
	return nil
}
