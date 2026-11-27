{
  perSystem =
    {
      lysGo,
      pkgs,
      self',
      ...
    }:
    {
      packages.lys-shell = pkgs.callPackage ./lys-shell.nix {
        inherit lysGo;
        inherit (self'.packages) lys-docthumb;
      };
    };
}
