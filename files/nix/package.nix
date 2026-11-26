{
  perSystem = {
    lysGo,
    pkgs,
    ...
  }: {
    packages.lys-files = pkgs.callPackage ./lys-files.nix {inherit lysGo;};
  };
}
