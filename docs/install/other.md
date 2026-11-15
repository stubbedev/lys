# Other systems

## Tarballs

Each [release](https://github.com/stubbedev/lys/releases/latest) has a
`lys-<app>-<version>-<arch>-linux.tar.gz` per app, `<arch>` being `x86_64` or
`aarch64`. A tarball holds the binaries, the units, data files and licences
that the app's packages install. Where each file goes, and what the binaries
load at run time, is on the app's page:

| App | Tarball install |
| --- | --- |
| lys-wm | [Tarball](/wm/guide/install#tarball) |
| lys-login | [Display manager from the archive](/login/install/other#display-manager-from-the-archive) |
| lys-settings | [Archive](/settings/guide/install#archive) |
| lys-power | [Other distributions](/power/#other-distributions) |
| lys-secrets | [Other distributions](/secrets/guide/getting-started#other-distributions) |
| lys-shell, lys-polkit | [lys-shell](/shell/guide/getting-started#install), [lys-polkit](/polkit/guide/install) |

The lys-login greeter also runs under greetd without the display manager:
[Greeter under greetd](/login/install/other#greeter-under-greetd).

## home-manager on any distribution

With Nix and home-manager, import the flake's home-manager modules:

```nix
{
  inputs.lys.url = "github:stubbedev/lys";

  # in the home-manager configuration
  imports = [
    inputs.lys.homeModules.lys-wm
    inputs.lys.homeModules.lys-shell
    inputs.lys.homeModules.lys-polkit
    inputs.lys.homeModules.lys-secrets
  ];
  programs.lys-wm.enable = true;
  programs.lys-shell.enable = true;
  services.lys-polkit.enable = true;
  services.lys-secrets.enable = true;
}
```

home-manager installs user packages and units only. The display manager,
portals, PAM services, login limits, GPU drivers and system services stay the
host's: each app's install page says what is left to do.

## From source

Every app is a `go install` away. The binaries use no cgo; `go.mod` names the Go
version, which any `go` from 1.21 on downloads itself.

```sh
go install github.com/stubbedev/lys/wm/cmd/lys-wm@latest
go install github.com/stubbedev/lys/wm/cmd/lys-wmctl@latest
go install github.com/stubbedev/lys/shell/cmd/lys-shell@latest
go install github.com/stubbedev/lys/login/cmd/lys-login@latest
go install github.com/stubbedev/lys/settings/cmd/lys-settings@latest
go install github.com/stubbedev/lys/polkit/cmd/lys-polkit@latest
go install github.com/stubbedev/lys/power/cmd/lys-power@latest
go install github.com/stubbedev/lys/secrets/cmd/lys-secrets@latest
go install github.com/stubbedev/lys/splash/cmd/lys-splash@latest
```

This installs the binaries only. Units, session files, PAM services, icons and
portal files come from the packages, the tarballs or a clone of the
repository.
