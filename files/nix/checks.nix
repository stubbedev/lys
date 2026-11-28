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
    pkg = self.packages.${pkgs.stdenv.hostPlatform.system}.lys-files;
    eval = settings:
      (inputs.nixpkgs.lib.nixosSystem {
        inherit (pkgs.stdenv.hostPlatform) system;
        modules = [
          self.nixosModules.lys-files
          {
            programs.lys-files = settings;
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
    off = eval {};
    rules = nixos.environment.etc."udev/rules.d".source;
    fail = cond: message: lib.optionalString cond "echo ${lib.escapeShellArg message} >&2; exit 1";
  in {
    checks.lys-files-nixos-module = pkgs.runCommand "lys-files-nixos-module" {nativeBuildInputs = [pkgs.systemd];} ''
      ${fail (!lib.elem pkg nixos.environment.systemPackages) "lys-files is not in environment.systemPackages"}
      ${fail (!lib.elem pkg nixos.services.udev.packages) "lys-files' udev rule is not in services.udev.packages"}
      ${fail (lib.elem pkg off.services.udev.packages) "lys-files' udev rule is installed without programs.lys-files.enable"}
      ${fail (lib.elem pkg off.environment.systemPackages) "lys-files is installed without programs.lys-files.enable"}
      rule=${rules}/69-lys-mtp.rules
      grep -qxF 'IMPORT{program}="${pkg}/lib/udev/lys-mtp-probe %k"' $rule
      test -x ${pkg}/lib/udev/lys-mtp-probe
      udevadm verify --resolve-names=never --no-style $rule
      touch $out
    '';
  };
}
