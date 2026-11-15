{
  perSystem =
    { lysGo, pkgs, ... }:
    {
      packages.lys-shell = pkgs.callPackage ./lys-shell.nix { inherit lysGo; };
    };
}
