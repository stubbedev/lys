{
  pkgs,
  self,
  ...
}: let
  user = "alice";
  password = "lys-login-vm";
  defaultCompositor = self.packages.${pkgs.stdenv.hostPlatform.system}.lys-wm;
  cursorTheme = "lys-vm-cursor";
  cursorSize = "48";

  staged = pkgs.writeText "lys-login-vm-staged.yaml" ''
    login:
      background-mode: color
      background-color: "#ff0000"
  '';

  session = pkgs.writeShellScript "lys-login-vm-login-session" ''
    echo "$(id -un) $XDG_SESSION_TYPE $XDG_SESSION_CLASS $XDG_SEAT $XDG_VTNR $XDG_SESSION_ID" > /tmp/lys-login/session
    loginctl show-session "$XDG_SESSION_ID" -p Class -p Type -p Seat -p VTNr -p Name > /tmp/lys-login/logind
    lys-login apply-config ${staged} > /tmp/lys-login/apply 2>&1; echo "exit $?" >> /tmp/lys-login/apply
    XCURSOR_THEME=${cursorTheme} XCURSOR_SIZE=${cursorSize} lys-login record-cursor > /tmp/lys-login/cursor 2>&1; echo "exit $?" >> /tmp/lys-login/cursor
    for _ in $(seq 60); do
      ${pkgs.systemd}/bin/busctl --user get-property org.freedesktop.secrets \
        /org/freedesktop/secrets/collection/login org.freedesktop.Secret.Collection Locked \
        > /tmp/lys-login/keyring.tmp 2>&1 || true
      mv /tmp/lys-login/keyring.tmp /tmp/lys-login/keyring
      [ "$(cat /tmp/lys-login/keyring)" = "b false" ] && break
      sleep 0.5
    done
    sleep 5
  '';
in {
  name = "login";
  sessions.${user} = session;
  module = {
    users.users.${user} = {
      isNormalUser = true;
      inherit password;
    };
  };
  script = _: ''
    import time

    def background():
        shot = Screendump(machine, "background")
        return {shot.pixel(shot.width // 10, shot.height // 2), shot.pixel(0, shot.height // 2)}

    def await_background(want, what, deadline=30.0, step=0.5):
        end = time.monotonic() + deadline
        seen = background()
        while seen != want:
            if time.monotonic() >= end:
                machine.screenshot(f"timeout-{what}")
                raise Exception(f"the {what}'s background is {seen} after {deadline}s, want {want}")
            time.sleep(step)
            seen = background()

    def login_attempts(user):
        return machine.succeed("scenario-journal -t lys-login -o cat --no-pager").count(f"dm: login of {user}:")

    def greeter_up():
        return machine.execute("pgrep -u lys-greeter -f 'lys-login greet'")[0] == 0

    machine.wait_for_unit("lys-login.service")
    machine.wait_until_succeeds("pgrep -u lys-greeter -f 'lys-login greet'", timeout=90)
    for absent in ("greetd", "cage"):
        if machine.execute(f"pgrep -x {absent}")[0] == 0:
            raise Exception(f"{absent} runs")
    machine.wait_until_succeeds("loginctl list-sessions --no-legend | grep -q lys-greeter", timeout=30)
    machine.wait_for_unit("graphical.target")
    for cmd, want in (
        ("systemctl show -p Id --value default.target", "graphical.target"),
        ("systemctl show -p Id --value display-manager.service", "lys-login.service"),
    ):
        got = machine.succeed(cmd).strip()
        if got != want:
            raise Exception(f"{cmd}: {got}, want {want}")
    kiosk = machine.succeed("for p in $(pgrep -u lys-greeter); do readlink /proc/$p/exe; done").split()
    if not any(exe.startswith("${defaultCompositor}/") for exe in kiosk):
        raise Exception(f"the greeter's kiosk is not the flake's lys-wm ${defaultCompositor}: {kiosk}")
    await_ready(machine, "username", 1)
    await_background({(0, 0, 0)}, "first greeter")
    machine.screenshot("login-01-greeter")

    machine.fail("systemctl cat accounts-daemon.service")
    listed = machine.succeed("scenario-journal -t lys-login -o cat --no-pager")
    if "not listing spare:" not in listed:
        raise Exception(f"the system account was not left out:\n{listed}")
    if "not listing ${user}:" in listed:
        raise Exception(f"the regular account was left out:\n{listed}")

    if machine.execute("su ${user} -s /bin/sh -c 'lys-login apply-config ${staged}'")[0] == 0:
        raise Exception("a user without a session changed the login screen")
    machine.fail("test -e /etc/lys-shell/runtime.yaml")
    machine.fail("test -e /etc/polkit-1/actions/dev.stubbe.lys.greeter.policy")

    if machine.execute("su ${user} -s /bin/sh -c 'XCURSOR_THEME=x lys-login record-cursor'")[0] == 0:
        raise Exception("a user without a session recorded a cursor")
    uid = machine.succeed("id -u ${user}").strip()
    machine.fail(f"test -e /var/lib/lys-login/cursor/{uid}")
    if machine.succeed("stat -c %a /home/${user}").strip() != "700":
        raise Exception("the user's home is not the default 0700")
    machine.fail("grep -q XCURSOR_THEME /run/lys-login/kiosk.yaml")

    machine.send_chars("${user}\nwrong-password\n")
    machine.wait_until_succeeds("scenario-journal -t lys-login -o cat --no-pager | grep -q 'greeter: authentication failed'", timeout=30)
    machine.screenshot("login-02-wrong-password")
    if login_attempts("${user}") != 1:
        raise Exception("the first login was not of ${user}:\n" + machine.succeed("scenario-journal -t lys-login -o cat --no-pager | grep 'dm: login of'"))
    if machine.execute("test -e /tmp/lys-login/session")[0] == 0:
        raise Exception("a wrong password started a session")
    if not greeter_up():
        raise Exception("a wrong password took the greeter down")

    machine.send_chars("${password}\n")
    machine.wait_until_succeeds("test -s /tmp/lys-login/logind", timeout=60)
    seen = machine.succeed("cat /tmp/lys-login/session").split()
    if seen[:5] != ["${user}", "wayland", "user", "seat0", "1"]:
        raise Exception(f"the session ran as {seen}")
    logind = machine.succeed("cat /tmp/lys-login/logind")
    for want in ("Class=user", "Type=wayland", "Seat=seat0", "VTNr=1", "Name=${user}"):
        if want not in logind:
            raise Exception(f"logind's session lacks {want}:\n{logind}")
    machine.screenshot("login-03-session")

    machine.wait_until_succeeds("grep -qx 'b false' /tmp/lys-login/keyring", timeout=40)
    if machine.execute("test -d /home/${user}/.local/share/lys-secrets")[0] != 0:
        raise Exception("lys-secrets keeps no collection for ${user}")

    apply = machine.succeed("cat /tmp/lys-login/apply")
    if "wrote /etc/lys-shell/runtime.yaml" not in apply or "exit 0" not in apply:
        raise Exception(f"apply-config from the session: {apply}")
    machine.succeed("grep -q 'ff0000' /etc/lys-shell/runtime.yaml")

    cursor = machine.succeed("cat /tmp/lys-login/cursor")
    if f"wrote /var/lib/lys-login/cursor/{uid}" not in cursor or "exit 0" not in cursor:
        raise Exception(f"record-cursor from the session: {cursor}")
    machine.succeed(f"su lys-greeter -s /bin/sh -c 'grep -q ${cursorTheme} /var/lib/lys-login/cursor/{uid}'")

    machine.wait_until_succeeds("pgrep -u lys-greeter -f 'lys-login greet'", timeout=30)
    await_ready(machine, "password", 1)
    await_background({(255, 0, 0)}, "next greeter")
    machine.screenshot("login-04-greeter-again")

    greeter_env = machine.succeed("tr '\\0' '\\n' < /proc/$(pgrep -u lys-greeter -f 'lys-login greet' | head -n 1)/environ")
    for want in ("XCURSOR_THEME=${cursorTheme}", "XCURSOR_SIZE=${cursorSize}"):
        if want not in greeter_env.splitlines():
            raise Exception(f"the next greeter runs without {want}:\n{greeter_env}")
    journal = machine.succeed("scenario-journal -u lys-login --no-pager")
    if "dm: greeter: the greeter ended" in journal:
        raise Exception("a greeter died and was restarted:\n" + journal[-4000:])
    if "dm: greeter connection:" in journal:
        raise Exception("a greeter connection failed:\n" + journal[-4000:])
    print(journal[-4000:])
  '';
}
