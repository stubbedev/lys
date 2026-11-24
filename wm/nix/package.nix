{
  perSystem = {
    lib,
    lysGo,
    pkgs,
    self',
    ...
  }: {
    packages = {
      lys-wm = lysGo.build {
        pname = "lys-wm";

        subPackages = ["wm/cmd/lys-wm" "wm/cmd/lys-wmctl"];

        resources = [
          "wm/packaging/systemd/lys-wm-session.target"
          "wm/packaging/xdg-desktop-portal/lys-wm-portals.conf"
          "wm/packaging/wayland-sessions/lys-wm.desktop"
        ];

        # purego loads these at runtime; the store paths are baked in so the
        # binary needs no library search path.
        ldflags = with pkgs; [
          "-X github.com/stubbedev/lys/wm/internal/gpu.defaultGBM=${libgbm}/lib/libgbm.so.1"
          "-X github.com/stubbedev/lys/lib/xkb.defaultRoot=${xkeyboard-config}/share/X11/xkb"
          "-X github.com/stubbedev/lys/lib/third_party/vk.defaultLibrary=${vulkan-loader}/lib/libvulkan.so.1"
          "-X github.com/stubbedev/lys/wm/internal/session.defaultLibinput=${libinput.out}/lib/libinput.so.10"
          "-X github.com/stubbedev/lys/wm/internal/xwayland.defaultXwayland=${xwayland}/bin/Xwayland"
        ];

        postInstall = ''
          install -Dm644 wm/packaging/systemd/lys-wm-session.target $out/share/systemd/user/lys-wm-session.target
          install -Dm644 wm/packaging/xdg-desktop-portal/lys-wm-portals.conf $out/share/xdg-desktop-portal/lys-wm-portals.conf
          install -Dm644 wm/packaging/wayland-sessions/lys-wm.desktop $out/share/wayland-sessions/lys-wm.desktop
          substituteInPlace $out/share/wayland-sessions/lys-wm.desktop --replace-fail "Exec=lys-wm" "Exec=$out/bin/lys-wm"
        '';

        doCheck = false;

        passthru.providedSessions = ["lys-wm"];

        meta = {
          description = "A pure-Go tiling Wayland compositor";
          homepage = "https://github.com/stubbedev/lys/tree/main/wm";
          license = lib.licenses.mit;
          mainProgram = "lys-wm";
          platforms = lib.platforms.unix;
        };
      };
    };

    apps = {
      lys-wm = {
        type = "app";
        program = lib.getExe self'.packages.lys-wm;
      };
    };
  };
}
