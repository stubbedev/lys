{
  lib,
  lysGo,
}:
lysGo.build {
  pname = "lys-polkit";

  subPackages = ["polkit/cmd/lys-polkit"];

  ldflags = [
    "-s"
  ];

  doCheck = false;

  postInstall = ''
    install -Dm644 polkit/dist/lys-polkit.service.in $out/lib/systemd/user/lys-polkit.service
    substituteInPlace $out/lib/systemd/user/lys-polkit.service --replace-fail @bindir@ $out/bin
  '';

  doInstallCheck = true;
  installCheckPhase = ''
    runHook preInstallCheck
    $out/bin/lys-polkit --version | grep -qxF "lys-polkit ${lysGo.version}"
    grep -qxF "ExecStart=$out/bin/lys-polkit" $out/lib/systemd/user/lys-polkit.service
    runHook postInstallCheck
  '';

  meta = {
    description = "The lys desktop's polkit authentication agent";
    homepage = "https://github.com/stubbedev/lys/tree/main/polkit";
    license = lib.licenses.mit;
    mainProgram = "lys-polkit";
    platforms = lib.platforms.linux;
  };
}
