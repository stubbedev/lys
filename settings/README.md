<p align="center">
  <img src="../docs/public/lys-settings.svg" width="160" alt="lys-settings">
</p>

<h1 align="center">lys-settings</h1>

<p align="center">
  The settings app of <a href="../README.md">lys</a>, for lys-shell and lys-wm.
</p>

<p align="center">
  <a href="https://stubbedev.github.io/lys/settings/">Documentation</a>
</p>

One window for the settings of
[lys-shell](../shell),
[lys-wm](../wm) and the system services they run on.
Config changes are written as runtime overrides to each project's `runtime.yaml`,
over the `config.yaml` you declare, which the app never writes. lys-wm checks every
change before it is written and reloads it after. A config that Home Manager links
into the Nix store stays editable.

## Install

lys-shell's Nix modules install it by default. The manual's
[install section](https://stubbedev.github.io/lys/install/) covers the packages,
and [Install](https://stubbedev.github.io/lys/settings/guide/install) installing it
alone (`lys.nixosModules.lys-settings`, `lys.homeModules.lys-settings`).

## Use

```sh
lys-settings                                          # open the window
lys-settings --page wm-keyboard                        # open on a page
lys-settings --setting input.touchpad.natural_scroll  # open on a setting, focused
lys-settings --list-pages                             # print the pages and settings as JSON
```

## Development

```sh
nix develop
just docker check  # every gate, in a container away from the desktop session
```

See [Development](https://stubbedev.github.io/lys/settings/guide/development) and [AGENTS.md](AGENTS.md).
