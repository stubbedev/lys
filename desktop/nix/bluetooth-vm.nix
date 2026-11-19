{
  vmTests,
  btvirt,
}: {
  name = "the bluetooth client finds no adapter, then powers one and pairs a second";
  module = {
    boot.kernelModules = ["hci_vhci"];
    hardware.bluetooth.enable = true;
    systemd.services.btvirt = {
      description = "Two emulated Bluetooth controllers";
      serviceConfig.ExecStart = "${btvirt}/bin/btvirt -l2";
    };
  };
  script = ''
    machine.succeed("systemctl start bluetooth.service")
    machine.wait_for_unit("bluetooth.service")
    out = machine.succeed("LYS_BT_NO_ADAPTER=1 ${vmTests}/bin/bluetooth.test -test.v -test.count=1 -test.run '^TestVMWithoutAnAdapter$' 2>&1")
    print(out)
    assert "--- PASS: TestVMWithoutAnAdapter" in out, "the missing adapter case did not run"
    machine.succeed("systemctl start btvirt.service")
    machine.wait_until_succeeds("test $(bluetoothctl list | wc -l) -eq 2")
    out = machine.succeed("${vmTests}/bin/bluetooth.test -test.v -test.count=1 -test.run '^TestVMPowersAnAdapterFindsAndPairsTheOther$' 2>&1")
    print(out)
    assert "--- PASS: TestVMPowersAnAdapterFindsAndPairsTheOther" in out, "the adapter and pairing test did not pass"
  '';
}
