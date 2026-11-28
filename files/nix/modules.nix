{self, ...}: {
  flake.nixosModules.lys-files = import ./nixos-module.nix self;
}
