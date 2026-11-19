{
  perSystem = {lysGo, ...}: {
    packages.network-vm-tests = lysGo.build {
      pname = "lys-desktop-network-vm-tests";
      goTests = ["desktop/service/network"];
      buildPhase = ''
        runHook preBuild
        go test -c -tags nmvm -o network.test ./desktop/service/network
        runHook postBuild
      '';
      doCheck = false;
      installPhase = ''
        runHook preInstall
        install -Dm755 network.test $out/bin/network.test
        runHook postInstall
      '';
    };
  };
}
