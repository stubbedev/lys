{inputs, ...}: {
  perSystem = {
    lib,
    pkgs,
    self',
    ...
  }: {
    devShells.settings = pkgs.mkShell {
      inputsFrom = [self'.packages.lys-settings];

      packages = with pkgs; [
        gopls
        golangci-lint
        gofumpt
        delve
        just
        git
        gh
        "${inputs.lys-ci}"
        fontconfig
        inter
        jetbrains-mono
        wtype
        dbus
      ];

      LD_LIBRARY_PATH = lib.makeLibraryPath (self'.packages.lys-settings.libs ++ [pkgs.pam]);
      LYS_SETTINGS_TEST_PWQUALITY = "${pkgs.libpwquality.lib}/lib/security/pam_pwquality.so";
      XKB_CONFIG_ROOT = "${pkgs.xkeyboard-config}/share/X11/xkb";
      XLOCALEDIR = "${pkgs.libX11}/share/X11/locale";
      FONTCONFIG_FILE = pkgs.makeFontsConf {
        fontDirectories = with pkgs; [inter jetbrains-mono dejavu_fonts];
      };

      CGO_ENABLED = "0";
      GOTOOLCHAIN = "local";
    };
  };
}
