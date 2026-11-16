{
  perSystem = {
    buildGoModule,
    lib,
    pkgs,
    self',
    ...
  }: let
    withPinnedGo = pkg:
      pkg.override (lib.mapAttrs (_: _: buildGoModule)
        (lib.filterAttrs (name: _: lib.match "buildGo.*Module" name != null)
          (lib.functionArgs pkg.override)));
  in {
    devShells.default = pkgs.mkShell {
      inputsFrom = map (c: self'.devShells.${c}) ["lib" "ui" "desktop" "wm" "shell" "login" "settings" "polkit" "power" "secrets" "splash"];
      packages = map withPinnedGo [
        pkgs.gopls
        pkgs.delve
        pkgs.gotools
        pkgs.gomodifytags
        pkgs.impl
        pkgs.go-tools
        pkgs.gotests
        pkgs.iferr
      ];
      env = import ../wm/nix/test-env.nix pkgs;
    };
    formatter = pkgs.alejandra;
  };
}
