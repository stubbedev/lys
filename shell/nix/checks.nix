{ self, inputs, ... }:
{
  perSystem =
    {
      pkgs,
      lib,
      lysGo,
      ...
    }:
    let
      pkg = self.packages.${pkgs.stdenv.hostPlatform.system}.lys-shell;
    in
    {
      checks =
        lib.optionalAttrs pkgs.stdenv.hostPlatform.isx86_64 {
          lys-shell-hyprland-vms = pkgs.callPackage ../tests/hyprland-vm.nix {
            inherit self lysGo;
          };
        }
        // {
          # Every wrapped binary finds the Vulkan loader: lys-ui renders on
          # the GPU through it and silently stays on the CPU without it, and
          # NixOS has no global library path to fall back on.
          # The modules install the settings app unless told not to.
          lys-shell-settings-app =
            let
              settingsPkg = self.packages.${pkgs.stdenv.hostPlatform.system}.lys-settings;
              nixos =
                enable:
                (inputs.nixpkgs.lib.nixosSystem {
                  modules = [
                    self.nixosModules.lys-shell
                    {
                      nixpkgs.hostPlatform = pkgs.stdenv.hostPlatform.system;
                      system.stateVersion = "26.05";
                      programs.lys-shell.enable = true;
                      programs.lys-shell.settingsApp.enable = enable;
                    }
                  ];
                }).config.environment.systemPackages;
              home =
                enable:
                (lib.evalModules {
                  modules = [
                    (import ./hm-module.nix self)
                    {
                      config._module.args = { inherit pkgs; };
                      options = {
                        assertions = lib.mkOption { type = lib.types.anything; };
                        home.packages = lib.mkOption { type = lib.types.listOf lib.types.package; };
                        systemd = lib.mkOption { type = lib.types.anything; };
                        xdg = lib.mkOption { type = lib.types.anything; };
                      };
                      config.programs.lys-shell.enable = true;
                      config.programs.lys-shell.settingsApp.enable = enable;
                    }
                  ];
                }).config.home.packages;
              has = packages: lib.elem settingsPkg packages;
            in
            pkgs.runCommand "lys-shell-settings-app" { } ''
              ${lib.optionalString (!has (nixos true)) "echo 'NixOS: enabled, and not installed' >&2; exit 1"}
              ${lib.optionalString (has (nixos false)) "echo 'NixOS: disabled, and installed' >&2; exit 1"}
              ${lib.optionalString (!has (home true)) "echo 'home-manager: enabled, and not installed' >&2; exit 1"}
              ${lib.optionalString (has (home false)) "echo 'home-manager: disabled, and installed' >&2; exit 1"}
              touch $out
            '';
          lys-shell-wrapper-vulkan = pkgs.runCommand "lys-shell-wrapper-vulkan" { } ''
            for bin in ${lib.concatMapStringsSep " " (b: "${pkg}/bin/${b}") pkg.wrapped}; do
              grep -q "${lib.getLib pkgs.vulkan-loader}/lib" "$bin" || { echo "$bin lacks the Vulkan loader on its library path" >&2; exit 1; }
            done
            touch $out
          '';
        };
    };
}
