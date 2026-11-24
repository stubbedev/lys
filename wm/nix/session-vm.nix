{
  pkgs,
  module,
  name,
  scenarios,
  devices ? ["virtio-gpu-pci"],
  kernelParams ? [],
  memorySize ? 2048,
  cores ? 2,
  diskSize ? 1024,
  timeout ? 900,
}: let
  inherit (pkgs) lib;
  user = "alice";
  password = user;
  uid = 1000;
  scenario = s:
    {
      outputs = ["Virtual-1"];
      kernelModules = {};
      drmDebug = 0;
      home = {};
      lysWMConfig = null;
      lysWMArgs = "";
      rendersOn = "card0";
      framedCursors = [];
      backgroundOnly = true;
      machineConfig = {};
      params = {};
      scripts = [];
      unplug = null;
    }
    // s;
  all = map scenario scenarios;
  names = map (s: s.name) all;
  home = s:
    pkgs.linkFarm "lys-wm-${s.name}-home" (lib.mapAttrsToList (path: text: {
        name = path;
        path = pkgs.writeText (baseNameOf path) text;
      })
      (s.home // lib.optionalAttrs (s.lysWMConfig != null) {".config/lys-wm/config.yaml" = s.lysWMConfig;}));
  spec = s: {
    inherit (s) name outputs kernelModules unplug params;
    renders_on = s.rendersOn;
    framed_cursors = s.framedCursors;
    background_only = s.backgroundOnly;
    drm_debug = s.drmDebug;
    sole_compositor = s.lysWMArgs == "";
    home = "${home s}";
    scripts = map (script: "${script}") s.scripts;
  };
  session = config: args: {
    command = "${pkgs.systemd}/bin/systemd-cat -t lys-wm ${config.programs.lys-wm.package}/bin/lys-wm --systemd --renderer lavapipe ${args}";
    inherit user;
  };
in
  assert lib.assertMsg (lib.allUnique names) "${name}: scenario names repeat: ${toString names}";
    import ./vm-test.nix pkgs {
      inherit name timeout;

      nodes.machine = {
        imports = [module];

        boot.kernelParams = kernelParams;
        virtualisation = {
          graphics = true;
          inherit cores memorySize diskSize;
          qemu.options = ["-vga none"] ++ map (d: "-device ${d}") devices;
        };

        hardware.graphics.enable = true;
        hardware.wirelessRegulatoryDatabase = true;
        programs.lys-wm.enable = true;
        users.users.${user} = {
          isNormalUser = true;
          inherit uid password;
        };
        environment.systemPackages = [pkgs.grim pkgs.jq];

        specialisation = lib.listToAttrs (map (s:
          lib.nameValuePair s.name {
            configuration = {config, ...}: {
              imports = [s.machineConfig];
              services.greetd = {
                enable = true;
                settings = {
                  initial_session = session config s.lysWMArgs;
                  default_session = session config s.lysWMArgs;
                };
              };
            };
          })
        all);
      };

      testScript = {nodes, ...}: ''
        import json

        USER = "${user}"
        PASSWORD = "${password}"
        RUNTIME = "/run/user/${toString uid}"
        BASE = "${nodes.machine.system.build.toplevel}"
        SCENARIOS = json.loads(${builtins.toJSON (builtins.toJSON (map spec all))})
        ${builtins.readFile ./session-vm.py}
      '';
    }
