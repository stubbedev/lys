{
  perSystem = {
    lysGo,
    pkgs,
    ...
  }: {
    packages.lys-viewer = pkgs.callPackage ./lys-viewer.nix {inherit lysGo;};
  };
}
