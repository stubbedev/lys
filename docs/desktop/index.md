# lys-desktop

The packages lys-shell, lys-login and lys-settings share, under
`github.com/stubbedev/lys/desktop`. It builds on [lys-ui](/ui/) and
[lys-lib](/lib/). Its API is shaped by those three apps and changes with
them; [lys-ui](/ui/) and [lys-lib](/lib/) are the libraries meant for other
programs.

```sh
go get github.com/stubbedev/lys
```

| Package | What it holds |
| --- | --- |
| `config` | lys-shell's config: discovery, imports, the types of every section, the JSON schema and the descriptions the [config reference](/shell/config/) is generated from |
| `styling` | Design tokens, palettes, themes and the colour providers (Matugen, Pywal, Wallust) |
| `i18n` | Fluent localization in English (US), German, Spanish, French, Italian, Dutch and European Portuguese, picked from `LANGUAGE`, `LC_ALL`, `LC_MESSAGES` and `LANG` by language (Brazilian Portuguese reads European Portuguese) with English as the fallback |
| `icons` | Icon sources (Tabler, Simple Icons, Material, Lucide), fetching and installing them |
| `service/...` | Clients of the session's services: audio over the PulseAudio protocol, MPRIS, UPower, notifications, idle, night light, clipboard, wallpaper, Bitwarden, the lys-wm IPC |
| `shell/...` | Widgets and helpers shared by the shell and the lock screen |
| `shell/barview` | The bar drawn from a config, without services: its zones, module buttons, separator and workspace strip, which lys-shell fills with live modules and lys-settings previews |
| `shell/osdview` | The on-screen display drawn from a config and an event, without services: the card lys-shell maps on a layer surface and lys-settings previews |
| `shell/notifyview` | The notification popup card drawn from a config and what a notification shows, without services, with the helpers the popups and the notification dropdown share; lys-shell stacks it and lys-settings previews it |
| `launchermatch` | The launcher's matcher: rofi's matching methods, case handling and sorting over a port of nucleo's fzf scoring, and the options `launcher` configures, which lys-shell filters its lists with and lys-settings' launcher preview matches its test query by |
| `shell/launcherview` | The launcher drawn from a config, without the engine, socket or surface: the panel and where it sits, its field, footer, list, rows and mode tabs, and the CSS a font or style preset adds, which lys-shell hosts on a layer surface and lys-settings previews |
| `shell/lockview` | The login screen drawn from a config, without PAM, input grabbing or a session: the background, clock, account and password prompt the greeter and the lock screen show and lys-settings previews |
| `settingspage` | The contract between lys-settings and the programs that open it: its command line and page list |
| `identity` | The desktop's directory name under each XDG base directory |
| `desktopentry`, `mime`, `opener` | Desktop entries, the shared-mime-info database, and opening a file with the default app |
| `spawn`, `shlex`, `strftime`, `glob`, `jinja`, `pango` | Running configured commands, splitting command lines, time formats, blocklist matching, the label templates module formats are written in, Pango markup read into lys-ui's rich labels |

The package documentation is on
[pkg.go.dev](https://pkg.go.dev/github.com/stubbedev/lys/desktop).
