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
    pname = "lys-viewer";

    subPackages = ["viewer/cmd/lys-viewer" "desktop/cmd/lys-docthumb"];

    goPackages = ["desktop/cmd/install-icons"];

    resources = ["viewer/resources/dev.stubbe.lys.viewer.desktop" "viewer/resources/lys-viewer.svg" "viewer/resources/icons.txt"];

    ldflags = [
      "-s"
      "-w"
      "-X github.com/stubbedev/lys/lib/xkb.defaultRoot=${xkeyboard-config}/share/X11/xkb"
    ];

    doCheck = false;

    nativeBuildInputs = [makeWrapper];

    postInstall = ''
      install -Dm0644 viewer/resources/dev.stubbe.lys.viewer.desktop -t $out/share/applications
      install -Dm0644 viewer/resources/lys-viewer.svg -t $out/share/icons/hicolor/scalable/apps
      go tool install-icons $out/share/lys-viewer/icons/hicolor/scalable/actions $(cat viewer/resources/icons.txt)
    '';

    postFixup = ''
      wrapProgram $out/bin/lys-viewer --prefix LD_LIBRARY_PATH : "${lib.makeLibraryPath libs}"
    '';

    doInstallCheck = true;
    nativeInstallCheckInputs = [desktop-file-utils];
    installCheckPhase = ''
      runHook preInstallCheck
      export HOME=$TMPDIR
      $out/bin/lys-viewer --version | grep -qxF "lys-viewer $version"
      status=0
      $out/bin/lys-viewer > /dev/null 2>&1 || status=$?
      if [ "$status" -ne 2 ]; then
        echo "lys-viewer without a file exited $status, want 2" >&2
        exit 1
      fi
      desktop-file-validate $out/share/applications/dev.stubbe.lys.viewer.desktop
      for icon in $(cat viewer/resources/icons.txt); do
        test -f $out/share/lys-viewer/icons/hicolor/scalable/actions/$icon.svg
      done
      runHook postInstallCheck
    '';

    passthru = {inherit libs;};

    meta = {
      description = "The lys desktop's viewer for pictures, videos and documents";
      homepage = "https://github.com/stubbedev/lys/tree/main/viewer";
      license = lib.licenses.mit;
      mainProgram = "lys-viewer";
      platforms = lib.platforms.linux;
    };
  }
