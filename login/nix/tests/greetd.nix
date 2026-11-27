{
  pkgs,
  self,
  vm,
  ...
}: let
  lysWm = self.packages.${pkgs.stdenv.hostPlatform.system}.lys-wm;
  password = "lys-login-vm";
  journald = ["${pkgs.systemd}/bin/systemd-cat" "--identifier=lys-greeter"];

  session = pkgs.writeShellScript "lys-login-vm-greetd-session" ''
    id -un > /tmp/lys-login/session
    sleep 2
  '';

  wallclockTest = self.packages.${pkgs.stdenv.hostPlatform.system}.lys-login.overrideAttrs {
    pname = "lys-login-wallclock-test";
    goTests = ["lib/wallclock"];
    postInstall = "";
    postFixup = "";
    buildPhase = ''
      runHook preBuild
      go test -c -o wallclock.test ./lib/wallclock
      runHook postBuild
    '';
    doCheck = false;
    installPhase = ''
      runHook preInstall
      install -Dm755 wallclock.test $out/bin/wallclock.test
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

    def greeter_never_failed(machine):
        failed = machine.succeed("scenario-journal -u greetd -o cat --no-pager | grep 'greeter exited without creating a session' || true")
        if failed:
            raise Exception(f"greetd started its greeter again on {machine.name}:\n" + logs_of(machine))

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

  scenarios = [
    {
      name = "lavapipe";
      module = user "pia";
      script = _: ''
        login_through(machine, "lavapipe", "pia")
        no_xkb_errors(machine)
        greeter_never_failed(machine)
        kiosk = machine.succeed("for p in $(pgrep -u greeter); do printf '%s %s\\n' $p $(readlink /proc/$p/exe); done").splitlines()
        lys_wm = [fields[0] for fields in map(str.split, kiosk) if len(fields) == 2 and fields[1].startswith("${lysWm}/")]
        if not lys_wm:
            raise Exception(f"the greeter does not run in lys-wm: {kiosk}")
        env = machine.succeed(f"tr '\\0' '\\n' < /proc/{lys_wm[0]}/environ").split()
        if "LYS_WM_RENDERER=lavapipe" not in env:
            raise Exception(f"greetd's lys-wm does not render on lavapipe: {env}")
      '';
    }
    {
      name = "hidden";
      module = user "hugo";
      script = _: ''
        machine.wait_for_unit("greetd.service")
        await_ready(machine, "username", 1)
        logged = machine.succeed("scenario-journal -t lys-greeter -o cat --no-pager")
        lists = [line.rsplit("login users:", 1)[1].split() for line in logged.splitlines() if "greeter: listing " in line and "login users:" in line]
        if not lists:
            raise Exception(f"the greeter did not say whom it lists:\n{logged}")
        if "spare" in lists[-1]:
            raise Exception(f"the account above login.defs' UID_MAX was listed: {lists[-1]}")
        if "hugo" not in lists[-1]:
            raise Exception(f"the regular account was left out: {lists[-1]}")
        greeter_never_failed(machine)
        machine.screenshot("hidden-01-list")
        machine.send_chars("spare\nspare-vm\n")
        machine.wait_until_succeeds("test -s /tmp/lys-login/session", timeout=30)
        seen = machine.succeed("cat /tmp/lys-login/session").strip()
        if seen != "spare":
            raise Exception(f"typing the hidden account's name ran the session as {seen}")
      '';
    }
    {
      name = "deadkey";
      specialisation = "deadkey";
      module.imports = [
        (user "dora")
        {
          specialisation.deadkey.configuration.services.xserver.xkb = {
            layout = "us";
            variant = "intl";
          };
        }
      ];
      script = _: ''
        machine.wait_for_unit("greetd.service")
        machine.succeed("printf 'dora:caf\\303\\251\\n' | chpasswd")
        await_ready(machine, "username", 1)
        machine.send_chars("dora\ncaf'e\n")
        machine.wait_until_succeeds("test -s /tmp/lys-login/session", timeout=30)
        no_xkb_errors(machine)
        greeter_never_failed(machine)
      '';
    }
    {
      name = "wallclock";
      module = {};
      script = _: ''
        machine.succeed(
            "cd /tmp && LYS_WALLCLOCK_VM=1 ${wallclockTest}/bin/wallclock.test "
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
      imports = [self.nixosModules.lys-login vm.module vm.systemAccount ./late-udmabuf-grant.nix] ++ map (s: s.module) scenarios;
      users.users.spare.uid = 30500;
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
