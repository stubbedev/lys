{
  lib,
  lysGo,
}:
lysGo.build {
  pname = "lys-power";

  subPackages = ["power/cmd/lys-power"];

  resources = ["power/dist/lys-power.service" "power/dist/lys-power-brightness.service" "power/dist/lys-power-attention.service" "power/dist/dev.stubbe.lys.Attention1.service" "power/dist/lys-power.tmpfiles" "power/dist/dev.stubbe.lys.Energy1.conf" "power/dist/dev.stubbe.lys.Pause1.conf" "power/dist/dev.stubbe.lys.Charge1.conf" "power/dist/dev.stubbe.lys.power.policy" "power/dist/dev.stubbe.lys.Nap1.conf" "power/dist/dev.stubbe.lys.Sleep1.conf"];

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
    substitute power/dist/lys-power-attention.service $out/lib/systemd/user/lys-power-attention.service \
      --replace-fail /usr/bin/lys-power "$out/bin/lys-power"
    install -d $out/share/dbus-1/services
    substitute power/dist/dev.stubbe.lys.Attention1.service $out/share/dbus-1/services/dev.stubbe.lys.Attention1.service \
      --replace-fail /usr/bin/lys-power "$out/bin/lys-power"
    install -m644 power/dist/lys-power.tmpfiles $out/lib/tmpfiles.d/lys-power.conf
    install -Dm644 power/dist/dev.stubbe.lys.Energy1.conf $out/share/dbus-1/system.d/dev.stubbe.lys.Energy1.conf
    install -Dm644 power/dist/dev.stubbe.lys.Pause1.conf $out/share/dbus-1/system.d/dev.stubbe.lys.Pause1.conf
    install -Dm644 power/dist/dev.stubbe.lys.Charge1.conf $out/share/dbus-1/system.d/dev.stubbe.lys.Charge1.conf
    install -Dm644 power/dist/dev.stubbe.lys.power.policy $out/share/polkit-1/actions/dev.stubbe.lys.power.policy
    install -Dm644 power/dist/dev.stubbe.lys.Nap1.conf $out/share/dbus-1/system.d/dev.stubbe.lys.Nap1.conf
    install -Dm644 power/dist/dev.stubbe.lys.Sleep1.conf $out/share/dbus-1/system.d/dev.stubbe.lys.Sleep1.conf
  '';

  doInstallCheck = true;
  installCheckPhase = ''
    runHook preInstallCheck
    $out/bin/lys-power version | grep -qxF "lys-power $version"
    grep -qxF "ExecStart=$out/bin/lys-power run" $out/lib/systemd/system/lys-power.service
    grep -qxF "ExecStart=$out/bin/lys-power brightness" $out/lib/systemd/user/lys-power-brightness.service
    grep -qxF "ExecStart=$out/bin/lys-power attention" $out/lib/systemd/user/lys-power-attention.service
    grep -qxF "Exec=$out/bin/lys-power attention" $out/share/dbus-1/services/dev.stubbe.lys.Attention1.service
    grep -qxF "SystemdService=lys-power-attention.service" $out/share/dbus-1/services/dev.stubbe.lys.Attention1.service
    grep -qxF "d /run/battery-charge 0775 root users -" $out/lib/tmpfiles.d/lys-power.conf
    grep -qF '<allow own="dev.stubbe.lys.Energy1"/>' $out/share/dbus-1/system.d/dev.stubbe.lys.Energy1.conf
    grep -qF '<allow own="dev.stubbe.lys.Pause1"/>' $out/share/dbus-1/system.d/dev.stubbe.lys.Pause1.conf
    grep -qF 'dev.stubbe.lys.power.pause-system-units' $out/share/polkit-1/actions/dev.stubbe.lys.power.policy
    grep -qF '<allow own="dev.stubbe.lys.Charge1"/>' $out/share/dbus-1/system.d/dev.stubbe.lys.Charge1.conf
    grep -qF 'dev.stubbe.lys.power.charge' $out/share/polkit-1/actions/dev.stubbe.lys.power.policy
    grep -qF '<allow own="dev.stubbe.lys.Nap1"/>' $out/share/dbus-1/system.d/dev.stubbe.lys.Nap1.conf
    grep -qF '<allow own="dev.stubbe.lys.Sleep1"/>' $out/share/dbus-1/system.d/dev.stubbe.lys.Sleep1.conf
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
