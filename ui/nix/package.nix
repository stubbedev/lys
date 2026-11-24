{
  perSystem = {
    lib,
    lysGo,
    pkgs,
    ...
  }: {
    packages.lys-ui = lysGo.build {
      pname = "lys-ui";
      subPackages = ["ui/..."];
      ldflags = [
        "-X github.com/stubbedev/lys/lib/xkb.defaultRoot=${pkgs.xkeyboard-config}/share/X11/xkb"
      ];
      doCheck = false;
      # The GPU renderer loads the Vulkan loader at run time (no
      # cgo, so no rpath): the binaries find it through the
      # wrapper's library path, NixOS having none of its own.
      nativeBuildInputs = [pkgs.makeWrapper];
      postFixup = ''
        for bin in "$out"/bin/*; do
          wrapProgram "$bin" --prefix LD_LIBRARY_PATH : ${lib.makeLibraryPath [pkgs.vulkan-loader]}
        done
      '';

      meta = {
        description = "pure-Go Wayland widget toolkit (module and demos)";
        homepage = "https://github.com/stubbedev/lys/tree/main/ui";
        license = lib.licenses.mit;
        platforms = lib.platforms.linux;
      };
    };
  };
}
