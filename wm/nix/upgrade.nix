pkgs: let
  launch = import ./launch.nix pkgs;
  bad = pkgs.writeShellScript "not-a-lys-wm" ''
    case "$1" in
      --handover-info) exec lys-wm "$@" ;;
    esac
    echo "this binary breaks on purpose" >&2
    sleep 1
    kill -SEGV $$
  '';
  headless = pkgs.writeShellScript "headless-session" ''
    set -euo pipefail
    install -d -m 0700 /tmp/headless
    export XDG_RUNTIME_DIR=/tmp/headless WAYLAND_DISPLAY=
    unset DISPLAY LYS_WM_SOCKET
    exec lys-wm --backend headless --renderer soft --holder --config /tmp/headless.yaml > /tmp/headless.log 2>&1
  '';
  headlessFoot = pkgs.writeShellScript "headless-foot" ''
    set -euo pipefail
    export XDG_RUNTIME_DIR=/tmp/headless WAYLAND_DISPLAY="$1"
    unset DISPLAY
    exec ${pkgs.foot}/bin/foot --app-id headless > /tmp/headless-foot.log 2>&1
  '';
in {
  name = "upgrade";
  backgroundOnly = false;
  lysWMArgs = "--holder";
  lysWMConfig = ''
    binds:
      - {chord: SUPER + Return, action: exec ${pkgs.foot}/bin/foot}
  '';
  machineConfig = {
    fonts.packages = [pkgs.dejavu_fonts];
    environment.systemPackages = [pkgs.wtype pkgs.mako pkgs.foot pkgs.xterm];
  };
  params = {
    bad = "${bad}";
    headless = "${headless}";
    headless_foot = "${headlessFoot}";
    typed = launch "typed" "${pkgs.foot}/bin/foot --app-id typed ${pkgs.bash}/bin/bash -c 'cat > /tmp/typed'";
    idle = launch "idle" "${pkgs.foot}/bin/foot --app-id idle";
    mako = launch "mako" "${pkgs.mako}/bin/mako";
    notify = launch "notify" "${pkgs.libnotify}/bin/notify-send upgrade hello";
    xterm = launch "xterm" "${pkgs.xterm}/bin/xterm";
  };
  scripts = [./upgrade.py];
}
