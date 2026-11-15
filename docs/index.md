---
layout: home

hero:
  name: lys
  text: A Wayland desktop in pure Go
  tagline: A compositor, a shell, a display manager, a settings app and the session services around them, built from one repository with CGO_ENABLED=0.
  image:
    src: /lys.svg
    alt: lys
  actions:
    - theme: brand
      text: Install
      link: /install/
    - theme: alt
      text: GitHub
      link: https://github.com/stubbedev/lys

features:
  - title: lys-shell
    details: The desktop shell. Bar, notifications, OSD, launcher, wallpaper, dialogs and an xdg-desktop-portal backend, on any compositor with wlr-layer-shell.
    link: /shell/
    icon:
      src: /lys.svg
  - title: lys-wm
    details: The compositor. hy3-style tiling on DRM/KMS, the protocols a desktop session needs, and Xwayland.
    link: /wm/
    icon:
      src: /lys-wm.svg
  - title: lys-login
    details: The display manager, greeter and lock screen. Starts sessions through PAM and shows the greeter in a lys-wm kiosk.
    link: /login/
    icon:
      src: /lys-login.svg
  - title: lys-settings
    details: One window for the settings of lys-shell, lys-wm and the system services they use.
    link: /settings/
    icon:
      src: /lys-settings.svg
  - title: lys-polkit
    details: The polkit authentication agent. Shows polkitd's requests in lys-shell's dialog.
    link: /polkit/
    icon:
      src: /lys-polkit.svg
  - title: lys-power
    details: Battery charge limits that learn when you unplug, and power profiles that follow the power source.
    link: /power/
    icon:
      src: /lys-power.svg
  - title: lys-secrets
    details: The freedesktop Secret Service, with collections encrypted at rest, and your Bitwarden vault for lys-shell.
    link: /secrets/
    icon:
      src: /lys-secrets.svg
  - title: lys-splash
    details: The boot splash, in place of Plymouth. Starts in the initrd, asks for disk passwords and hands its last frame to the display manager.
    link: /splash/
    icon:
      src: /lys-splash.svg
---

## The components

The apps are what you install and run. Each is one binary, built and released
with the others under one version.

| Component | What it is | Binary |
| --- | --- | --- |
| [lys-shell](/shell/) | Desktop shell: bar, notifications, launcher, portal | `lys-shell` |
| [lys-wm](/wm/) | Tiling Wayland compositor | `lys-wm`, `lys-wmctl` |
| [lys-login](/login/) | Display manager, greeter and lock screen | `lys-login` |
| [lys-settings](/settings/) | Settings app for lys-shell and lys-wm | `lys-settings` |
| [lys-polkit](/polkit/) | Polkit authentication agent | `lys-polkit` |
| [lys-power](/power/) | Battery charging and power profile policy | `lys-power` |
| [lys-secrets](/secrets/) | Secret Service and Bitwarden vault | `lys-secrets` |
| [lys-splash](/splash/) | Boot splash | `lys-splash` |

[Install](/install/) sets up the whole desktop or any part of it.

## The libraries

The apps are built on three Go libraries. They are part of the
`github.com/stubbedev/lys` module, for anyone writing Wayland programs in Go.

| Library | What it is | Import path |
| --- | --- | --- |
| [lys-lib](/lib/) | Rendering, Vulkan, Wayland, media, PipeWire, D-Bus and PAM | `github.com/stubbedev/lys/lib/...` |
| [lys-ui](/ui/) | Widget toolkit: components, widgets, CSS, accessibility | `github.com/stubbedev/lys/ui/...` |
| [lys-desktop](/desktop/) | The config, theming, translations and service clients the apps share | `github.com/stubbedev/lys/desktop/...` |

```sh
go get github.com/stubbedev/lys
```
