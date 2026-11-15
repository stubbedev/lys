{ self, ... }:
{
  flake = {
    # NixOS: `imports = [ lys.nixosModules.lys-shell ]; programs.lys-shell.enable = true;`
    nixosModules.lys-shell = import ./nixos-module.nix self;

    # home-manager: `imports = [ lys.homeManagerModules.lys-shell ]; programs.lys-shell.enable = true;`
    homeManagerModules.lys-shell = import ./hm-module.nix self;
  };
}
