{
  perSystem = {
    lysGo,
    pkgs,
    ...
  }: {
    packages.lys-settings = pkgs.callPackage ./lys-settings.nix {inherit lysGo;};
  };
}
