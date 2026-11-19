{self, ...}: {
  perSystem = {
    lib,
    pkgs,
    ...
  }: let
    packages = self.packages.${pkgs.stdenv.hostPlatform.system};
    indent = script: lib.concatMapStringsSep "\n" (line: lib.optionalString (line != "") "        ${line}") (lib.splitString "\n" script);
    scenarios = name: list: let
      names = map (scenario: scenario.name) list;
    in
      assert lib.assertMsg (lib.allUnique names) "${name}: scenario names repeat: ${toString names}";
        pkgs.testers.runNixOSTest {
          inherit name;
          nodes.machine.imports = map (scenario: scenario.module) list;
          testScript = ''
            import traceback

            failed = []
            ${lib.concatMapStrings (scenario: let
                quoted = builtins.toJSON scenario.name;
              in ''
                try:
                    with subtest(${quoted}):
                ${indent scenario.script}
                except Exception:
                    traceback.print_exc()
                    failed.append(${quoted})
              '')
              list}
            assert not failed, f"these scenarios failed: {', '.join(failed)}"
          '';
        };
    x86 = lib.optionals pkgs.stdenv.hostPlatform.isx86_64;
    secondOutput = "/sys/class/drm/card1-Virtual-2";
    screencast = import ../wm/nix/screencast-scenario.nix {
      inherit pkgs;
      lysShell = self.nixosModules.lys-shell;
      portal = "lys-shell";
      seconds = 10;
    };
  in {
    checks =
      {
        lys-desktop-vms-system = scenarios "lys-desktop-vms-system" (
          [
            (import ../lib/nix/kms-vm.nix packages.drm-tests)
            (import ../settings/nix/accounts-vm.nix)
            (import ../desktop/nix/network-vm.nix packages.network-vm-tests)
            (import ../secrets/nix/tests/activation-vm.nix self)
          ]
          ++ x86 [
            (import ../polkit/tests/polkit-vm.nix {
              inherit pkgs;
              inherit (packages) lys-polkit;
            })
            (import ../shell/tests/clockwatch-vm.nix {inherit pkgs lib self;})
          ]
        );
      }
      // lib.optionalAttrs pkgs.stdenv.hostPlatform.isx86_64 {
        lys-desktop-vms-session = import ../wm/nix/session-vm.nix {
          inherit pkgs;
          module = self.nixosModules.lys-wm;
          name = "lys-desktop-vms-session";
          devices = ["virtio-gpu-pci" "virtio-gpu-pci"];
          memorySize = 4096;
          cores = 4;
          scenarios = [
            (import ../shell/tests/lys-wm-vm.nix {inherit pkgs self;})
            (screencast
              // {
                machineConfig = {
                  imports = [screencast.machineConfig];
                  systemd.services.second-output-off = {
                    requiredBy = ["greetd.service"];
                    before = ["greetd.service"];
                    serviceConfig.Type = "oneshot";
                    script = ''
                      udevadm wait --settle --timeout=60 ${secondOutput}
                      echo off > ${secondOutput}/status
                    '';
                  };
                };
              })
          ];
        };
      };
  };
}
