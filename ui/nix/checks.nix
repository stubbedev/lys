{
  perSystem = {
    lib,
    lysGo,
    pkgs,
    ...
  }: {
    checks.demos = lysGo.build {
      pname = "lys-ui-demos";
      subPackages = ["ui/cmd/..."];
      ldflags = [
        "-X github.com/stubbedev/lys/lib/xkb.defaultRoot=${pkgs.xkeyboard-config}/share/X11/xkb"
      ];
      doCheck = false;
      nativeBuildInputs = [pkgs.makeWrapper];
      postFixup = ''
        for bin in "$out"/bin/*; do
          wrapProgram "$bin" --prefix LD_LIBRARY_PATH : ${lib.makeLibraryPath [pkgs.vulkan-loader]}
        done
      '';

      meta = {
        description = "lys-ui's demos and the headless suite's clients, for development";
        homepage = "https://github.com/stubbedev/lys/tree/main/ui";
        license = lib.licenses.mit;
        platforms = lib.platforms.linux;
      };
    };
  };
}
