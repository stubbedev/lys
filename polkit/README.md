<p align="center">
  <img src="../docs/public/lys-polkit.svg" width="160" alt="lys-polkit">
</p>

<h1 align="center">lys-polkit</h1>

<p align="center">
  The polkit authentication agent of <a href="../README.md">lys</a>.
</p>

<p align="center">
  <a href="https://stubbedev.github.io/lys/polkit/">Documentation</a>
</p>

lys-polkit registers with polkitd as the session's authentication agent. When a
program requests an action that needs authentication (`pkexec`, a settings
panel, a package manager), lys-polkit runs the PAM conversation through
polkit's agent helper and shows it in
[lys-shell](../shell)'s authentication dialog.

## Install

With home-manager:

```nix
{
  inputs.lys.url = "github:stubbedev/lys";

  # in the home-manager configuration
  imports = [inputs.lys.homeManagerModules.lys-polkit];
  services.lys-polkit.enable = true;
  services.hyprpolkitagent.enable = false; # one agent per session
}
```

The manual's [install section](https://stubbedev.github.io/lys/install/) covers
the packages; [Install](https://stubbedev.github.io/lys/polkit/guide/install) the
rest.

## Usage

It runs as a systemd user service with the graphical session and needs
polkitd, systemd-logind and lys-shell.

```sh
systemctl --user status lys-polkit
pkexec --disable-internal-agent true   # opens the dialog
lys-polkit revoke                      # drop retained admin authorizations, like sudo -k
```
