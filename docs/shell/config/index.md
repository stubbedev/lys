---
title: Config reference
outline: [2]
---

<div v-pre>

# Config reference

Every config file lives at `~/.config/lys-shell/config.yaml`. Each page below covers one section. Every field has a default; start with an empty file and add only what you want to change.

::: tip
Editor intellisense via JSON Schema. Install the [YAML extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml) for VSCode or `yaml-language-server` for Neovim, Helix, or Zed. The schema is written to `~/.config/lys-shell/schema.json` on startup, and the config file lys-shell creates starts with the `# yaml-language-server: $schema=./schema.json` modeline that points the server at it.
:::

## Top-level sections

| Section | What it controls |
|---|---|
| [`animations`](/shell/config/animations) | Enter/exit and change animations for transient surfaces. |
| [`bar`](/shell/config/bar) | Bar chrome: per-monitor layout, spacing, colors, and button styling. |
| [`clipboard`](/shell/config/clipboard) | Clipboard history settings. |
| [`colorpicker`](/shell/config/colorpicker) | Color picker settings. |
| [`dock`](/shell/config/dock) | A dock of pinned apps and open windows, on its own surface beside the bar. |
| [`dropdowns`](/shell/config/dropdowns) | Per-dropdown foldout panel sizing. |
| [`general`](/shell/config/general) | Shell-wide settings that don't belong to any specific module. |
| [`idle`](/shell/config/idle) | Built-in idle policy: keyboard backlight off, dim, lock, screens off, suspend, lock before sleep. |
| [`launcher`](/shell/config/launcher) | Application launcher / dmenu (rofi replacement). |
| [`login`](/shell/config/login) | Login screen: the greeter and the lock screen, one screen in two modes. |
| [`mail-account`](/shell/config/mail-account) | One mail account in the `modules.mail` per-account breakdown. |
| [`osd`](/shell/config/osd) | On-screen display overlay for transient events like volume and brightness. |
| [`passwords`](/shell/config/passwords) | The Passwords page over the keyring. |
| [`portal`](/shell/config/portal) | Desktop-portal policy for sandboxed apps. |
| [`share-picker`](/shell/config/share-picker) | Screen-share picker shown by xdg-desktop-portal when an app requests a window, output, or region to capture. |
| [`styling`](/shell/config/styling) | Theme, palette, and rounding tokens applied shell-wide. Changes recompile the stylesheet. |
| [`thresholds`](/shell/config/thresholds) | Warning levels of CPU, RAM, disk and battery, shared by their bar modules and the dashboard. |
| [`toast-preset`](/shell/config/toast-preset) | A reusable toast preset, triggerable by id with `lys-shell toast --preset <id>`. |
| [`wallpaper`](/shell/config/wallpaper) | Wallpaper rendering, cycling, and per-monitor overrides. |

## Bar modules

Modules appear inside the `bar.layout` list. Each row links to the full reference.

| Module | Purpose |
|---|---|
| [`audio`](/shell/config/modules/audio) | Speaker volume and mute, the microphone's state, and a capture indicator. |
| [`background-apps`](/shell/config/modules/background-apps) | Background apps indicator. |
| [`battery`](/shell/config/modules/battery) | Battery level, charging state, and a dropdown with the batteries of peripherals the kernel reports (wireless mice, keyboards, controllers, pens) and power-profile controls. Low and critical battery levels send critical notifications whether or not the module is placed. |
| [`bitwarden`](/shell/config/modules/bitwarden) | Bitwarden vault: the bar shows whether it is locked; the dropdown logs in, unlocks, searches the vault and copies an item's username, password or one-time code. |
| [`bluetooth`](/shell/config/modules/bluetooth) | Bluetooth connection status with a dropdown for pairing and managing devices. |
| [`brightness`](/shell/config/modules/brightness) | Backlight control bar module. |
| [`cava`](/shell/config/modules/cava) | Audio frequency bars visualizing the output stream. |
| [`clock`](/shell/config/modules/clock) | Clock module configuration. |
| [`cpu`](/shell/config/modules/cpu) | CPU usage, frequency, and temperature. |
| [`custom`](/shell/config/modules/custom) | User-defined module that runs a shell command and renders the output in the bar. |
| [`dashboard`](/shell/config/modules/dashboard) | Quick-access button with a distro icon; opens the dashboard dropdown. |
| [`dropdown-dashboard-user-session`](/shell/config/modules/dropdown-dashboard-user-session) | Settings for user session the in dashboard |
| [`idle-inhibit`](/shell/config/modules/idle-inhibit) | Toggle that prevents screen dim, lock, and suspend while active. |
| [`keybind-mode`](/shell/config/modules/keybind-mode) | Current keybind-mode indicator for modal compositors. |
| [`keyboard-input`](/shell/config/modules/keyboard-input) | Active keyboard layout indicator. |
| [`mail`](/shell/config/modules/mail) | Unread mail count, backed by a notmuch query. |
| [`media`](/shell/config/modules/media) | Now-playing title and playback controls for the active MPRIS player. |
| [`minimized-windows`](/shell/config/modules/minimized-windows) | Minimized windows, like a dock. |
| [`netstat`](/shell/config/modules/netstat) | Network traffic counters (up/down rates). |
| [`network`](/shell/config/modules/network) | Network connection status with a dropdown for switching connections. |
| [`nightlight`](/shell/config/modules/nightlight) | Night light: warms the display colors, by hand or on a schedule. |
| [`notifications`](/shell/config/modules/notifications) | Notification center: icon in the bar, dropdown with history, DND toggle. |
| [`power`](/shell/config/modules/power) | Shutdown, reboot, and logout menu. |
| [`power-profiles`](/shell/config/modules/power-profiles) | Power profile indicator and switcher (lys-power). |
| [`ram`](/shell/config/modules/ram) | Memory and swap usage. |
| [`recorder`](/shell/config/modules/recorder) | Native screen recorder encoding on the GPU's video encoder. |
| [`screenshot`](/shell/config/modules/screenshot) | Screenshot capture button. |
| [`separator`](/shell/config/modules/separator) | A vertical rule between bar modules. |
| [`storage`](/shell/config/modules/storage) | Disk usage for a mount point. |
| [`systray`](/shell/config/modules/systray) | System tray icons via the StatusNotifierItem protocol. |
| [`treeman`](/shell/config/modules/treeman) | treeman worktree health across all registered repos, in a dropdown. |
| [`weather`](/shell/config/modules/weather) | Current conditions with hourly and daily forecasts in a dropdown. |
| [`window-title`](/shell/config/modules/window-title) | Active window title with optional app-icon prefix. |
| [`workspaces`](/shell/config/modules/workspaces) | Workspace indicators with click-to-switch, on whichever compositor runs. |
| [`world-clock`](/shell/config/modules/world-clock) | Multiple timezones shown together in a dropdown. |

## Shared types

Every named type referenced across the config (`Color`, `ClickAction`, `Size`, and others) is documented on the [types page](/shell/config/types).

</div>
