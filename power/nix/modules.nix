{self, ...}: {
  flake.nixosModules.lys-power = import ./nixos-module.nix self;
}
