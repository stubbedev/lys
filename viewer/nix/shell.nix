{inputs, ...}: {
  perSystem = {
    lib,
    pkgs,
    self',
    ...
  }: {
    devShells.viewer = pkgs.mkShell {
      inputsFrom = [self'.packages.lys-viewer];

      packages = with pkgs; [
        gopls
        golangci-lint
        gofumpt
        delve
        just
        git
        gh
        "${inputs.lys-ci}"
        desktop-file-utils
      ];

      LD_LIBRARY_PATH = lib.makeLibraryPath self'.packages.lys-viewer.libs;
      XKB_CONFIG_ROOT = "${pkgs.xkeyboard-config}/share/X11/xkb";
      XLOCALEDIR = "${pkgs.libX11}/share/X11/locale";
      CGO_ENABLED = "0";
      GOTOOLCHAIN = "local";
      LYS_DIST_GO = "${self'.packages.dist-go}/bin/go";
    };
  };
}
