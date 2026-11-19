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
| `--setting <path>` | Open on the page holding a setting, with its control focused. A lys-shell config path (`bar.location`), or `wm:` and a lys-wm path (`wm:input.touchpad.natural_scroll`); `wm:` may be left out where lys-shell has no such path |
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
| Connectivity | `wifi` |
| Appearance | `theme`, `wallpaper`, `fonts`, `motion` |
| Displays | `wm-outputs` (the arrangement and lys-wm's display keys with lys-wm; lys-shell's tearing mode) |
| Sound | `sound` |
| Input | `wm-input`, `wm-binds` |
| Notifications & Focus | `notifications` (with Do Not Disturb and the bar's bell), `osd` |
| Power & Battery | `idle` |
| Apps | `default-apps`, `login-items`, `launcher`, `launcher-modes` |
| Privacy & Security | `privacy` (with the screen share picker), `passwords`, `lock` (the lock and login screens) |
| Time & Language | `date-time`, `region` |
| Desktop | `bar` (with its buttons and dropdowns; each dropdown's size is on its bar item's page), `dock`, `wm-windows`, `wm-effects`, `wm-tiling`, `wm-session` |
| Bar Items: Status | `audio`, `background-apps`, `battery`, `bluetooth`, `brightness`, `cpu`, `dashboard`, `idle-inhibit`, `keybind-mode`, `keyboard-input`, `netstat`, `network`, `nightlight`, `power`, `power-profiles`, `ram`, `storage`, `systray` |
| Bar Items: Media | `cava`, `media`, `recorder`, `screenshot` |
| Bar Items: Productivity | `bitwarden`, `clock`, `custom`, `mail`, `separator`, `treeman`, `weather`, `world-clock` |
| Bar Items: Workspaces & Windows | `minimized-windows`, `window-title`, `workspaces` (one page for every compositor, showing what the running one reads) |

The `wm-` pages and `wm-outputs` are lys-wm's; see [lys-wm settings](./lys-wm).

lys-shell's pages edit its config and work without the shell running.

Some pages also act on the system:

| Page | What it changes | Through |
| --- | --- | --- |
| Users | Your name, avatar and password (the password change also re-keys the lys-secrets keyring); enrolled fingerprints; for administrators, adding and deleting users and their account type | accounts-daemon, PAM, fprintd |
| Wi-Fi & Network | Wi-Fi on or off, airplane mode, the networks in range (connect with a password, disconnect, forget a saved one), joining a hidden network; the wired link's speed and address. Enterprise networks sign in from the bar's network menu, whose secret agent answers NetworkManager | NetworkManager, rfkill |
| Date & Time | Time zone, network time, manual time; the bar clock's 12- or 24-hour format | systemd-timedated |
| Language & Region | System language, console keymap, the login screen's keyboard layout (copied from lys-wm); your date, number, currency, measurement and paper formats in `~/.config/locale.conf` | systemd-localed |
| Sound | Output and input devices, volume, balance, input level, per-app volume; notification sounds | PulseAudio protocol |
| Login Items | XDG autostart entries and systemd user services | Autostart directories, systemd user manager |
| Default Apps | Default app per category and file type; the terminal | `mimeapps.list` |
| Passwords | Filter, reveal and copy keyring items; how many minutes until a reveal asks for the collection password again | Secret Service |
| Privacy | Per-app portal grants: background and notifications switch on or off, GameMode and realtime are read-only, others can be revoked; the screen-share reminder | xdg-desktop-portal permission store |
| Login & Lock Screen | **Apply to login screen** sends the background, clock and login screen settings to lys-login's display manager, which asks polkit when you are not the logged-in user; the footer says whether they were applied, invalid, not authorized, or the authentication was dismissed | `/run/lys-login/apply.sock` |
| Displays (`wm-outputs`) | Output layout, tested and applied live | wlr-output-management |

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
