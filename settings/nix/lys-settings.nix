{
  lib,
  lysGo,
  makeWrapper,
  xkeyboard-config,
  libva,
  vulkan-loader,
}: let
  # What the binary dlopens through purego: lys-ui's GPU renderer and the
  # recorder pages' device probes.
  libs = [libva vulkan-loader];
in
  lysGo.build {
    pname = "lys-settings";

    subPackages = ["settings/cmd/lys-settings" "settings/cmd/lys-users"];

    goPackages = ["desktop/cmd/install-icons"];

    resources = ["settings/resources/dev.stubbe.lys.settings.desktop" "settings/resources/lys-settings.svg" "settings/resources/icons.txt" "settings/resources/dev.stubbe.lys.settings.users.policy" "settings/resources/dev.stubbe.lys.Users1.conf" "settings/resources/dev.stubbe.lys.Users1.service" "settings/resources/lys-users.service"];

    ldflags = [
      "-s"
      "-w"
      "-X github.com/stubbedev/lys/lib/xkb.defaultRoot=${xkeyboard-config}/share/X11/xkb"
    ];

    # The suite runs in `just check`: it needs lys-wm and fonts.
    doCheck = false;

    nativeBuildInputs = [makeWrapper];

    postInstall = ''
      install -Dm0644 settings/resources/dev.stubbe.lys.settings.desktop -t $out/share/applications
      install -Dm0644 settings/resources/lys-settings.svg -t $out/share/icons/hicolor/scalable/apps
      go tool install-icons $out/share/lys-settings/icons/hicolor/scalable/actions $(cat settings/resources/icons.txt)
      install -Dm0755 $out/bin/lys-users -t $out/libexec
      rm $out/bin/lys-users
      install -Dm0644 settings/resources/dev.stubbe.lys.settings.users.policy -t $out/share/polkit-1/actions
      install -Dm0644 settings/resources/dev.stubbe.lys.Users1.conf -t $out/share/dbus-1/system.d
      install -d $out/share/dbus-1/system-services $out/lib/systemd/system
      substitute settings/resources/dev.stubbe.lys.Users1.service $out/share/dbus-1/system-services/dev.stubbe.lys.Users1.service \
        --replace-fail /usr/libexec/lys-users $out/libexec/lys-users
      substitute settings/resources/lys-users.service $out/lib/systemd/system/lys-users.service \
        --replace-fail /usr/libexec/lys-users $out/libexec/lys-users
    '';

    postFixup = ''
      wrapProgram $out/bin/lys-settings --prefix LD_LIBRARY_PATH : "${lib.makeLibraryPath libs}"
    '';

    doInstallCheck = true;
    installCheckPhase = ''
      runHook preInstallCheck
      export HOME=$TMPDIR XDG_CONFIG_HOME=$TMPDIR/config
      $out/bin/lys-settings --version | grep -qxF "lys-settings $version"
      $out/bin/lys-settings --list-pages | grep -qF '"id":"overview"'
      if $out/bin/lys-settings --bogus 2>/dev/null; then
        echo "an unknown flag was accepted" >&2
        exit 1
      fi
      grep -qxF "Exec=$out/libexec/lys-users" $out/share/dbus-1/system-services/dev.stubbe.lys.Users1.service
      grep -qxF "ExecStart=$out/libexec/lys-users" $out/lib/systemd/system/lys-users.service
      status=0
      $out/libexec/lys-users --bogus > /dev/null 2>&1 || status=$?
      if [ "$status" -ne 2 ]; then
        echo "lys-users with an unknown flag exited $status, want 2" >&2
        exit 1
      fi
      runHook postInstallCheck
    '';

    passthru = {inherit libs;};

    meta = {
      description = "The lys desktop's settings app, for lys-shell and lys-wm";
      homepage = "https://github.com/stubbedev/lys/tree/main/settings";
      license = lib.licenses.mit;
      mainProgram = "lys-settings";
      platforms = lib.platforms.linux;
    };
  }
