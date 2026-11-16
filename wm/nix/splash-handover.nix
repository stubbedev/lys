pkgs: let
  helper = import ./c-program.nix pkgs {
    name = "splash-handover";
    source = ./splash-handover.c;
    libraries = [pkgs.libdrm pkgs.systemd];
    modules = ["libdrm" "libsystemd"];
  };
in {
  name = "splash-handover";
  outputs = ["Virtual-1" "Virtual-2"];
  kernelModules.vkms = "";
  drmDebug = 16;
  machineConfig = {
    environment.systemPackages = [helper];
    systemd.services.splash-handover = {
      requiredBy = ["greetd.service"];
      before = ["greetd.service"];
      after = ["systemd-modules-load.service"];
      serviceConfig = {
        Type = "notify";
        RemainAfterExit = true;
        ExecStartPre = "${pkgs.systemd}/bin/udevadm settle --timeout=60";
        ExecStart = "${helper}/bin/splash-handover preferred";
      };
    };
  };
  scripts = [./splash-handover.py];
}
