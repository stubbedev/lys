self: {
  config,
  lib,
  pkgs,
  ...
}: let
  cfg = config.services.lys-power;
in {
  options.services.lys-power = {
    enable = lib.mkEnableOption "lys-power, adaptive battery charging and power profile switching";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.lys-power;
      defaultText = lib.literalExpression "lys.packages.\${system}.lys-power";
      description = "The lys-power package, whose system unit, user unit and tmpfiles rule are installed.";
    };

    autoBrightness.enable = lib.mkEnableOption "automatic screen brightness, the lys-power-brightness user service, in every graphical session";

    devicePower = {
      policy = lib.mkOption {
        type = lib.types.enum ["battery" "always" "off"];
        default = "battery";
        description = ''
          When idle devices power down: `battery` on battery only, with
          everything back at full performance on AC; `always` keeps PCI
          runtime power management and USB autosuspend on AC as well;
          `off` leaves every device alone.
        '';
      };

      deny = lib.mkOption {
        type = lib.types.listOf (lib.types.strMatching "(pci|usb):[0-9a-fA-F]{4}:[0-9a-fA-F]{4}");
        default = [];
        example = ["usb:046d:c52b" "pci:8086:a840"];
        description = "Devices the device power policy never touches, as pci:vendor:device or usb:vendor:product.";
      };
    };

    usbWake = lib.mkOption {
      type = lib.types.enum ["keep" "battery"];
      default = "keep";
      description = ''
        Whether removable USB devices may wake the machine: `keep` leaves
        every device's wakeup alone; `battery` disables it on removable USB
        devices on battery, so a mouse bumped in a bag does not wake a
        closed laptop, and enables it again on AC.
      '';
    };
  };

  config = lib.mkMerge [
    (lib.mkIf cfg.enable {
      services.upower.enable = lib.mkDefault true;
      services.power-profiles-daemon.enable = lib.mkDefault true;

      environment.systemPackages = [cfg.package];
      systemd.packages = [cfg.package];
      systemd.tmpfiles.packages = [cfg.package];
      services.dbus.packages = [cfg.package];
      security.polkit.enable = lib.mkDefault true;
      systemd.services.lys-power = {
        wantedBy = ["graphical.target"];
        environment = {
          LYS_POWER_DEVICE_POLICY = cfg.devicePower.policy;
          LYS_POWER_DEVICE_DENY = lib.concatStringsSep " " cfg.devicePower.deny;
          LYS_POWER_USB_WAKE = cfg.usbWake;
        };
      };
    })
    (lib.mkIf cfg.autoBrightness.enable {
      environment.systemPackages = [cfg.package];
      systemd.packages = [cfg.package];
      systemd.user.services.lys-power-brightness.wantedBy = ["graphical-session.target"];
    })
  ];
}
