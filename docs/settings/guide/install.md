# Install

[Install](/install/) covers the packages and the flake for the whole desktop.
This page is what is particular to lys-settings.

## Nix

lys-shell's NixOS and Home Manager modules install lys-settings by default:

```nix
programs.lys-shell.settingsApp.enable = false;              # leave it out
programs.lys-shell.settingsApp.package = pkgs.lys-settings; # or use another package
```

To install it alone, as on lys-wm without the shell:

```nix
{
  inputs.lys.url = "github:stubbedev/lys";

  imports = [inputs.lys.homeModules.lys-settings]; # or nixosModules.lys-settings
  programs.lys-settings.enable = true;
}
```

| Option | Default | Effect |
| --- | --- | --- |
| `programs.lys-settings.enable` | `false` | Install the package |
| `programs.lys-settings.package` | the flake's lys-settings | Package to install |

The flake's `overlays.default` adds `pkgs.lys-settings`.

## Debian, Ubuntu and Fedora

The `.deb` and `.rpm` recommend lys-shell or lys-wm.

## Archive

```sh
tar xzf lys-settings-*-linux.tar.gz
cd lys-settings-*-linux
sudo install -Dm755 lys-settings -t /usr/local/bin
sudo install -Dm644 dev.stubbe.lys.settings.desktop -t /usr/local/share/applications
sudo install -Dm644 lys-settings.svg -t /usr/local/share/icons/hicolor/scalable/apps
sudo mkdir -p /usr/local/share/lys-settings
sudo cp -r icons /usr/local/share/lys-settings/
```

The app loads its icons from `../share/lys-settings/icons` relative to its binary.

## Versions

lys-settings knows the lys-shell keys of the release it comes with. lys-wm's
rules are read at run time from the `lys-wm` on `PATH`: a key that lys-wm does
not know is refused with its message, and a lys-wm that cannot report its schema
or built-in config leaves the **Compositor** pages showing the error.
