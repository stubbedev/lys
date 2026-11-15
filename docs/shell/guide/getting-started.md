---
title: Getting started
---

# Getting started

lys-shell is a Wayland desktop shell written in Go on [lys-ui](/ui/). It runs on any compositor with `wlr-layer-shell` and draws a bar, dropdowns, notifications, an OSD, a launcher and the wallpaper. It is also an xdg-desktop-portal backend. Workspace modules exist for lys-wm, Hyprland, Niri, Mango and Sway.

![lys-shell's bar and dropdowns on a desktop](/lys-shell-preview.png)

![lys-settings editing lys-shell's config](/lys-settings-preview.png)

## Install

[Install](/install/) covers the packages, the tarball and the flake; [NixOS](/shell/guide/getting-started-nixos) has the module options. lys-shell's packages install the binaries, the systemd user units, the portal and D-Bus files, the icons, shell completions and a GeoClue drop-in.

### Build from source

The build needs Go. The binaries use no cgo. `go.mod` names the Go version; any `go` from 1.21 downloads that toolchain itself.

```sh
git clone https://github.com/stubbedev/lys
cd lys
CGO_ENABLED=0 go build -o dist/ ./shell/cmd/...
sudo install -Dm755 dist/lys-shell dist/lys-lock -t /usr/local/bin
lys-shell icons setup
```

`lys-shell icons setup` copies the bundled icons into `~/.local/share/lys-shell/icons`. The packages install them system-wide instead.

### Runtime dependencies

lys-shell loads these libraries at runtime:

| Library | Used for |
| --- | --- |
| Vulkan | GPU rendering, zero-copy screen sharing, the screen recorder. Without it, rendering and screen sharing fall back to the CPU. |
| VA-API | The screen recorder on GPUs whose Vulkan driver has no video encoder (Intel Xe2). |

The recorder needs a GPU that encodes H.264 through Vulkan Video or VA-API; otherwise it reports why it cannot record. Screen sharing talks to the PipeWire daemon (1.0 or newer) directly.

Modules talk to these services. Install the ones you use:

| Service | Modules |
| --- | --- |
| PipeWire with WirePlumber and pipewire-pulse | `audio` |
| NetworkManager | `network` |
| BlueZ | `bluetooth` |
| UPower | `battery` |
| power-profiles-daemon | `power-profiles` |
| GeoClue | `nightlight` auto-schedule without coordinates |
| tesseract | `lys-shell screenshot text` |

::: code-group

```sh [Arch]
sudo pacman -S --needed vulkan-icd-loader libva intel-media-driver
sudo pacman -S --needed bluez bluez-utils networkmanager upower \
  power-profiles-daemon pipewire wireplumber pipewire-pulse
sudo systemctl enable --now bluetooth NetworkManager upower power-profiles-daemon
```

```sh [Debian / Ubuntu]
sudo apt install libvulkan1 libva2 libva-drm2 intel-media-va-driver mesa-va-drivers
sudo apt install dbus-user-session bluez network-manager \
  upower power-profiles-daemon pipewire-pulse wireplumber
sudo systemctl enable --now bluetooth NetworkManager upower power-profiles-daemon
```

```sh [Fedora]
sudo dnf install vulkan-loader libva intel-media-driver mesa-va-drivers
# Workstation ships these; Minimal and Server need them:
sudo dnf install pipewire-pulseaudio wireplumber NetworkManager bluez upower \
  power-profiles-daemon
```

:::

## Run

```sh
systemctl --user enable --now lys-shell.service   # packaged install
lys-shell panel start                             # or: start it in the background
lys-shell shell                                   # or: run it in the foreground, logging to the terminal
```

`lys-shell panel status`, `panel restart` and `panel stop` manage a running shell. Only one shell runs per session.

## Configure

lys-shell reads `$XDG_CONFIG_HOME/lys-shell/config.yaml` (default `~/.config/lys-shell/config.yaml`) and applies changes on save. A missing file is created holding only the schema modeline; every field has a default. See [Editing config](/shell/guide/editing-config) and the [config reference](/shell/config/).

`lys-shell panel settings` opens [lys-settings](/settings/), which edits the same config live.

## Lock screen

Locking is done by [lys-login](/login/), which must be on the `PATH`. `lys-shell lock`, logind's Lock signal and `login.lock.lock-on-start` each start `lys-login lock` as its own process, which survives a shell restart. If that process dies while the session is still locked, the shell starts it again. On Hyprland this needs `hl.config({ misc = { allow_session_lock_restore = true } })`.
