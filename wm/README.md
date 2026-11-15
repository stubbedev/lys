<p align="center">
  <img src="../docs/public/lys-wm.svg" width="160" alt="lys-wm">
</p>

<h1 align="center">lys-wm</h1>

<p align="center">
  The tiling Wayland compositor of <a href="../README.md">lys</a>, in pure Go (<code>CGO_ENABLED=0</code>).
</p>

<p align="center">
  <a href="https://stubbedev.github.io/lys/wm/">Documentation</a>
</p>

hy3-style tiling with splits and tab groups, on DRM/KMS through logind and
libinput, with NVIDIA and Mesa, several GPUs, the protocols a full desktop session
needs, and Xwayland for X11 programs. [Scope](https://stubbedev.github.io/lys/wm/guide/scope)
has the whole list and the non-goals.

## Install

The manual's [install section](https://stubbedev.github.io/lys/install/) covers
NixOS (`lys.nixosModules.lys-wm`), the Debian, Ubuntu and Fedora packages and the
tarball. [lys-wm's install page](https://stubbedev.github.io/lys/wm/guide/install)
lists what each installs and needs.

## Use

Pick the `lys-wm` session in your display manager, or run `lys-wm` inside another
Wayland session to get a nested window. `SUPER + Return` runs `foot`,
`SUPER + Q` closes a window and `SUPER + SHIFT + E` leaves the session.

```sh
lys-wmctl windows                # query a running lys-wm
lys-wmctl dispatch workspace 3   # or control it
```

- [Getting started](https://stubbedev.github.io/lys/wm/guide/getting-started): default binds and a first config
- [Config](https://stubbedev.github.io/lys/wm/config): every key of `~/.config/lys-wm/config.yaml`
- [Contributing](https://stubbedev.github.io/lys/wm/guide/contributing): development setup
