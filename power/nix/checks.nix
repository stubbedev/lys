{
  self,
  inputs,
  ...
}: {
  perSystem = {
    lib,
    pkgs,
    ...
  }: let
    pkg = self.packages.${pkgs.stdenv.hostPlatform.system}.lys-power;
    eval = settings:
      (inputs.nixpkgs.lib.nixosSystem {
        inherit (pkgs.stdenv.hostPlatform) system;
        modules = [
          self.nixosModules.lys-power
          {
            services.lys-power = settings;
            boot.loader.grub.enable = false;
            fileSystems."/" = {
              device = "none";
              fsType = "tmpfs";
            };
            system.stateVersion = "25.11";
          }
        ];
      }).config;
    nixos = eval {enable = true;};
    bright = eval {
      enable = true;
      autoBrightness.enable = true;
    };
    brightOnly = eval {autoBrightness.enable = true;};
    devices = eval {
      enable = true;
      devicePower = {
        policy = "off";
        deny = ["usb:046d:c52b" "pci:8086:a840"];
      };
      usbWake = "battery";
    };
    environment = config: config.systemd.services.lys-power.environment;
    wants = nixos.systemd.services.lys-power.wantedBy;
    userWants = config: config.systemd.user.services.lys-power-brightness.wantedBy or [];
    fail = cond: message: lib.optionalString cond "echo ${lib.escapeShellArg message} >&2; exit 1";
  in {
    checks.lys-power-nixos-module = pkgs.runCommand "lys-power-nixos-module" {} ''
      unit=${pkg}/lib/systemd/system/lys-power.service
      grep -qxF "ExecStart=${lib.getExe pkg} run" $unit
      grep -qxF "After=power-profiles-daemon.service upower.service" $unit
      grep -qxF "Wants=upower.service" $unit
      grep -qxF "StateDirectory=lys-power" $unit
      grep -qxF "Restart=on-failure" $unit
      grep -qxF "RestartSec=10s" $unit
      grep -qxF "ProtectSystem=strict" $unit
      grep -qxF "ReadWritePaths=-/run/battery-charge -/sys/module/pcie_aspm/parameters/policy -/sys/module/snd_hda_intel/parameters/power_save" $unit
      grep -qxF "ReadOnlyPaths=/proc/sys -/proc/sysrq-trigger -/proc/acpi -/proc/fs -/proc/irq -/sys/kernel -/sys/module -/sys/power -/sys/firmware -/sys/fs" $unit
      if grep -qx "ProtectKernelTunables=.*" $unit; then echo "lys-power.service sets ProtectKernelTunables, which makes the charge thresholds read-only" >&2; exit 1; fi
      grep -qxF "d /run/battery-charge 0775 root users -" ${pkg}/lib/tmpfiles.d/lys-power.conf
      grep -qxF "WantedBy=graphical.target" $unit
      ${fail (wants != ["graphical.target"]) "lys-power is wanted by ${toString wants}, not graphical.target alone"}
      ${fail (!lib.elem pkg nixos.systemd.packages) "lys-power is not in systemd.packages"}
      ${fail (!lib.elem pkg nixos.systemd.tmpfiles.packages) "lys-power is not in systemd.tmpfiles.packages"}
      ${fail (!lib.elem pkg nixos.environment.systemPackages) "lys-power is not in environment.systemPackages"}
      ${fail (!lib.elem pkg nixos.services.dbus.packages) "lys-power's system bus policy is not in services.dbus.packages"}
      ${fail (!nixos.services.upower.enable) "upower is not enabled"}
      ${fail (!nixos.services.power-profiles-daemon.enable) "power-profiles-daemon is not enabled"}
      ${fail ((environment nixos).LYS_POWER_DEVICE_POLICY != "battery" || (environment nixos).LYS_POWER_DEVICE_DENY != "") "the device power policy does not default to battery with nothing denied"}
      ${fail ((environment devices).LYS_POWER_DEVICE_POLICY != "off") "devicePower.policy does not reach LYS_POWER_DEVICE_POLICY"}
      ${fail ((environment devices).LYS_POWER_DEVICE_DENY != "usb:046d:c52b pci:8086:a840") "devicePower.deny does not reach LYS_POWER_DEVICE_DENY"}
      ${fail ((environment nixos).LYS_POWER_USB_WAKE != "keep") "USB wake does not default to keep"}
      ${fail ((environment devices).LYS_POWER_USB_WAKE != "battery") "usbWake does not reach LYS_POWER_USB_WAKE"}
      user=${pkg}/lib/systemd/user/lys-power-brightness.service
      grep -qxF "ExecStart=${lib.getExe pkg} brightness" $user
      grep -qxF "PartOf=graphical-session.target" $user
      grep -qxF "Restart=on-failure" $user
      grep -qxF "RestrictAddressFamilies=AF_UNIX AF_NETLINK" $user
      grep -qxF "WantedBy=graphical-session.target" $user
      attention=${pkg}/lib/systemd/user/lys-power-attention.service
      grep -qxF "ExecStart=${lib.getExe pkg} attention" $attention
      grep -qxF "BusName=dev.stubbe.lys.Attention1" $attention
      grep -qxF "RestrictAddressFamilies=AF_UNIX" $attention
      if grep -q "^WantedBy=" $attention; then echo "lys-power-attention.service is installed into a target; only the check may start it" >&2; exit 1; fi
      grep -qxF "SystemdService=lys-power-attention.service" ${pkg}/share/dbus-1/services/dev.stubbe.lys.Attention1.service
      ${fail (userWants nixos != []) "lys-power-brightness is wanted without autoBrightness.enable"}
      ${fail (userWants bright != ["graphical-session.target"]) "with autoBrightness.enable, lys-power-brightness is wanted by ${toString (userWants bright)}, not graphical-session.target alone"}
      ${fail (userWants brightOnly != ["graphical-session.target"]) "autoBrightness.enable alone does not start lys-power-brightness with the graphical session"}
      ${fail (!lib.elem pkg brightOnly.systemd.packages) "autoBrightness.enable alone does not put lys-power in systemd.packages"}
      ${fail (brightOnly.systemd.services.lys-power.wantedBy or [] != []) "autoBrightness.enable alone starts the lys-power system service"}
      touch $out
    '';
  };
}
