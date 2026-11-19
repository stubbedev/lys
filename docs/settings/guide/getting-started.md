# Getting started

Open the window from the launcher, the desktop menu or a terminal:

```sh
lys-settings
lys-settings --page wm-input
```

A second launch raises the open window on the page or setting it names.

## Command line

| Option | Effect |
| --- | --- |
| `--page <id>` | Open on a page |
| `--setting <path>` | Open on the page holding a setting, with its control focused. A lys-shell config path (`bar.location`), or `wm:` and a lys-wm path (`wm:input.touchpad.natural_scroll`); `wm:` may be left out where lys-shell has no such path. A control a page reads from the system rather than a config, such as the time zone, has a path under its page's ID (`date-time.time-zone`, `region.lc-paper`, `sound.balance`) and opens that page. The Users page's parts open it by `users.name`, `users.picture`, `users.password`, `users.fingerprint` and `users.add` |
| `--list-pages` | Print every page and its settings as JSON and exit |
| `-V`, `--version` | Print the version |
| `-h`, `--help` | Print help |

An unknown setting, or a `--page` that does not hold the `--setting`, exits with
status 2. **Ctrl+F** focuses the search field, which finds settings by name,
description and keywords.

## Pages

The window opens on the overview, a tile per group. The sidebar groups pages by
what they are for, so one page may hold lys-shell's and lys-wm's settings side
by side. With only one of the two installed (on `PATH`), the other's rows are
hidden, and a page left with nothing to show is gone; search, `--list-pages` and
`--setting` follow the same rule.

| Sidebar group | Page IDs |
| --- | --- |
| Home | `overview` |
| Account | `users` |
| Connectivity | `wifi`, `bluetooth-devices` |
| Appearance | `theme`, `wallpaper`, `fonts`, `motion` |
| Displays | `wm-outputs` (the arrangement and lys-wm's display keys with lys-wm; lys-shell's tearing mode) |
| Sound | `sound` |
| Input | `wm-keyboard` (layouts, typing, keyboard accessibility, the cursor), `wm-pointer` (mouse, touchpad, gestures, trackpoint), `wm-binds` (keyboard shortcuts, app shortcuts, modes) |
| Notifications & Focus | `notifications` (with Do Not Disturb and the bar's bell), `osd` |
| Power & Battery | `power-battery` (with the battery alerts and when to switch to power saver), `idle` |
| Apps | `default-apps` (with the terminal the launcher opens), `login-items`, `launcher`, `launcher-modes` |
| Privacy & Security | `privacy` (with the screen share picker), `passwords`, `lock` (the lock and login screens: the background is picked by picture and shows only the chosen kind's settings; the PAM service sits under **Unlocking**, folded away; links lead to when it locks and to fingerprints) |
| Time & Language | `date-time`, `region` |
| Desktop | `bar` (with its buttons and dropdowns; each dropdown's size is on its bar item's page), `dock`, `wm-windows`, `wm-effects`, `wm-tiling`, `wm-session` |
| Bar Items: Status | `audio`, `background-apps`, `battery`, `bluetooth`, `brightness`, `cpu`, `dashboard`, `idle-inhibit`, `keybind-mode`, `keyboard-input`, `netstat`, `network`, `nightlight`, `power`, `power-profiles`, `ram`, `storage`, `systray` |
| Bar Items: Media | `cava`, `media`, `recorder`, `screenshot` |
| Bar Items: Productivity | `bitwarden`, `clock`, `custom`, `mail`, `separator`, `treeman`, `weather`, `world-clock` |
| Bar Items: Workspaces & Windows | `minimized-windows`, `window-title`, `workspaces` (one page for every compositor, showing what the running one reads) |

The `wm-` pages and `wm-outputs` are lys-wm's; see [lys-wm settings](./lys-wm).

A bar item page only sets how the item looks and acts in the bar. Where the
feature itself has a page, a **Related** link at its foot opens it: Audio opens
Sound, Battery opens Battery & Power Mode, Bluetooth the Bluetooth devices,
Clock Date & Time, Keep awake Idle & Power, Keybind Mode Keys & Gestures, and
Keyboard Input the lys-wm Input page. A link to a page that is not installed is
left out.

A row that has no effect in the current state stays out of the way: the dock's
rows are greyed out while the dock is off, and a border's width and color, the
dock's hide delay and zoom size, the media icon rows of an icon type that does not use them, and the icon padding of a bar button style
without an icon box are hidden until they apply. Search and `--setting` for a
hidden row land on the row that hides it.

Displays are chosen, never typed: the dock shows on **All displays** or on the
tiles chosen, and each bar layout is for All displays or a connected display
and starts from nothing or another layout. A display listed but not connected
is kept, and named, until it is removed.

A list of fixed choices names each choice once, so none can be listed twice:
where the order counts (the dashboard's session buttons, the launcher's match
fields and search providers) every choice has a switch and moves up or down;
where it means nothing (a password preset's character sets) the choices are
toggle chips.

Keyboard Input's short names are given to layouts, never typed as layout
names: the layouts lys-wm is set up with are listed by their full name for a
short one, any other xkb layout or variant can be added from a list, and a
short name for a layout no longer set up stays until it is removed.

The dashboard's alarm levels are three bars with two stops each, which cannot
cross: usage (0–100 %) and CPU temperature (0–150 °C) turn to warning, then
error, as they rise; the battery turns to warning, then critical, at or below
its stops.

The Cava page picks its style and direction from drawings of the bars, takes
its frequency range as one range whose ends cannot cross, offers the framerate
as 30, 60, 120 or 144 fps (or a custom rate), and folds the smoothing away
under **Advanced**.

lys-shell's pages edit its config and work without the shell running.

Some pages also act on the system:

| Page | What it changes | Through |
| --- | --- | --- |
| Users | Your name, avatar and password (the password change also re-keys the lys-secrets keyring); enrolled fingerprints (the hands greyed out over "No fingerprint reader found", or what failed, when none can be enrolled); for administrators, adding users, their account type, and removing one after asking whether their home folder stays or is deleted | accounts-daemon, PAM, fprintd |
| Wi-Fi & Network | Wi-Fi on or off, airplane mode, the networks in range (connect with a password, disconnect, forget a saved one), joining a hidden network; the wired link's speed and address; switching saved VPN and WireGuard profiles on and off and removing them. Enterprise networks sign in from the bar's network menu, whose secret agent answers NetworkManager | NetworkManager, rfkill |
| Battery & Power Mode | Shows the battery's charge, state, time to full or empty and health, and says so when there is no battery or UPower is not running; turns the charge limit on or off where the battery supports one; switches the power mode, and says so when power-profiles-daemon is not running | UPower, power-profiles-daemon |
| Bluetooth | Bluetooth on or off and the name other devices see; paired devices with their type, battery level, connect, disconnect, rename and forget; searching for nearby devices and pairing them, with the PIN, passkey or confirmation the pairing asks for answered on the page. Its pairing agent answers only the pairing the page starts; lys-shell stays BlueZ's default agent for requests from other devices | BlueZ |
| Date & Time | Time zone, network time, manual time; the bar clock's 12- or 24-hour format | systemd-timedated |
| Language & Region | System language, console keymap, the login screen's keyboard layout (copied from lys-wm); your date, number, currency, measurement and paper formats in `~/.config/locale.conf` | systemd-localed |
| Sound | Output and input devices, volume, balance, input level, per-app volume; notification sounds | PulseAudio protocol |
| Login Items | XDG autostart entries and systemd user services | Autostart directories, systemd user manager |
| Default Apps | Default app per category and file type; the terminal | `mimeapps.list` |
| Passwords | Filter, reveal and copy keyring items; how many minutes until a reveal asks for the collection password again | Secret Service |
| Privacy | Per-app portal grants: background and notifications switch on or off, GameMode and realtime are read-only, others can be revoked; the screen-share reminder; a link to the online lyrics on the Media page, which send every track played to lrclib.net | xdg-desktop-portal permission store |
| Login & Lock Screen | **Apply to login screen** sends the background, clock and login screen settings to lys-login's display manager, which asks polkit when you are not the logged-in user; the footer says whether they were applied, invalid, not authorized, or the authentication was dismissed, and whether the login screen has changes not applied yet (the last applied settings are kept in `~/.local/state/lys-settings/greeter-applied.yaml`) | `/run/lys-login/apply.sock` |
| Notifications | **Send Test Notification** shows a notification with the settings on the page; the page says why when nothing answers | org.freedesktop.Notifications |
| On-Screen Display | **Show OSD** shows a volume toast with the settings on the page; each toast preset's **Show** shows that preset and the card names the `lys-shell toast --preset` line that does the same | `lys-shell toast` |
| Displays (`wm-outputs`) | Output layout, tested and applied live | wlr-output-management |

## Durations and sizes

A time a setting waits, such as how long a notification stays, how soon the
lock screen asks for the password or how often the wallpaper changes, is picked
from a row of common durations in words, as are the system monitors' poll
intervals, how soon Bitwarden clears the clipboard and how long keeping awake
lasts. Where 0 means something it has its own chip that says what
(**Never**, **Immediately**, **Off**, **Instant**, **Until turned off**);
where it means nothing, as for a poll interval, it is not offered.
**Custom…** takes any other value in the unit named beside it (seconds,
minutes or days); a value off the row shows there.
Counts and sizes with a few usual values, such as the launcher's history or the
login screen's cursor size, are picked the same way.

## Where changes go

| Pages | Written to | Takes effect |
| --- | --- | --- |
| lys-shell's | `~/.config/lys-shell/runtime.yaml` | At once, through the shell's config watcher |
| Compositor | `~/.config/lys-wm/runtime.yaml` | At once on a running lys-wm, through `lys-wmctl reload` |

Both files are overrides over the project's `config.yaml`, which the app only reads.

## Source badges and reset

| Badge | Meaning |
| --- | --- |
| **custom** | An override of a key `config.yaml` does not set |
| **override** | An override that replaces a `config.yaml` value |
| **config** | A `config.yaml` value that differs from the default |

A row's reset button, shown while the value differs from the default, puts the
setting back to its default. Where `config.yaml` sets another value, it writes an
override holding the default. Clearing a value removes its override.

**Reset all** asks for confirmation, then removes every override of both lys-shell
and lys-wm, so `config.yaml`, then the defaults, apply again.
