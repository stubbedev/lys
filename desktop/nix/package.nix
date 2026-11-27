{
  perSystem = {
    lysGo,
    pkgs,
    ...
  }: {
    packages.lys-docthumb = pkgs.callPackage ./lys-docthumb.nix {inherit lysGo;};
  };
}
