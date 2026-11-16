{
  pkgs,
  lib,
  ...
}: let
  hashed = password:
    toString (pkgs.runCommand "lys-login-vm-keyboard-password" {nativeBuildInputs = [pkgs.mkpasswd];} ''
      mkpasswd -m sha-512 -S lyskeyboardvmslt ${lib.escapeShellArg password} > $out
    '');

  session = pkgs.writeShellScript "lys-login-vm-keyboard-session" ''
    id -un >> /tmp/lys-login/sessions
    sleep 3
  '';

  helpers = ''
    def journal(machine):
        return machine.succeed("scenario-journal -t lys-login -o cat --no-pager")

    def await_layout(machine, name, count):
        machine.wait_until_succeeds(
            f"[ \"$(scenario-journal -t lys-login -o cat --no-pager | grep -c 'greeter: keyboard layout \"{name}\"')\" -ge {count} ]",
            timeout=60,
        )

    def sessions(machine):
        return machine.succeed("cat /tmp/lys-login/sessions 2>/dev/null || true").split()

    def await_sessions(machine, count):
        machine.wait_until_succeeds(f"[ \"$(wc -l < /tmp/lys-login/sessions)\" -ge {count} ]", timeout=60)

    def system_layout_only(machine, layout):
        machine.wait_for_unit("lys-login.service")
        machine.wait_until_succeeds("pgrep -u lys-greeter -f 'lys-login greet'", timeout=90)
        localed = machine.succeed("localectl status")
        if f"X11 Layout: {layout}" not in localed:
            raise Exception(f"localed does not report {layout}:\n{localed}")
        kiosk = machine.succeed("cat /run/lys-login/kiosk.yaml")
        if f'layout: "{layout}"' not in kiosk:
            raise Exception(f"the kiosk config lacks the layout {layout}:\n{kiosk}")
        for proc in ("lys-login daemon", "lys-login greet"):
            pid = machine.succeed(f"pgrep -f '{proc}' | head -n 1").strip()
            env = machine.succeed(f"tr '\\0' '\\n' < /proc/{pid}/environ")
            if "XKB_DEFAULT" in env:
                raise Exception(f"{proc} runs with XKB_DEFAULT_*:\n{env}")
  '';

  scenario = {
    name,
    user,
    password,
    xkb,
    script,
  }: {
    inherit name;
    specialisation = name;
    sessions.${user} = session;
    module = {
      users.users.${user} = {
        isNormalUser = true;
        hashedPasswordFile = hashed password;
      };
      specialisation.${name}.configuration.services.xserver.xkb = xkb;
    };
    script = _: helpers + script;
  };
in [
  (scenario {
    name = "keyboard_german";
    user = "greta";
    password = "zürichy";
    xkb.layout = "de";
    script = ''
      system_layout_only(machine, "de")
      await_ready(machine, "username", 1)
      await_layout(machine, "German", 1)
      machine.screenshot("keyboard-german-01-greeter")
      machine.send_chars("greta\ny[richz\n")
      await_sessions(machine, 1)
      if sessions(machine) != ["greta"]:
          raise Exception(f"the German password started {sessions(machine)}:\n" + journal(machine))
    '';
  })
  (scenario {
    name = "keyboard_switching";
    user = "sven";
    password = "zy-pass";
    xkb = {
      layout = "de,us";
      options = "grp:caps_toggle";
    };
    script = ''
      system_layout_only(machine, "de,us")
      await_ready(machine, "username", 1)
      await_layout(machine, "German", 1)
      machine.screenshot("keyboard-switching-01-greeter-german")
      machine.send_key("caps_lock")
      await_layout(machine, "English (US)", 1)
      machine.screenshot("keyboard-switching-02-greeter-us")
      machine.send_chars("sven\nzy-pass\n")
      await_sessions(machine, 1)
      remembered = machine.succeed("cat /var/lib/lys-greeter/last-layout/sven").strip()
      if remembered != "English (US)":
          raise Exception(f"the greeter remembered {remembered!r} for sven")

      await_ready(machine, "password", 1)
      await_layout(machine, "English (US)", 2)
      machine.screenshot("keyboard-switching-03-greeter-remembered-us")
      machine.send_chars("zy-pass\n")
      await_sessions(machine, 2)
      if sessions(machine) != ["sven", "sven"]:
          raise Exception(f"the logins started {sessions(machine)}:\n" + journal(machine))
      print(journal(machine)[-4000:])
    '';
  })
]
