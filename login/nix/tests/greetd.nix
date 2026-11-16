{
  pkgs,
  lib,
  self,
  vm,
}: let
  password = "lys-login-vm";
  journald = ["${pkgs.systemd}/bin/systemd-cat" "--identifier=lys-greeter"];

  session = pkgs.writeShellScript "lys-login-vm-greetd-session" ''
    id -un > /tmp/lys-login/session
    sleep 2
  '';

  screenTest = self.packages.${pkgs.stdenv.hostPlatform.system}.lys-login.overrideAttrs {
    pname = "lys-login-screen-test";
    postFixup = "";
    buildPhase = ''
      runHook preBuild
      go test -c -o screen.test ./login/internal/screen
      runHook postBuild
    '';
    doCheck = false;
    installPhase = ''
      runHook preInstall
      install -Dm755 screen.test $out/bin/screen.test
      runHook postInstall
    '';
  };

  helpers = ''
    def logs_of(machine):
        return machine.succeed("scenario-journal -u greetd -t lys-greeter -o cat --no-pager")

    def no_xkb_errors(machine):
        logged = machine.succeed("scenario-journal -t lys-greeter -o cat --no-pager | grep 'XKB-' || true")
        if logged:
            raise Exception(f"the greeter on {machine.name} lacks keyboard or locale data:\n{logged}")

    def login_through(machine, scenario, user):
        machine.wait_for_unit("greetd.service")
        await_ready(machine, "username", 1)
        machine.screenshot(f"{scenario}-01-greeter")

        machine.send_chars(f"{user}\nwrong-password\n")
        machine.wait_until_succeeds("scenario-journal -t lys-greeter -o cat --no-pager | grep -q 'greeter: authentication failed'", timeout=30)
        machine.screenshot(f"{scenario}-02-wrong-password")
        machine.fail("test -e /tmp/lys-login/session")

        machine.send_chars("${password}\n")
        machine.wait_until_succeeds("test -s /tmp/lys-login/session", timeout=60)
        seen = machine.succeed("cat /tmp/lys-login/session").strip()
        if seen != user:
            raise Exception(f"the session ran as {seen}:\n" + logs_of(machine))

        await_ready(machine, "password", 1)
        machine.screenshot(f"{scenario}-03-greeter-again")
  '';

  user = name: {
    users.users.${name} = {
      isNormalUser = true;
      inherit password;
    };
  };

  wrapper = specialisation: renderer: env: {
    specialisation.${specialisation}.configuration.programs.lys-login.greeter = {
      renderer = lib.mkForce renderer;
      graphicsWrapper = lib.mkForce (journald ++ ["env"] ++ env);
    };
  };

  scenarios = [
    {
      name = "pixman";
      module = user "pia";
      script = _: ''
        login_through(machine, "pixman", "pia")
        no_xkb_errors(machine)
      '';
    }
    {
      name = "hidden";
      module = user "hugo";
      script = _: ''
        machine.wait_for_unit("greetd.service")
        await_ready(machine, "username", 1)
        logged = machine.succeed("scenario-journal -t lys-greeter -o cat --no-pager")
        if "not listing spare:" not in logged:
            raise Exception(f"the system account was not left out:\n{logged}")
        if "not listing hugo:" in logged:
            raise Exception(f"the regular account was left out:\n{logged}")
        machine.screenshot("hidden-01-list")
        machine.send_chars("spare\nspare-vm\n")
        machine.wait_until_succeeds("test -s /tmp/lys-login/session", timeout=30)
        seen = machine.succeed("cat /tmp/lys-login/session").strip()
        if seen != "spare":
            raise Exception(f"typing the hidden account's name ran the session as {seen}")
      '';
    }
    {
      name = "gles";
      specialisation = "gles";
      module.imports = [(user "gail") (wrapper "gles" "auto" ["WLR_RENDERER_ALLOW_SOFTWARE=1"])];
      script = _: ''
        login_through(machine, "gles", "gail")
        no_xkb_errors(machine)
      '';
    }
    {
      name = "deadkey";
      specialisation = "deadkey";
      module.imports = [(user "dora") (wrapper "deadkey" "software" ["XKB_DEFAULT_LAYOUT=us" "XKB_DEFAULT_VARIANT=intl"])];
      script = _: ''
        machine.wait_for_unit("greetd.service")
        machine.succeed("printf 'dora:caf\\303\\251\\n' | chpasswd")
        await_ready(machine, "username", 1)
        machine.send_chars("dora\ncaf'e\n")
        machine.wait_until_succeeds("test -s /tmp/lys-login/session", timeout=30)
        no_xkb_errors(machine)
      '';
    }
    {
      name = "wallclock";
      module = {};
      script = _: ''
        machine.succeed(
            "cd /tmp && LYS_LOGIN_WALLCLOCK_VM=1 ${screenTest}/bin/screen.test "
            + "-test.v -test.count=1 -test.run '^TestSettingTheClockWakesTheTimer$' >&2"
        )
      '';
    }
  ];
in
  vm.suite {
    name = "lys-login-vms-greetd";
    displayManager = "greetd.service";
    journal = "-t lys-greeter";
    node = {
      imports = [self.nixosModules.lys-login vm.module vm.systemAccount] ++ map (s: s.module) scenarios;
      services.timesyncd.enable = false;
      programs.lys-login.greeter = {
        enable = true;
        displayManager = "greetd";
        renderer = "software";
        graphicsWrapper = journald;
        session = {
          dirs = [];
          x11Dirs = [];
          command = "${session}";
        };
      };
    };
    scenarios = map (s: s // {script = nodes: helpers + s.script nodes;}) scenarios;
    globalTimeout = 900;
  }
