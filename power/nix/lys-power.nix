{
  lib,
  lysGo,
}:
lysGo.build {
  pname = "lys-power";

  subPackages = ["power/cmd/lys-power"];

  resources = ["power/dist/lys-power.service" "power/dist/lys-power-brightness.service" "power/dist/lys-power.tmpfiles" "power/dist/dev.stubbe.lys.Energy1.conf" "power/dist/dev.stubbe.lys.Pause1.conf" "power/dist/dev.stubbe.lys.power.policy"];

  ldflags = [
    "-s"
  ];

  doCheck = false;

  postInstall = ''
    install -d $out/lib/systemd/system $out/lib/systemd/user $out/lib/tmpfiles.d
    substitute power/dist/lys-power.service $out/lib/systemd/system/lys-power.service \
      --replace-fail /usr/bin/lys-power "$out/bin/lys-power"
    substitute power/dist/lys-power-brightness.service $out/lib/systemd/user/lys-power-brightness.service \
      --replace-fail /usr/bin/lys-power "$out/bin/lys-power"
    install -m644 power/dist/lys-power.tmpfiles $out/lib/tmpfiles.d/lys-power.conf
    install -Dm644 power/dist/dev.stubbe.lys.Energy1.conf $out/share/dbus-1/system.d/dev.stubbe.lys.Energy1.conf
    install -Dm644 power/dist/dev.stubbe.lys.Pause1.conf $out/share/dbus-1/system.d/dev.stubbe.lys.Pause1.conf
    install -Dm644 power/dist/dev.stubbe.lys.power.policy $out/share/polkit-1/actions/dev.stubbe.lys.power.policy
  '';

  doInstallCheck = true;
  installCheckPhase = ''
    runHook preInstallCheck
    $out/bin/lys-power version | grep -qxF "lys-power $version"
    grep -qxF "ExecStart=$out/bin/lys-power run" $out/lib/systemd/system/lys-power.service
    grep -qxF "ExecStart=$out/bin/lys-power brightness" $out/lib/systemd/user/lys-power-brightness.service
    grep -qxF "d /run/battery-charge 0775 root users -" $out/lib/tmpfiles.d/lys-power.conf
    grep -qF '<allow own="dev.stubbe.lys.Energy1"/>' $out/share/dbus-1/system.d/dev.stubbe.lys.Energy1.conf
    grep -qF '<allow own="dev.stubbe.lys.Pause1"/>' $out/share/dbus-1/system.d/dev.stubbe.lys.Pause1.conf
    grep -qF 'dev.stubbe.lys.power.pause-system-units' $out/share/polkit-1/actions/dev.stubbe.lys.power.policy
    runHook postInstallCheck
  '';

  meta = {
    description = "The lys desktop's adaptive battery charging and power profile policy";
    homepage = "https://github.com/stubbedev/lys/tree/main/power";
    license = lib.licenses.mit;
    mainProgram = "lys-power";
    platforms = lib.platforms.linux;
  };
}
