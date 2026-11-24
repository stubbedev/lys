{
  pkgs,
  lysShell ? null,
  portal,
  seconds,
}: let
  firefoxPrefs = pkgs.writeText "user.js" ''
    user_pref("media.navigator.permission.disabled", true);
    user_pref("browser.aboutwelcome.enabled", false);
    user_pref("browser.shell.checkDefaultBrowser", false);
    user_pref("browser.startup.homepage_override.mstone", "ignore");
    user_pref("datareporting.policy.dataSubmissionEnabled", false);
    user_pref("toolkit.telemetry.reportingpolicy.firstRun", false);
  '';
  launch = import ./launch.nix pkgs;
  page = "file://${./screencast.html}?seconds=${toString seconds}";
  electronApp = pkgs.runCommand "electron-share" {} "mkdir $out && cp ${./electron-share}/* $out/ && cp ${./screencast.html} $out/screencast.html";
  python = pkgs.python3.withPackages (ps: [ps.dbus-python ps.pygobject3]);
  frames = import ./c-program.nix pkgs {
    name = "pw-frames";
    source = ./pw-frames.c;
    libraries = [pkgs.pipewire];
    modules = ["libpipewire-0.3"];
  };
  background = pkgs.writeShellScript "background" ''
    set -euo pipefail
    name=$1
    shift
    # shellcheck disable=SC2016
    ${pkgs.util-linux}/bin/setsid -f ${pkgs.bash}/bin/bash -c '"$@" > "/tmp/$0.log" 2>&1; echo $? > "/tmp/$0.status"' "$name" "$@" < /dev/null > /dev/null 2>&1
  '';
  methodCalls = pkgs.writeShellScript "method-calls" ''
    exec ${pkgs.dbus}/bin/dbus-monitor --session "type='method_call',interface='$1',member='$2'"
  '';
  portals = {
    wlr = {
      name = "screencast";
      home = {};
      log = "xdg-desktop-portal-wlr";
      chooserRan = "wlroots: chooser called";
      backgroundOnly = true;
      machineConfig = {config, ...}: let
        chooser = pkgs.writeShellScript "share-pick" ''
          set -euo pipefail
          ${pkgs.gnugrep}/bin/grep -m1 -F -- "$(${pkgs.coreutils}/bin/cat /tmp/share-pick)"
        '';
      in {
        xdg.portal.wlr.enable = true;
        xdg.portal.config.lys-wm = {
          "org.freedesktop.impl.portal.ScreenCast" = "wlr";
          "org.freedesktop.impl.portal.Screenshot" = "wlr";
        };
        xdg.portal.wlr.settings.screencast = {
          chooser_type = "dmenu";
          chooser_cmd = "${chooser}";
        };
        systemd.user.services.xdg-desktop-portal-wlr.serviceConfig.ExecStart = pkgs.lib.mkForce [
          ""
          "${pkgs.xdg-desktop-portal-wlr}/libexec/xdg-desktop-portal-wlr --loglevel=DEBUG --config=${(pkgs.formats.ini {}).generate "xdg-desktop-portal-wlr.ini" config.xdg.portal.wlr.settings}"
        ];
      };
    };
    lys-shell = {
      name = "screencast-lys-shell";
      log = "lys-shell";
      chooserRan = null;
      backgroundOnly = false;
      home.".config/lys-shell/config.yaml" = builtins.toJSON {
        wallpaper = {
          wallpaper = pkgs.runCommand "desktop.png" {nativeBuildInputs = [pkgs.imagemagick];} "magick -size 64x64 xc:'#1e1e2e' png24:$out";
          fit-mode = "fill";
        };
      };
      machineConfig = {
        imports = [lysShell];
        programs.lys-shell = {
          enable = true;
          systemd.enable = true;
        };
        fonts.packages = [pkgs.inter];
      };
    };
  };
  chosen = portals.${portal};
  launchers = builtins.mapAttrs launch {
    animation = "${pkgs.weston}/bin/weston-presentation-shm";
    firefox = "${pkgs.firefox}/bin/firefox --no-remote --profile /tmp/firefox '${page}'";
    chromium = "${pkgs.chromium}/bin/chromium --enable-logging=stderr --v=1 --no-first-run --user-data-dir=/tmp/chromium '${page}'";
    tabs = "${pkgs.chromium}/bin/chromium --ozone-platform=wayland --enable-logging=stderr --vmodule=wayland_window_drag_controller=3,tab_drag_controller=3,wayland_data_drag_controller=3 --no-first-run --user-data-dir=/tmp/chromium-tabs file:///tmp/tab-one.html file:///tmp/tab-two.html";
    electron = "${pkgs.electron}/bin/electron --ozone-platform=wayland ${electronApp} ${toString seconds}";
  };
in {
  inherit (chosen) name backgroundOnly home;
  lysWMConfig = ''
    rules:
      - {title: ^presentation-shm:, tile: true}
      - {app_id: ^firefox$, suppress: [maximize]}
  '';
  machineConfig = {
    imports = [chosen.machineConfig];
    services.pipewire.enable = true;
    environment.systemPackages = [python frames pkgs.wayland-utils];
  };
  params = {
    screencast = "${python}/bin/python3 ${./screencast.py}";
    firefox_prefs = "${firefoxPrefs}";
    portal_log = chosen.log;
    chooser_ran = chosen.chooserRan;
    max_gap_ms = 2000;
    background = "${background}";
    method_calls = "${methodCalls}";
    page_source = "${./screencast.html}";
    inherit launchers portal seconds;
  };
  scripts = [./screencast-test.py];
}
