{
  perSystem = {
    lysGo,
    pkgs,
    self',
    ...
  }: {
    packages.lys-files = pkgs.callPackage ./lys-files.nix {
      inherit lysGo;
      inherit (self'.packages) lys-docthumb;
    };
  };
}
