{
  pkgs,
  lib,
  self,
  vm,
}: let
  scenarios = lib.flatten (map (path: import path {inherit pkgs lib self vm;}) [
    ./scenarios/login.nix
    ./scenarios/handover.nix
    ./scenarios/x11-session.nix
    ./scenarios/accessibility.nix
    ./scenarios/lock.nix
    ./scenarios/switch.nix
    ./scenarios/rekey.nix
    ./scenarios/keyboard.nix
    ./scenarios/keyring.nix
    ./scenarios/upgrade.nix
  ]);
  byName = lib.listToAttrs (map (s: lib.nameValuePair s.name s) scenarios);

  node = {
    imports =
      [
        self.nixosModules.lys-login
        self.nixosModules.lys-wm
        self.nixosModules.lys-secrets
        vm.module
        vm.systemAccount
      ]
      ++ map (s: s.module) scenarios;
    programs.lys-wm.enable = true;
    services.lys-secrets.enable = true;
    programs.lys-login = {
      lock.enable = true;
      greeter = {
        enable = true;
        session = {
          dirs = [];
          x11Dirs = [];
          command = "${vm.dispatch (lib.mergeAttrsList (map (s: s.sessions or {}) scenarios))}";
        };
      };
    };
  };
in
  group: names:
    vm.suite {
      name = "lys-login-vms-${group}";
      displayManager = "lys-login.service";
      journal = "-t lys-login";
      inherit node;
      scenarios = map (name: byName.${name}) names;
      globalTimeout = 900;
    }
