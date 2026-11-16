{
  perSystem = {lysGo, ...}: {
    packages.drm-tests = lysGo.build {
      pname = "lys-lib-drm-tests";
      goTests = ["lib/drm"];
      buildPhase = ''
        runHook preBuild
        go test -c -o drm.test ./lib/drm
        runHook postBuild
      '';
      doCheck = false;
      installPhase = ''
        runHook preInstall
        install -Dm755 drm.test $out/bin/drm.test
        runHook postInstall
      '';
    };
  };
}
