{self, ...}: {
  flake = {
    # NixOS: `imports = [self.nixosModules.lys-secrets]; services.lys-secrets.enable = true;`
    nixosModules.lys-secrets = import ./nixos-module.nix self;

    # home-manager: `imports = [lys.homeModules.lys-secrets]; services.lys-secrets.enable = true;`
    homeModules.lys-secrets = import ./hm-module.nix self;
  };
}
