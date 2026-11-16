drmTests: {
  name = "the drm package's KMS tests as root on a vkms-only card";
  module = {
    boot.kernelModules = ["vkms" "udmabuf"];
    virtualisation.graphics = false;
    virtualisation.qemu.options = ["-vga none"];
  };
  script = ''
    machine.wait_for_unit("multi-user.target")
    machine.succeed("udevadm settle", "test \"$(readlink -f /sys/class/drm/card0/device)\" = /sys/devices/faux/vkms", "test ! -e /sys/class/drm/card1")
    out = machine.succeed("${drmTests}/bin/drm.test -test.v -test.count=1 -test.skip TestTimelinePointsSignalEventFDs 2>&1")
    print(out)
    assert "--- SKIP" not in out, "a KMS test skipped on the vkms card"
    print(machine.succeed("${drmTests}/bin/drm.test -test.run '^$' -test.bench '^BenchmarkSyncFileSignalTime' -test.benchmem -test.benchtime=200000x 2>&1"))
  '';
}
