{
  perSystem = {
    lysGo,
    pkgs,
    ...
  }: {
    packages.lys-polkit = pkgs.callPackage ./lys-polkit.nix {inherit lysGo;};
  };
}
