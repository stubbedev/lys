{
  inputs,
  self,
  ...
}: {
  perSystem = {
    pkgs,
    self',
    system,
    ...
  }: let
    module = self.nixosModules.lys-wm;
    vms = args: import ./session-vm.nix ({inherit pkgs module;} // args);
    desktop = group: scenarios:
      vms {
        name = "lys-wm-vms-${group}";
        memorySize = 4096;
        inherit scenarios;
      };
    unfree = import inputs.nixpkgs {
      inherit system;
      config.allowUnfreePredicate = p: builtins.elem (pkgs.lib.getName p) ["intellij-idea" "steam" "steam-unwrapped"];
    };
    xwaylandApp = app:
      import ./session-vm.nix {
        inherit module;
        pkgs = unfree;
        name = "lys-wm-vms-xwayland-${app}";
        memorySize = 6144;
        cores = 4;
        diskSize = 8192;
        timeout = 1800;
        scenarios = [
          (import ./xwayland-apps.nix {
            inherit app;
            pkgs = unfree;
          })
        ];
      };
  in {
    legacyPackages.vm-steam = xwaylandApp "steam";
    legacyPackages.vm-xwayland-idea = xwaylandApp "idea";
    legacyPackages.vm-pace-drm = import ./pace-drm.nix {
      inherit pkgs module;
      inherit (self'.packages) lys-wm;
    };
    checks = {
      vms-desktop = desktop "desktop" [
        {name = "drm";}
        {
          name = "hdr-refused";
          lysWMConfig = ''
            outputs:
              - match: "*"
                hdr: true
          '';
          scripts = [
            (pkgs.writeText "hdr-refused.py" ''
              machine.succeed("journalctl -b --no-pager -t lys-wm | grep 'output Virtual-1 stays SDR'")
            '')
          ];
        }
        {
          name = "rtkit";
          machineConfig = {lib, ...}: {
            security.pam.loginLimits = lib.mkForce [];
            security.rtkit.enable = true;
          };
          scripts = [./rtkit.py];
        }
        (import ./autostart.nix pkgs)
        (import ./upgrade.nix pkgs)
      ];
      vms-apps = desktop "apps" [
        (import ./apps.nix pkgs)
      ];
      vms-share = desktop "share" [
        (import ./xdnd.nix pkgs)
        (import ./screencast-scenario.nix {
          inherit pkgs;
          portal = "wlr";
          seconds = 10;
        })
      ];
      vms-gpus = vms {
        name = "lys-wm-vms-gpus";
        devices = ["virtio-gpu-pci,id=gpu0" "virtio-gpu-pci,id=gpu1"];
        scenarios = [
          {
            name = "configured-gpu";
            lysWMConfig = "gpu: card1";
            rendersOn = "card1";
            outputs = ["Virtual-1" "Virtual-2"];
          }
          (import ./xwayland.nix pkgs)
          {
            name = "two-gpus";
            outputs = ["Virtual-1" "Virtual-2"];
            unplug = {
              card = "card1";
              outputs = ["Virtual-1"];
            };
            scripts = [
              (pkgs.writeText "two-gpus.py" ''
                machine.succeed("journalctl -b --no-pager -t lys-wm | grep 'Virtual-2 in .* scans out no linear buffer of the primary GPU, so its frames take a CPU copy'")
              '')
            ];
          }
          (import ./lid.nix pkgs)
        ];
      };
      vms-vkms = vms {
        name = "lys-wm-vms-vkms";
        devices = ["virtio-gpu-pci" "virtio-mouse-pci"];
        kernelParams = ["log_buf_len=64M"];
        scenarios = [
          (import ./splash-handover.nix pkgs)
          {
            name = "display-only";
            outputs = ["Virtual-1" "Virtual-2"];
            kernelModules.vkms = "";
            scripts = [
              (pkgs.writeText "display-only.py" ''
                machine.succeed("journalctl -b --no-pager -t lys-wm | grep 'reach the screen through the primary GPU.s linear buffers'")
                machine.fail("journalctl -b --no-pager -t lys-wm | grep 'scans out no linear buffer'")
              '')
            ];
          }
          (import ./cursor.nix pkgs)
        ];
      };
      vms-go-tests = import ./go-test-vms.nix {
        inherit pkgs;
        inherit (self'.packages) lys-wm;
      };
      module = let
        machine = inputs.nixpkgs.lib.nixosSystem {
          modules = [
            module
            {
              nixpkgs.hostPlatform = system;
              programs.lys-wm.enable = true;
              services.greetd.enable = true;
              boot.loader.grub.enable = false;
              fileSystems."/" = {
                device = "nodev";
                fsType = "tmpfs";
              };
              system.stateVersion = "25.11";
            }
          ];
        };
        sessions = machine.config.services.displayManager.sessionData.desktops;
      in
        assert builtins.elem self'.packages.lys-wm machine.config.xdg.portal.configPackages;
        assert machine.config.programs.foot.enable;
        assert builtins.all (a: a.assertion) machine.config.assertions;
        assert !machine.config.xdg.portal.enable;
        assert !machine.config.xdg.portal.wlr.enable;
        assert !builtins.elem pkgs.xdg-desktop-portal-gtk machine.config.xdg.portal.extraPortals;
        assert builtins.elem "lys-wm" machine.config.services.displayManager.sessionData.sessionNames;
          pkgs.runCommand "lys-wm-module" {} ''
            grep -q "^Exec=${self'.packages.lys-wm}/bin/lys-wm --systemd$" ${sessions}/share/wayland-sessions/lys-wm.desktop
            touch $out
          '';
    };
  };
}
