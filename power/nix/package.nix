{
  perSystem = {
    lysGo,
    pkgs,
    ...
  }: {
    packages.lys-power = pkgs.callPackage ./lys-power.nix {inherit lysGo;};
  };
}
