# Install

[Install](/install/) sets up the whole desktop. This page is what lys-wm's
packages, tarball and Nix modules hold and need.

Every install provides:

- `lys-wm` and `lys-wmctl`
- the `lys-wm` display manager session (`lys-wm --systemd`)
- the `lys-wm-session.target` systemd user unit
- `lys-wm-portals.conf`, the [portal routing](../config#session)
- login limits (`rtprio 1`, `nice -10`) so the compositor thread can run
  with real-time priority

## NixOS

Import `lys.nixosModules.lys-wm` ([NixOS](/install/nixos) adds the flake) and
set `programs.lys-wm.enable = true`.

| Option | Default | Effect |
| --- | --- | --- |
| `programs.lys-wm.enable` | `false` | Install lys-wm, register the session, the user unit and portal config, set the login limits, enable `foot` and the NixOS Wayland session defaults (polkit, portals) |
| `programs.lys-wm.package` | the flake's build | The lys-wm package |

Set `programs.foot.enable = false` to leave out `foot`. lys-shell, which the
portal config prefers, is not included. GPU drivers come from
`/run/opengl-driver`.

## home-manager on another distribution

Import `lys.homeModules.lys-wm` and set
`programs.lys-wm.enable = true` (same `package` option). It installs lys-wm
and `foot`, and links `lys-wm-session.target` into `~/.config/systemd/user`.
The display manager, portals, login limits and GPU drivers stay the host's:
point the display manager at `lys-wm --systemd` yourself.
[NixOS](/install/nixos#packages-only) shows how to try lys-wm nested with
`nix run`, without installing it.

## Debian and Ubuntu

```sh
sudo apt install ./lys-wm_<version>_amd64.deb    # _arm64.deb on aarch64
```

- Depends: `libegl1`, `libgles2`, `libgbm1`, `libinput10`, `xkb-data`.
- Recommends: `xwayland`, `xdg-desktop-portal`, `lys-shell`, `foot`,
  `libvulkan1`, `mesa-vulkan-drivers`.

`lys-shell` is not in the distribution archive; it is in the same
[release](https://github.com/stubbedev/lys/releases/latest). It is the
session's only portal backend: without it, no portal answers.

## Fedora

```sh
sudo dnf install ./lys-wm-<version>-1.fc<release>.x86_64.rpm    # .aarch64.rpm on aarch64
```

- Requires: `libglvnd-egl`, `libglvnd-gles`, `mesa-libgbm`, `libinput`,
  `xkeyboard-config`.
- Recommends: `xorg-x11-server-Xwayland`, `xdg-desktop-portal`, `lys-shell`,
  `foot`, `vulkan-loader`, `mesa-vulkan-drivers`.

The lys-shell and portal notes above apply.

## Tarball

The tarball holds the binaries, session file, user unit, portal config,
limits file and licences. `<arch>` is `x86_64` or `aarch64`:

```sh
tar xzf lys-wm-<version>-<arch>-linux.tar.gz
cd lys-wm-<version>-<arch>-linux
sudo install -m 0755 lys-wm lys-wmctl /usr/bin/
sudo install -Dm 0644 lys-wm-session.target /usr/lib/systemd/user/lys-wm-session.target
sudo install -Dm 0644 lys-wm-portals.conf /usr/share/xdg-desktop-portal/lys-wm-portals.conf
sudo install -Dm 0644 lys-wm.desktop /usr/share/wayland-sessions/lys-wm.desktop
sudo install -Dm 0644 90-lys-wm.conf /etc/security/limits.d/90-lys-wm.conf
```

The binaries need the glibc loader (`/lib64/ld-linux-x86-64.so.2` or
`/lib/ld-linux-aarch64.so.1`) and these runtime dependencies. The overrides
work on every install.

| Dependency | Default | Override |
| --- | --- | --- |
| Vulkan | `libvulkan.so.1` | `LYS_LIBVULKAN` |
| EGL (GLES renderer, drm only) | `libEGL.so.1` | `LYS_WM_LIBEGL` |
| GLES2 (GLES renderer, drm only) | `libGLESv2.so.2` | `LYS_WM_LIBGLESV2` |
| GBM (scanout buffers) | `libgbm.so.1` | `LYS_WM_LIBGBM` |
| libinput | `libinput.so.10` | `LYS_WM_LIBINPUT` |
| xkeyboard-config | `/usr/share/X11/xkb` | `XKB_CONFIG_ROOT` |
| Tab font | Inter Regular, built in | `tabs.font` in the config |
| Xwayland | `Xwayland` on `PATH` | `LYS_WM_XWAYLAND` |

## Check it

```sh
lys-wm -backend none     # unknown backend "none" (available: drm, nested, headless)
lys-wmctl                # no command
```

Both exit 1, which shows the binaries load. Then
[start a session](./getting-started).
