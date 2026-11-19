{
  perSystem = {lysGo, ...}: {
    packages.network-vm-tests = lysGo.build {
      pname = "lys-desktop-network-vm-tests";
      goTests = ["desktop/service/network" "desktop/service/rfkill"];
      buildPhase = ''
        runHook preBuild
        go test -c -tags nmvm -o network.test ./desktop/service/network
        go test -c -tags nmvm -o rfkill.test ./desktop/service/rfkill
        runHook postBuild
      '';
      doCheck = false;
      installPhase = ''
        runHook preInstall
        install -Dm755 network.test $out/bin/network.test
        install -Dm755 rfkill.test $out/bin/rfkill.test
        runHook postInstall
      '';
    };
  };
}
