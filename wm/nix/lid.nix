pkgs: let
  lidFifo = "/run/lid-switch/lid";
  suspendTag = "lid-suspend";
  primaryConnector = "/sys/class/drm/card0-Virtual-1";
  python = pkgs.python3.withPackages (ps: [ps.evdev]);
in {
  name = "lid";
  outputs = ["Virtual-2"];
  kernelModules = {
    test_power = "usb_online=off";
    uinput = "";
  };
  machineConfig.systemd.services = {
    lid-switch = {
      wantedBy = ["multi-user.target"];
      before = ["greetd.service"];
      serviceConfig = {
        RuntimeDirectory = "lid-switch";
        ExecStart = "${python}/bin/python3 ${./lid-switch.py} ${lidFifo}";
      };
    };
    primary-output-off = {
      requiredBy = ["greetd.service"];
      before = ["greetd.service"];
      serviceConfig.Type = "oneshot";
      script = ''
        udevadm wait --settle --timeout=60 ${primaryConnector}
        echo off > ${primaryConnector}/status
      '';
    };
    systemd-suspend.serviceConfig.ExecStart = ["" "${pkgs.util-linux}/bin/logger -t ${suspendTag} suspend"];
  };
  params = {
    lid_fifo = lidFifo;
    suspend_tag = suspendTag;
    external_card = "card1";
  };
  scripts = [./lid.py];
}
