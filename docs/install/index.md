# Install

A lys desktop is a set of apps that work together. Install all of them for the
whole desktop, or only the ones you want next to another desktop.

| App | Role in the desktop | Needs |
| --- | --- | --- |
| [lys-wm](/wm/) | The compositor and the `lys-wm` session | |
| [lys-shell](/shell/) | Bar, notifications, launcher, wallpaper, dialogs and the desktop portal | any compositor with `wlr-layer-shell` |
| [lys-login](/login/) | Display manager, greeter and lock screen | lys-wm; lys-shell locks through it |
| [lys-settings](/settings/) | The settings window | lys-shell or lys-wm |
| [lys-polkit](/polkit/) | Polkit authentication agent | lys-shell's dialogs |
| [lys-power](/power/) | Adaptive battery charging and power profile switching | power-profiles-daemon and UPower, optional |
| [lys-secrets](/secrets/) | Secret Service, unlocked at login | lys-shell's dialogs for prompts |
| [lys-splash](/splash/) | Boot splash | a systemd initrd or initramfs-tools |

## Releases

Every app is released together under one version. Each `vX.Y.Z` tag on
[stubbedev/lys](https://github.com/stubbedev/lys/releases) has, for every
app, a Debian package, an RPM and a tarball for x86_64 and aarch64. There is no
apt or dnf repository, so packages do not update themselves. The Nix flake
builds from source.

## Pick your system

- [NixOS](./nixos): the flake's NixOS and home-manager modules.
- [Debian and Ubuntu](./debian): the `.deb` packages.
- [Fedora](./fedora): the `.rpm` packages.
- [Other systems](./other): the tarballs, home-manager on any distribution, or
  `go install`.
