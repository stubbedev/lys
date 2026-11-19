{
  perSystem = {
    lysGo,
    pkgs,
    ...
  }: {
    packages = {
      desktop-vm-tests = lysGo.build {
        pname = "lys-desktop-vm-tests";
        goTests = ["desktop/service/network" "desktop/service/rfkill" "desktop/service/bluetooth"];
        buildPhase = ''
          runHook preBuild
          for pkg in network rfkill bluetooth; do
            go test -c -tags vm -o "$pkg.test" "./desktop/service/$pkg"
          done
          runHook postBuild
        '';
        doCheck = false;
        installPhase = ''
          runHook preInstall
          for pkg in network rfkill bluetooth; do
            install -Dm755 "$pkg.test" "$out/bin/$pkg.test"
          done
          runHook postInstall
        '';
      };
      btvirt = pkgs.bluez.overrideAttrs (old: {
        pname = "btvirt";
        configureFlags = old.configureFlags ++ ["--enable-testing"];
        postInstall =
          (old.postInstall or "")
          + ''
            install -Dm755 emulator/btvirt $out/bin/btvirt
          '';
      });
    };
  };
}
