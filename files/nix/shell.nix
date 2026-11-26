{inputs, ...}: {
  perSystem = {
    lib,
    pkgs,
    self',
    ...
  }: {
    devShells.files = pkgs.mkShell {
      inputsFrom = [self'.packages.lys-files];

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
        dbus
      ];

      LD_LIBRARY_PATH = lib.makeLibraryPath self'.packages.lys-files.libs;
      XKB_CONFIG_ROOT = "${pkgs.xkeyboard-config}/share/X11/xkb";
      XLOCALEDIR = "${pkgs.libX11}/share/X11/locale";
      CGO_ENABLED = "0";
      GOTOOLCHAIN = "local";
      LYS_DIST_GO = "${self'.packages.dist-go}/bin/go";
    };
  };
}
