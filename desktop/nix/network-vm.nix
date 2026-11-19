networkTests: {
  name = "the network client joins, refuses and forgets a simulated Wi-Fi network, and airplane mode blocks it";
  module = {lib, ...}: {
    networking.wireless.enable = lib.mkOverride 5 true;
    boot.kernelModules = ["mac80211_hwsim"];
    boot.extraModprobeConfig = "options mac80211_hwsim radios=2";
    services.hostapd = {
      enable = true;
      radios.wlan0 = {
        band = "2g";
        channel = 1;
        networks.wlan0 = {
          ssid = "lys-vm";
          authentication = {
            mode = "wpa2-sha1";
            wpaPassword = "correct horse battery";
          };
        };
      };
    };
    networking.interfaces.wlan0.ipv4.addresses = [
      {
        address = "192.168.77.1";
        prefixLength = 24;
      }
    ];
    networking.firewall.trustedInterfaces = ["wlan0"];
    services.dnsmasq = {
      enable = true;
      resolveLocalQueries = false;
      settings = {
        interface = "wlan0";
        bind-interfaces = true;
        dhcp-range = "192.168.77.10,192.168.77.50,12h";
      };
    };
    networking.networkmanager = {
      enable = true;
      unmanaged = ["wlan0"];
    };
  };
  script = ''
    machine.wait_for_unit("hostapd.service")
    machine.wait_for_unit("dnsmasq.service")
    machine.wait_for_unit("NetworkManager.service")
    machine.wait_until_succeeds("nmcli -t -f DEVICE,STATE device | grep -q '^wlan1:disconnected'")
    out = machine.succeed("${networkTests}/bin/network.test -test.v -test.count=1 -test.run '^TestVM' 2>&1")
    print(out)
    for test in ["TestVMConnectsWithThePasswordAndForgets", "TestVMSaysAWrongPasswordWasRefused"]:
        assert f"--- PASS: {test}" in out, f"{test} did not pass in the VM"
    out = machine.succeed("${networkTests}/bin/rfkill.test -test.v -test.count=1 -test.run '^TestVM' 2>&1")
    print(out)
    assert "--- PASS: TestVMAirplaneModeBlocksTheRadiosAndLetsThemGo" in out, "airplane mode did not pass in the VM"
    machine.succeed("systemctl stop NetworkManager.service")
    out = machine.succeed("LYS_NM_STOPPED=1 ${networkTests}/bin/network.test -test.v -test.count=1 -test.run '^TestVMWithoutNetworkManager$' 2>&1")
    print(out)
    assert "--- PASS: TestVMWithoutNetworkManager" in out, "the missing NetworkManager case did not run"
  '';
}
