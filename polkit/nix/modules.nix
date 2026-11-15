{self, ...}: {
  flake.homeModules.lys-polkit = import ./hm-module.nix self;
}
