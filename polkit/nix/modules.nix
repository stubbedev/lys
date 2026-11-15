{self, ...}: {
  flake.homeManagerModules.lys-polkit = import ./hm-module.nix self;
}
