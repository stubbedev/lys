# lys-login as the display manager, showing its greeter in a lys-wm kiosk,
# taking the screen from lys-splash and handing it to alice's lys-wm session,
# which shows swaybg's colour. greeter-gate holds lys-login until
# the test creates /run/greeter-go, then marks the kernel log, so a test can
# use the splash first and read the DRM log from the hand-over on.
{
  pkgs,
  self,
  wmPackage,
  drmTools,
}: {config, ...}: let
  sessionConfig = pkgs.writeText "lys-splash-vm-session.yaml" ''
    autostart:
      - ${pkgs.swaybg}/bin/swaybg -c '#3050a0'
    animations:
      enabled: true
      events:
        fade_in:
          curve: linear
          duration: 1500
  '';
in {
  imports = [self.nixosModules.lys-login self.nixosModules.lys-wm];
  boot.kernelParams = ["drm.debug=0x10" "log_buf_len=64M"];
  programs.lys-wm = {
    enable = true;
    package = wmPackage;
  };
  programs.lys-login.greeter = {
    enable = true;
    displayManager = "lys-login";
    compositorPackage = config.programs.lys-wm.package;
    settings.login = {
      show-clock = false;
      background-mode = "color";
      background-color = "#2a4a3a";
    };
    settings.animations.lock.enter-duration = 2000;
    session = {
      dirs = [];
      x11Dirs = [];
      command = "${config.programs.lys-wm.package}/bin/lys-wm --config ${sessionConfig}";
    };
  };
  systemd.services.greeter-gate = {
    wantedBy = ["lys-login.service"];
    before = ["lys-login.service"];
    serviceConfig = {
      Type = "oneshot";
      TimeoutStartSec = "infinity";
    };
    script = ''
      until test -e /run/greeter-go; do sleep 0.1; done
      echo 'greeter-gate: the greeter may start' > /dev/kmsg
    '';
  };
  users.users.alice = {
    isNormalUser = true;
    password = "lys-splash-vm";
  };
  system.switch.enable = true;
  virtualisation.cores = 4;
  virtualisation.memorySize = 2048;
  environment.sessionVariables = {
    LIBGL_ALWAYS_SOFTWARE = "1";
    LYS_WM_RENDERER = "lavapipe";
  };
  environment.systemPackages = [drmTools];
  hardware.graphics.enable = true;
  hardware.graphics.extraPackages = [pkgs.mesa];
  fonts.packages = [pkgs.inter];
}
