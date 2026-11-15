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
    nixos =
      (inputs.nixpkgs.lib.nixosSystem {
        inherit (pkgs.stdenv.hostPlatform) system;
        modules = [
          self.nixosModules.lys-power
          {
            services.lys-power.enable = true;
            boot.loader.grub.enable = false;
            fileSystems."/" = {
              device = "none";
              fsType = "tmpfs";
            };
            system.stateVersion = "25.11";
          }
        ];
      }).config;
    wants = nixos.systemd.services.lys-power.wantedBy;
    fail = cond: message: lib.optionalString cond "echo ${lib.escapeShellArg message} >&2; exit 1";
  in {
    checks.lys-power-nixos-module = pkgs.runCommand "lys-power-nixos-module" {} ''
      unit=${pkg}/lib/systemd/system/lys-power.service
      grep -qxF "ExecStart=${lib.getExe pkg} run" $unit
      grep -qxF "After=power-profiles-daemon.service upower.service" $unit
      grep -qxF "StateDirectory=lys-power" $unit
      grep -qxF "Restart=on-failure" $unit
      grep -qxF "RestartSec=10s" $unit
      grep -qxF "ProtectSystem=strict" $unit
      grep -qxF "ReadWritePaths=/sys/class/power_supply -/run/battery-charge" $unit
      grep -qxF "d /run/battery-charge 0775 root users -" ${pkg}/lib/tmpfiles.d/lys-power.conf
      grep -qxF "WantedBy=graphical.target" $unit
      ${fail (wants != ["graphical.target"]) "lys-power is wanted by ${toString wants}, not graphical.target alone"}
      ${fail (!lib.elem pkg nixos.systemd.packages) "lys-power is not in systemd.packages"}
      ${fail (!lib.elem pkg nixos.systemd.tmpfiles.packages) "lys-power is not in systemd.tmpfiles.packages"}
      ${fail (!lib.elem pkg nixos.environment.systemPackages) "lys-power is not in environment.systemPackages"}
      ${fail (!nixos.services.upower.enable) "upower is not enabled"}
      ${fail (!nixos.services.power-profiles-daemon.enable) "power-profiles-daemon is not enabled"}
      touch $out
    '';
  };
}
