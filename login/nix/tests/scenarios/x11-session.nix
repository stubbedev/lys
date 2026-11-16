{
  pkgs,
  lib,
  ...
}: let
  user = "ada";
  password = "correct horse";
  xsessions = pkgs.writeTextDir "share/xsessions/stand-in.desktop" ''
    [Desktop Entry]
    Name=Stand-in
    Exec=stand-in-session
  '';
  startx = pkgs.writeShellScriptBin "startx" ''
    echo "$XDG_SESSION_TYPE $*" > /tmp/lys-login/session
    loginctl show-session "$XDG_SESSION_ID" -p Type > /tmp/lys-login/logind
    sleep 5
  '';
in {
  name = "x11_session";
  specialisation = "x11-session";
  module = {
    environment.systemPackages = [startx];
    users.users.${user} = {
      isNormalUser = true;
      inherit password;
    };
    specialisation.x11-session.configuration.programs.lys-login.greeter.session = {
      command = lib.mkForce "";
      x11Dirs = lib.mkForce ["${xsessions}/share/xsessions"];
    };
  };
  script = _: ''
    machine.wait_for_unit("lys-login.service")
    machine.wait_until_succeeds("pgrep -u lys-greeter -f 'lys-login greet'", timeout=90)
    machine.wait_until_succeeds("loginctl list-sessions --no-legend | grep -q lys-greeter", timeout=30)
    sessions = machine.succeed("loginctl list-sessions --no-legend | awk '/lys-greeter/ {print $1}'").split()
    greeter = [machine.succeed(f"loginctl show-session {s} -p Class -p Type") for s in sessions]
    if not any("Class=greeter" in g and "Type=wayland" in g for g in greeter):
        raise Exception(f"no wayland greeter session among {greeter}")
    await_ready(machine, "username", 1)

    machine.send_chars("${user}\n${password}\n")
    machine.wait_until_succeeds("test -s /tmp/lys-login/logind", timeout=60)

    session = machine.succeed("cat /tmp/lys-login/session").split()
    if session[:3] != ["x11", "/usr/bin/env", "stand-in-session"]:
        raise Exception(f"the session ran as {session}")
    logind = machine.succeed("cat /tmp/lys-login/logind")
    if "Type=x11" not in logind:
        raise Exception(f"logind holds the X11 session as {logind}")
  '';
}
