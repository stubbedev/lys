# NixOS

The repository is one flake, `github:stubbedev/lys`. It provides:

| Output | What it is |
| --- | --- |
| `packages.<system>.lys-<app>` | Each app: `lys-shell`, `lys-wm`, `lys-login`, `lys-settings`, `lys-polkit`, `lys-secrets`, `lys-splash` |
| `nixosModules.lys-<app>` | NixOS modules for lys-shell, lys-wm, lys-login, lys-settings, lys-secrets and lys-splash |
| `homeManagerModules.lys-<app>` | home-manager modules for lys-shell, lys-wm, lys-login, lys-settings, lys-polkit and lys-secrets |
| `overlays.default` | Adds every lys package to `pkgs` |

## The whole desktop

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    home-manager.url = "github:nix-community/home-manager";
    lys.url = "github:stubbedev/lys";
  };
  outputs = {nixpkgs, home-manager, lys, ...}: {
    nixosConfigurations.host = nixpkgs.lib.nixosSystem {
      modules = [
        ./configuration.nix
        lys.nixosModules.lys-wm
        lys.nixosModules.lys-shell
        lys.nixosModules.lys-login
        lys.nixosModules.lys-secrets
        lys.nixosModules.lys-splash
        home-manager.nixosModules.home-manager
        ({config, ...}: {
          programs.lys-wm.enable = true;
          programs.lys-shell.enable = true;
          programs.lys-login = {
            greeter.enable = true;
            greeter.compositorPackage = config.programs.lys-wm.package;
            lock.enable = true;
          };
          services.lys-secrets.enable = true;
          boot.lys-splash.enable = true;

          home-manager.users.alice = {
            imports = [lys.homeManagerModules.lys-polkit];
            services.lys-polkit.enable = true;
          };
        })
      ];
    };
  };
}
```

This gives:

- lys-wm with its session, portal routing and login limits;
- lys-shell with lys-settings, as the portal backend for every interface;
- lys-login as `display-manager.service`, and the lock screen's PAM service;
- lys-secrets as the Secret Service, unlocked when you log in;
- lys-splash in the systemd initrd;
- lys-polkit as the polkit agent of alice's session.

Leave out any module you do not want. Each app's page has its options:

| App | Module | Options |
| --- | --- | --- |
| lys-wm | NixOS, home-manager | [Install](/wm/guide/install#nixos) |
| lys-shell | NixOS, home-manager | [NixOS](/shell/guide/getting-started-nixos) |
| lys-login | NixOS, home-manager | [NixOS](/login/install/nixos) |
| lys-settings | NixOS, home-manager | [Install](/settings/guide/install) |
| lys-polkit | home-manager | [Install](/polkit/guide/install) |
| lys-secrets | NixOS, home-manager | [Nix options](/secrets/guide/configuration#nix-options) |
| lys-splash | NixOS | [Install](/splash/guide/install#nixos) |

## Packages only

To install the packages without the modules, add the overlay and name them:

```nix
{
  nixpkgs.overlays = [lys.overlays.default];
  environment.systemPackages = with pkgs; [lys-wm lys-shell lys-settings];
}
```

To try lys-wm nested in your current Wayland session without installing
anything:

```sh
nix run github:stubbedev/lys#lys-wm
nix run github:stubbedev/lys#lys-wm -- --renderer soft   # if the GPU driver is unusable from Nix
```
