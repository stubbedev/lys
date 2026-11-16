{
  pkgs,
  lib,
}: let
  cursorFile = "/run/lys-login-vm/cursor";

  scenarioJournal = pkgs.writeShellScriptBin "scenario-journal" ''
    if [ -s ${cursorFile} ]; then
      exec journalctl --after-cursor="$(cat ${cursorFile})" "$@"
    fi
    exec journalctl "$@"
  '';

  dispatch = sessions:
    pkgs.writeShellScript "lys-login-vm-session" ''
      case "$(id -un)" in
      ${lib.concatStrings (lib.mapAttrsToList (user: session: "  ${user}) exec ${session} ;;\n") sessions)}
        *) echo "lys-login-vm: no session for $(id -un)" >&2; exit 1 ;;
      esac
    '';

  pyList = xs: "[" + lib.concatMapStringsSep ", " (x: x) xs + "]";
  pyStr = s: "\"${s}\"";
  pyOpt = s:
    if s == null
    then "None"
    else pyStr s;
in {
  inherit dispatch;

  lockConfig = pkgs.writeText "lys-login-vm-lock-config.yaml" ''
    login:
      lock:
        pam-service: lys-shell
  '';

  lysWmSession = {
    name,
    agent,
    config ? {},
  }:
    pkgs.writeShellScript "lys-login-vm-${name}-session" ''
      exec lys-wm --config ${(pkgs.formats.yaml {}).generate "lys-login-vm-${name}-session.yaml" ({
          binds = [];
          animations.enabled = false;
          autostart = ["${agent}"];
        }
        // config)}
    '';

  module = {
    virtualisation.graphics = true;
    virtualisation.cores = 4;
    virtualisation.memorySize = 2048;
    virtualisation.qemu.options = [
      "-vga none"
      "-device virtio-gpu-pci"
    ];
    environment.sessionVariables = {
      LIBGL_ALWAYS_SOFTWARE = "1";
      LYS_WM_RENDERER = "gles";
    };
    environment.systemPackages = [scenarioJournal];
    hardware.graphics.enable = true;
    hardware.graphics.extraPackages = [pkgs.mesa];
    fonts.packages = [pkgs.inter];
    systemd.tmpfiles.rules = [
      "d /tmp/lys-login 0777 root root -"
      "d ${builtins.dirOf cursorFile} 0755 root root -"
    ];
  };

  systemAccount = {
    services.accounts-daemon.enable = true;
    users.users.spare = {
      isNormalUser = true;
      password = "spare-vm";
    };
    systemd.tmpfiles.rules = [
      "d /var/lib/AccountsService/users 0700 root root -"
      "f+ /var/lib/AccountsService/users/spare 0600 root root - [User]\\nSystemAccount=true\\n"
    ];
  };

  suite = {
    name,
    displayManager,
    journal,
    node,
    scenarios,
    globalTimeout,
  }:
    pkgs.testers.runNixOSTest ({nodes, ...}: {
      inherit name globalTimeout;
      nodes.machine = node;
      testScript =
        ''
          GREETER_STATE = ["/var/lib/lys-greeter/" + f for f in ("last-session", "last-user", "last-layout", "accessibility")]

          def await_ready(machine, field, occurrence):
              machine.wait_until_succeeds(
                  f"[ \"$(scenario-journal ${journal} -o cat --no-pager | grep -c 'greeter: ready: .* the {field} field')\" -ge {occurrence} ]",
                  timeout=180,
              )

          def no_user_processes():
              return "[ -z \"$(ps -eo uid= | awk '$1 >= 1000 && $1 < 65534')\" ]"

          def reset(machine, specialisation):
              machine.wait_for_unit("multi-user.target")
              machine.succeed("systemctl stop ${displayManager}")
              machine.succeed("for uid in $(loginctl list-users --no-legend | awk '$1 > 0 {print $1}'); do loginctl terminate-user $uid || ! loginctl show-user $uid > /dev/null 2>&1 || exit 1; done")
              machine.wait_until_succeeds("[ -z \"$(loginctl list-users --no-legend | awk '$1 > 0')\" ]", timeout=60)
              machine.wait_until_succeeds(no_user_processes(), timeout=60)
              machine.succeed(
                  "find /tmp/lys-login -mindepth 1 -delete",
                  "rm -rf /var/lib/lys-login/cursor /etc/lys-shell/runtime.yaml " + " ".join(GREETER_STATE),
              )
              toplevel = "/run/booted-system" if specialisation is None else f"/run/booted-system/specialisation/{specialisation}"
              machine.succeed(f"[ \"$(readlink -f /run/current-system)\" = \"$(readlink -f {toplevel})\" ] || {toplevel}/bin/switch-to-configuration test")
              machine.succeed(
                  "systemctl stop ${displayManager} systemd-localed",
                  "journalctl --sync",
                  "journalctl -n 1 --show-cursor -o cat --no-pager | sed -n 's/^-- cursor: //p' > ${cursorFile}",
                  "test -s ${cursorFile}",
                  "systemctl start ${displayManager}",
              )

          def diagnose(machine, name):
              machine.screenshot(f"failure-{name}")
              print(machine.execute("scenario-journal -b --no-pager -n 300")[1])
              print(machine.execute("for f in /tmp/lys-login/*.err; do [ -e \"$f\" ] && echo \"--- $f\" && cat \"$f\"; done")[1])

          def run_scenarios(machine, scenarios):
              machine.start()
              failures = []
              for i, (name, specialisation, run) in enumerate(scenarios):
                  if i > 0 or specialisation is not None:
                      try:
                          reset(machine, specialisation)
                      except Exception as e:
                          raise Exception(f"resetting the machine for scenario {name} failed: {e}" + "".join(f"\nscenario {f}" for f in failures)) from e
                  print(f"=== scenario {name}")
                  try:
                      run()
                  except Exception as e:
                      diagnose(machine, name)
                      failures.append(f"{name} failed: {e}")
              if failures:
                  raise Exception("\n".join(f"scenario {f}" for f in failures))

        ''
        + lib.concatMapStrings (s: "def ${s.name}():\n" + lib.concatMapStrings (line: "    ${line}\n") (lib.splitString "\n" (s.script nodes)) + "\n") scenarios
        + ''
          run_scenarios(machine, ${pyList (map (s: "(${pyStr s.name}, ${pyOpt (s.specialisation or null)}, ${s.name})") scenarios)})
        '';
    });
}
