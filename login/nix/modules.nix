{self, ...}: {
  flake = {
    # NixOS: `imports = [self.nixosModules.lys-login]; programs.lys-login.greeter.enable = true;`
    nixosModules.lys-login = import ./nixos-module.nix self;

    # home-manager: `imports = [lys.homeManagerModules.lys-login];`
    homeManagerModules.lys-login = import ./hm-module.nix self;
  };
}
