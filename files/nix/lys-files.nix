{
  lib,
  lysGo,
  makeWrapper,
  desktop-file-utils,
  xkeyboard-config,
  libva,
  vulkan-loader,
}: let
  libs = [libva vulkan-loader];
in
  lysGo.build {
    pname = "lys-files";

    subPackages = ["files/cmd/lys-files" "desktop/cmd/lys-docthumb"];

    goPackages = ["desktop/cmd/install-icons"];

    resources = [
      "files/resources/dev.stubbe.lys.files.desktop"
      "files/resources/org.freedesktop.FileManager1.service"
      "files/resources/lys-files.svg"
      "files/resources/icons.txt"
    ];

    ldflags = [
      "-s"
      "-w"
      "-X github.com/stubbedev/lys/lib/xkb.defaultRoot=${xkeyboard-config}/share/X11/xkb"
    ];

    doCheck = false;

    nativeBuildInputs = [makeWrapper];

    postInstall = ''
      install -Dm0644 files/resources/dev.stubbe.lys.files.desktop -t $out/share/applications
      install -Dm0644 files/resources/lys-files.svg -t $out/share/icons/hicolor/scalable/apps
      install -d $out/share/dbus-1/services
      substitute files/resources/org.freedesktop.FileManager1.service \
        $out/share/dbus-1/services/org.freedesktop.FileManager1.service \
        --replace-fail /usr/bin/lys-files "$out/bin/lys-files"
      go tool install-icons $out/share/lys-files/icons/hicolor/scalable/actions $(cat files/resources/icons.txt)
    '';

    postFixup = ''
      wrapProgram $out/bin/lys-files --prefix LD_LIBRARY_PATH : "${lib.makeLibraryPath libs}"
    '';

    doInstallCheck = true;
    nativeInstallCheckInputs = [desktop-file-utils];
    installCheckPhase = ''
      runHook preInstallCheck
      export HOME=$TMPDIR
      $out/bin/lys-files --version | grep -qxF "lys-files $version"
      status=0
      $out/bin/lys-files --bogus > /dev/null 2>&1 || status=$?
      if [ "$status" -ne 2 ]; then
        echo "lys-files --bogus exited $status, want 2" >&2
        exit 1
      fi
      desktop-file-validate $out/share/applications/dev.stubbe.lys.files.desktop
      grep -qxF "Exec=$out/bin/lys-files --service" $out/share/dbus-1/services/org.freedesktop.FileManager1.service
      for icon in $(cat files/resources/icons.txt); do
        test -f $out/share/lys-files/icons/hicolor/scalable/actions/$icon.svg
      done
      runHook postInstallCheck
    '';

    passthru = {inherit libs;};

    meta = {
      description = "The lys desktop's file manager";
      homepage = "https://github.com/stubbedev/lys/tree/main/files";
      license = lib.licenses.mit;
      mainProgram = "lys-files";
      platforms = lib.platforms.linux;
    };
  }
