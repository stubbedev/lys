{
  perSystem = {
    lib,
    lysGo,
    pkgs,
    ...
  }: let
    # What the binary dlopens through purego: libpam for authentication
    # and sessions, and what drawing loads through
    # lys-ui (libvulkan, libva).
    libs = [pkgs.pam pkgs.libva pkgs.vulkan-loader];
  in {
    packages.lys-login = lysGo.build {
      pname = "lys-login";

      subPackages = ["login/cmd/lys-login"];

      resources = ["login/resources/dev.stubbe.lys.login.policy"];

      ldflags = [
        "-s"
        "-w"
        "-X github.com/stubbedev/lys/lib/xkb.defaultRoot=${pkgs.xkeyboard-config}/share/X11/xkb"
        "-X github.com/stubbedev/lys/lib/xkb.defaultXLocaleDir=${pkgs.libX11}/share/X11/locale"
      ];

      # The suite runs in `just check`: it needs a compositor and D-Bus.
      doCheck = false;

      nativeBuildInputs = [pkgs.makeWrapper];

      postInstall = ''
        install -Dm0644 login/resources/dev.stubbe.lys.login.policy $out/share/polkit-1/actions/dev.stubbe.lys.login.policy
      '';

      postFixup = ''
        wrapProgram $out/bin/lys-login --prefix LD_LIBRARY_PATH : "${lib.makeLibraryPath libs}"
      '';

      passthru = {inherit libs;};

      meta = {
        description = "the lys desktop's display manager, greeter and lock screen";
        homepage = "https://github.com/stubbedev/lys/tree/main/login";
        license = lib.licenses.mit;
        mainProgram = "lys-login";
        platforms = lib.platforms.linux;
      };
    };
  };
}
