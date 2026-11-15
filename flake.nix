{
  description = "lys - a pure-Go Wayland desktop: compositor, shell, login, settings and the libraries under them";

  nixConfig = {
    extra-substituters = ["https://nix.stubbe.dev/c/default/default"];
    extra-trusted-public-keys = ["default:6uWvXutL9cXjV3lii+Ur5ff+ArQoG4kMBKNXWrIxhHg="];
  };

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    # A second, newer Mesa the lib GPU gate also runs against, so a driver
    # release breaks a gate before it reaches a desktop. Locked on its own:
    # `nix flake update nixpkgs-mesa` moves it without moving the rest.
    nixpkgs-mesa.url = "github:NixOS/nixpkgs/nixos-unstable";
    lys-ci.url = "github:stubbedev/lys-ci";
    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };
    devtools = {
      url = "github:stubbedev/flakes.nix";
      inputs.flake-parts.follows = "flake-parts";
    };
    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = inputs @ {flake-parts, ...}: let
    checks = import ./nix/namespace.nix inputs.nixpkgs.lib;
  in
    flake-parts.lib.mkFlake {inherit inputs;} {
      systems = ["x86_64-linux" "aarch64-linux"];
      imports = [
        inputs.home-manager.flakeModules.home-manager
        ./nix/go.nix
        ./nix/shell.nix
        ./nix/overlay.nix
        (checks "lib" ./lib/nix/checks.nix)
        ./lib/nix/shell.nix
        ./ui/nix/package.nix
        (checks "ui" ./ui/nix/checks.nix)
        ./ui/nix/shell.nix
        ./desktop/nix/shell.nix
        ./wm/nix/package.nix
        ./wm/nix/shell.nix
        ./wm/nix/module.nix
        (checks "wm" ./wm/nix/checks.nix)
        ./shell/nix/package.nix
        ./shell/nix/shell.nix
        ./shell/nix/modules.nix
        (checks "shell" ./shell/nix/checks.nix)
        ./login/nix/package.nix
        ./login/nix/shell.nix
        ./login/nix/modules.nix
        (checks "login" ./login/nix/checks.nix)
        (checks "login" ./login/nix/distro-vm.nix)
        ./settings/nix/package.nix
        ./settings/nix/shell.nix
        ./settings/nix/modules.nix
        (checks "settings" ./settings/nix/checks.nix)
        ./polkit/nix/package.nix
        ./polkit/nix/shell.nix
        ./polkit/nix/modules.nix
        (checks "polkit" ./polkit/nix/checks.nix)
        ./secrets/nix/package.nix
        ./secrets/nix/shell.nix
        ./secrets/nix/modules.nix
        (checks "secrets" ./secrets/nix/checks.nix)
        ./splash/nix/package.nix
        ./splash/nix/shell.nix
        ./splash/nix/module.nix
        (checks "splash" ./splash/nix/checks.nix)
      ];
      flake.lib = {
        gpuDriverEnv = import ./lib/nix/gpu-drivers.nix;
        screencast = import ./wm/nix/screencast.nix;
      };
    };
}
