{
  perSystem = {
    lysGo,
    pkgs,
    self',
    ...
  }: {
    packages.lys-viewer = pkgs.callPackage ./lys-viewer.nix {
      inherit lysGo;
      inherit (self'.packages) lys-docthumb;
    };
  };
}
