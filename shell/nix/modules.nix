{ self, ... }:
{
  flake = {
    # NixOS: `imports = [ lys.nixosModules.lys-shell ]; programs.lys-shell.enable = true;`
    nixosModules.lys-shell = import ./nixos-module.nix self;

    # home-manager: `imports = [ lys.homeModules.lys-shell ]; programs.lys-shell.enable = true;`
    homeModules.lys-shell = import ./hm-module.nix self;
  };
}
