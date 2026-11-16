{
  pkgs,
  lib,
  self,
  lysGo,
}:

let
  user = "alice";
  password = "lys-shell-vm";
  uid = 1000;
  runtimeDir = "/run/user/${toString uid}";

  ui = import ../../ui/tests/hyprland-vm.nix { inherit pkgs lib lysGo; };

  hyprLua = pkgs.writeText "hyprland-vm.lua" ''
    hl.monitor({ output = "Virtual-1", mode = "1280x800", position = "0x0", scale = 1 })
    hl.monitor({ output = "HEADLESS-2", mode = "1024x768", position = "1280x0", scale = 1 })
    hl.monitor({ output = "", mode = "preferred", position = "auto", scale = 1 })

    hl.config({
      animations = { enabled = false },
      input = { kb_layout = "us,de", repeat_delay = 2000 },
      decoration = { rounding = 0, shadow = { enabled = false } },
      misc = {
        disable_splash_rendering = true,
        force_default_wallpaper = 0,
        allow_session_lock_restore = true,
      },
      debug = { disable_logs = false },
    })

    hl.workspace_rule({ workspace = "1", monitor = "Virtual-1" })
    hl.workspace_rule({ workspace = "2", monitor = "Virtual-1" })
    hl.workspace_rule({ workspace = "3", monitor = "HEADLESS-2" })

    hl.define_submap("resize", function()
      hl.bind("escape", hl.dsp.submap("reset"))
    end)

    hl.on("hyprland.start", function()
      hl.exec_cmd("dbus-update-activation-environment --systemd WAYLAND_DISPLAY XDG_CURRENT_DESKTOP HYPRLAND_INSTANCE_SIGNATURE && systemctl --user start hyprland-vm-session.target")
      hl.exec_cmd("lys-shell shell > /tmp/lys/shell.log 2>&1")
    end)
  '';

  inherit (import ./vm-tools.nix { inherit pkgs self; }) vmclick portalClient testApp;

  shellConfig = (pkgs.formats.yaml { }).generate "config.yaml" {
    login.lock = {
      enabled = true;
      pam-service = "lys-shell";
    };
    bar.layout = [
      {
        monitor = "*";
        left = [
          "hyprland-workspaces"
          "window-title"
        ];
        center = [ "clock" ];
        right = [
          "keybind-mode"
          "keyboard-input"
          "notifications"
          "nightlight"
        ];
      }
    ];
  };
in
pkgs.testers.nixosTest {
  name = "lys-shell-hyprland-vms";

  nodes.machine =
    { ... }:
    {
      imports = [
        self.nixosModules.lys-shell
        self.nixosModules.lys-secrets
        self.nixosModules.lys-login
        ui.module
      ];

      programs.lys-shell.enable = true;
      programs.lys-login = {
        enable = true;
        lock.enable = true;
      };
      services.lys-secrets.enable = true;

      virtualisation.resolution = {
        x = 1280;
        y = 800;
      };

      services.getty.autologinUser = user;
      environment.loginShellInit = ''
        if [ "$(tty)" = /dev/tty1 ]; then
          exec Hyprland > /tmp/lys/hyprland.log 2>&1
        fi
      '';
      environment.sessionVariables.LIBGL_ALWAYS_SOFTWARE = "1";
      systemd.tmpfiles.rules = [
        "d /tmp/lys 0777 root root -"
        "d /home/${user}/.config 0755 ${user} users -"
        "d /home/${user}/.config/hypr 0755 ${user} users -"
        "d /home/${user}/.config/lys-shell 0755 ${user} users -"
        "L+ /home/${user}/.config/hypr/hyprland.lua - - - - ${hyprLua}"
        "C /home/${user}/.config/lys-shell/config.yaml 0644 ${user} users - ${shellConfig}"
      ];
      fonts.packages = [ pkgs.inter ];

      users.users.${user} = {
        isNormalUser = true;
        uid = lib.mkForce uid;
        inherit password;
        extraGroups = [
          "video"
          "render"
          "input"
        ];
      };

      systemd.user.targets.hyprland-vm-session = {
        bindsTo = [ "graphical-session.target" ];
        wants = [ "graphical-session-pre.target" ];
        after = [ "graphical-session-pre.target" ];
      };

      services.pipewire.enable = true;

      environment.systemPackages = [
        portalClient
        testApp
        vmclick
        pkgs.foot
        pkgs.libnotify
        pkgs.dbus
        pkgs.libsecret
      ];
    };

  testScript = ''
    import json
    import time

    ${ui.testScript}
    def shell_scenarios():
        def wait(what, check, tries=60, pause=1.0):
            for _ in range(tries):
                if check():
                    return
                time.sleep(pause)
            print(machine.execute("tail -n 40 /tmp/lys/*.json; cat /tmp/lys/*.log | grep -v aquamarine | tail -n 80")[1])
            print(machine.execute("journalctl _UID=${toString uid} --no-pager | grep -v aquamarine | grep -i 'lys\\|portal' | tail -n 60")[1])
            raise Exception("timed out waiting for " + what)

        wait("Hyprland's socket", lambda: machine.execute("ls ${runtimeDir}/hypr/*/.socket.sock")[0] == 0, tries=90)
        sig = machine.succeed("ls ${runtimeDir}/hypr").split()[0]
        env = (
            f"XDG_RUNTIME_DIR=${runtimeDir} HYPRLAND_INSTANCE_SIGNATURE={sig} "
            "DBUS_SESSION_BUS_ADDRESS=unix:path=${runtimeDir}/bus"
        )

        def hypr(args):
            return machine.succeed(f"su - ${user} -c '{env} hyprctl {args}'")

        def dispatch(lua):
            escaped = lua.replace('"', '\\"')
            reply = hypr(f'dispatch "{escaped}"').strip()
            if reply != "ok":
                raise Exception(f"dispatch {lua} answered {reply}")

        def hypr_json(args):
            return json.loads(hypr(f"-j {args}"))

        print(hypr("version"))
        config_errors = hypr("configerrors").strip()
        if config_errors:
            raise Exception(f"Hyprland rejected the config: {config_errors}")

        def namespaces():
            found = []
            for mon in hypr_json("layers").values():
                for level in mon["levels"].values():
                    found += [layer["namespace"] for layer in level]
            return found

        def shell(args):
            return machine.succeed(
                f"su - ${user} -c '{env} WAYLAND_DISPLAY=$(basename $(ls ${runtimeDir}/wayland-* | head -1)) "
                f"lys-shell {args}'"
            )

        def shell_bg(args, log):
            machine.succeed(
                f"su - ${user} -c '{env} WAYLAND_DISPLAY=$(basename $(ls ${runtimeDir}/wayland-* | head -1)) "
                f"lys-shell {args} > /tmp/lys/{log}.log 2>&1 &'"
            )

        def outputs():
            return sorted(m["name"] for m in hypr_json("monitors"))

        def bars_on(names):
            return all(f"lys-shell-bar-{n}" in namespaces() for n in names)

        wait("the first output", lambda: outputs() == ["Virtual-1"])
        wait("a bar on the output", lambda: bars_on(["Virtual-1"]), tries=90)

        hypr("output create headless HEADLESS-2")
        wait("the hotplugged output", lambda: "HEADLESS-2" in outputs())
        names = outputs()
        wait("a bar on every output", lambda: bars_on(names), tries=60)
        for m in hypr_json("monitors"):
            if m["reserved"][1] <= 0:
                raise Exception(f"{m['name']} reserves no space for the bar: {m['reserved']}")
        machine.screenshot("01-bar")

        dispatch('hl.dsp.exec_cmd("foot", { workspace = "1 silent" })')
        dispatch('hl.dsp.exec_cmd("foot", { workspace = "3 silent" })')
        wait("two windows", lambda: len(hypr_json("clients")) == 2, tries=60)
        dispatch("hl.dsp.focus({ workspace = 2 })")
        time.sleep(1)
        machine.screenshot("02-workspace-2")
        dispatch("hl.dsp.focus({ workspace = 1 })")
        dispatch('hl.dsp.workspace.toggle_special("scratch")')
        time.sleep(1)
        machine.screenshot("03-special-workspace")
        dispatch('hl.dsp.workspace.toggle_special("scratch")')

        hypr("switchxkblayout all next")
        dispatch('hl.dsp.submap("resize")')
        time.sleep(1)
        machine.screenshot("04-layout-de-and-resize-submap")
        dispatch('hl.dsp.submap("reset")')
        hypr("switchxkblayout all prev")

        shell_bg("launcher -show drun", "launcher")
        wait("the launcher", lambda: "lys-shell-launcher" in namespaces())
        time.sleep(1)
        machine.screenshot("05-launcher")
        machine.send_key("esc")
        wait("the launcher to close", lambda: "lys-shell-launcher" not in namespaces())

        machine.succeed(
            f"su - ${user} -c '{env} "
            "notify-send -a Firefox \"Download complete\" \"report.pdf finished downloading\"'"
        )
        wait("the notification popup", lambda: "lys-shell-notification-popup" in namespaces())
        time.sleep(1)
        machine.screenshot("06-notification")

        def click(x, y):
            mons = hypr_json("monitors")
            width = max(m["x"] + m["width"] for m in mons)
            height = max(m["y"] + m["height"] for m in mons)
            machine.succeed(
                f"su - ${user} -c '{env} WAYLAND_DISPLAY=$(basename $(ls ${runtimeDir}/wayland-* | head -1)) "
                f"vmclick {x} {y} {width} {height}'"
            )

        def layer(namespace):
            for mon in hypr_json("layers").values():
                for level in mon["levels"].values():
                    for found in level:
                        if found["namespace"] == namespace:
                            return found
            raise Exception(f"no {namespace} layer")

        click(640, 22)
        time.sleep(2)
        machine.screenshot("06b-calendar-dropdown")
        click(1100, 700)
        time.sleep(1)

        machine.succeed(f"su - ${user} -c '{env} portal-client screenshot > /tmp/lys/shot.json 2>&1 &'")
        wait("the screenshot consent dialog", lambda: "lys-shell-portal-dialog" in namespaces())
        time.sleep(1)
        machine.screenshot("06c-screenshot-consent")
        dialog = layer("lys-shell-portal-dialog")
        click(dialog["x"] + dialog["w"] - 60, dialog["y"] + dialog["h"] - 32)
        wait("the screenshot answer", lambda: machine.execute("grep -q code /tmp/lys/shot.json")[0] == 0, tries=30)
        shot = json.loads(machine.succeed("cat /tmp/lys/shot.json"))
        uri = shot["results"]["uri"]
        if not uri.startswith("file://"):
            raise Exception(f"screenshot portal answered {shot}")
        machine.succeed(f"test -s '{uri[len('file://'):]}'")

        machine.succeed(f"su - ${user} -c '{env} portal-client screencast > /tmp/lys/cast.json 2>&1 &'")
        wait("the share picker", lambda: "lys-shell-share-picker" in namespaces())
        time.sleep(2)
        machine.screenshot("06c-share-picker")
        machine.send_key("esc")
        wait("the cancelled screencast", lambda: machine.execute("grep -q code /tmp/lys/cast.json")[0] == 0)
        print(machine.succeed("cat /tmp/lys/cast.json"))

        shell_bg("screenshot region", "region")
        wait("the region overlay", lambda: "lys-shell-region-overlay" in namespaces())
        time.sleep(1)
        machine.screenshot("06d-region-overlay")
        machine.send_key("esc")
        wait("the region overlay to close", lambda: "lys-shell-region-overlay" not in namespaces())

        def monitors_of(namespace):
            return [
                name
                for name, mon in hypr_json("layers").items()
                for level in mon["levels"].values()
                if any(found["namespace"] == namespace for found in level)
            ]

        def toast_and_notify(word):
            shell(f"toast --duration 4000 {word}")
            machine.succeed(f"su - ${user} -c '{env} notify-send {word}'")

        dispatch('hl.dsp.focus({ monitor = "HEADLESS-2" })')
        toast_and_notify("Primary")
        wait("the OSD", lambda: "lys-shell-osd" in namespaces())
        wait("the popup", lambda: "lys-shell-notification-popup" in namespaces())
        if monitors_of("lys-shell-osd") != ["Virtual-1"] or "Virtual-1" not in monitors_of("lys-shell-notification-popup"):
            raise Exception(f"default placement: OSD on {monitors_of('lys-shell-osd')}, popups on {monitors_of('lys-shell-notification-popup')}, want the primary Virtual-1")
        machine.screenshot("06e-osd-and-popup-on-primary")
        wait("the OSD to time out", lambda: "lys-shell-osd" not in namespaces(), tries=15)
        shell("config set osd.monitor HEADLESS-2")
        shell("config set modules.notifications.popup-monitor HEADLESS-2")
        toast_and_notify("Configured")
        wait("the OSD on the configured output", lambda: monitors_of("lys-shell-osd") == ["HEADLESS-2"])
        wait("a popup on the configured output", lambda: "HEADLESS-2" in monitors_of("lys-shell-notification-popup"))
        machine.screenshot("06e-osd-and-popup-on-configured-output")
        shell("config reset osd.monitor")
        shell("config reset modules.notifications.popup-monitor")
        dispatch('hl.dsp.focus({ monitor = "Virtual-1" })')

        machine.succeed(f"su - ${user} -c '{env} portal-client pickcolor > /tmp/lys/color.json 2>&1 &'")
        wait("the colour picker", lambda: "lys-shell-color-picker" in namespaces())
        time.sleep(1)
        machine.screenshot("06f-color-picker")
        click(400, 400)
        wait("the picked colour", lambda: machine.execute("grep -q code /tmp/lys/color.json")[0] == 0, tries=30)
        picked = json.loads(machine.succeed("cat /tmp/lys/color.json"))
        color = picked["results"].get("color")
        if picked["code"] != 0 or not color or len(color) != 3:
            raise Exception(f"PickColor answered {picked}")

        machine.succeed(f"su - ${user} -c '{env} portal-client globalshortcuts > /tmp/lys/shortcuts.json 2>&1 &'")
        wait("the shortcut consent dialog", lambda: "lys-shell-portal-dialog" in namespaces())
        time.sleep(1)
        machine.screenshot("06g-shortcut-consent")
        dialog = layer("lys-shell-portal-dialog")
        click(dialog["x"] + dialog["w"] - 60, dialog["y"] + dialog["h"] - 32)
        wait("the bound shortcut", lambda: machine.execute("grep -q bound /tmp/lys/shortcuts.json")[0] == 0, tries=30)
        bound = json.loads(machine.succeed("head -n1 /tmp/lys/shortcuts.json"))
        if bound["bound"] != 0:
            raise Exception(f"BindShortcuts answered {bound}")
        shortcuts = hypr_json("globalshortcuts")
        held = [s for s in shortcuts if s["description"] == "lys-shell VM test"]
        if len(held) != 1 or held[0].get("app_id") != "dev.stubbe.lys.VmTest" or held[0].get("trigger") != "CTRL+ALT+p":
            raise Exception(f"the compositor holds no CTRL+ALT+p hotkey for the bound shortcut: {shortcuts}")
        machine.send_key("ctrl-alt-p")
        wait("the shortcut's activation", lambda: machine.execute("grep -q activated /tmp/lys/shortcuts.json")[0] == 0, tries=30)

        def logged(log, line):
            return line in machine.execute(f"cat /tmp/lys/{log}.log")[1]

        if machine.execute("command -v hyprsunset")[0] == 0:
            raise Exception("hyprsunset is installed; the night light must not need it")
        click(1262, 22)
        wait("the night light on", lambda: logged("shell", "nightlight: on at 5000K 100% through hyprland_ctm_control_manager_v1"), tries=30)
        machine.screenshot("06h-nightlight-on")
        click(1262, 22)
        wait("the night light off", lambda: logged("shell", "nightlight: off"), tries=30)
        if logged("shell", "another program controls the display colours"):
            raise Exception("the compositor refused the night light")

        def count(log, line):
            return machine.execute(f"cat /tmp/lys/{log}.log")[1].count(line)

        shell("lock")
        wait("the session lock", lambda: logged("shell", "lock: session locked (2 monitors)"))
        time.sleep(1)
        machine.screenshot("07-locked")
        machine.send_chars("${password}\n")
        wait("the unlock", lambda: logged("shell", "lock: session unlocked"))
        machine.screenshot("08-unlocked")

        def secret_tool(args, stdin=""):
            return machine.execute(f"su - ${user} -c 'printf %s {stdin} | env {env} timeout 20 secret-tool {args}'")

        wait("lys-secrets running", lambda: machine.execute(f"su - ${user} -c '{env} systemctl --user is-active lys-secrets'")[0] == 0)
        status, _ = secret_tool("store --label=vm vm-test 1", "vm-secret")
        if status != 0:
            raise Exception("secret-tool store failed after the lock screen unlock")
        if secret_tool("lookup vm-test 1")[1] != "vm-secret":
            raise Exception("secret-tool lookup did not return the stored secret")
        if machine.execute("ls /home/${user}/.local/share/lys-secrets/login.keyring")[0] != 0:
            raise Exception("no login collection on disk")
        if machine.execute("grep -c vm-secret /home/${user}/.local/share/lys-secrets/login.keyring")[0] == 0:
            raise Exception("the secret is stored in the clear")

        machine.succeed(f"su - ${user} -c '{env} systemctl --user restart lys-secrets'")
        wait("lys-secrets restarted", lambda: machine.execute(f"su - ${user} -c '{env} systemctl --user is-active lys-secrets'")[0] == 0)
        machine.succeed(f"su - ${user} -c '{env} timeout 60 secret-tool lookup vm-test 1 > /tmp/lys/lookup.txt 2>&1 &'")
        wait("the keyring password dialog", lambda: "lys-shell-portal-dialog" in namespaces())
        time.sleep(1)
        machine.screenshot("08b-keyring-unlock-dialog")
        machine.send_chars("${password}\n")
        wait("the lookup through the dialog", lambda: machine.execute("cat /tmp/lys/lookup.txt")[1] == "vm-secret", tries=30)

        shell("lock")
        wait("the second lock", lambda: count("shell", "lock: session locked") == 2)
        machine.succeed("pkill -KILL -f 'lys-login lock'")
        wait("the shell to re-acquire the lock", lambda: logged("shell", "still locked; re-acquiring") and count("shell", "lock: session locked") == 3)
        time.sleep(1)
        machine.screenshot("09-relocked-after-the-lock-client-died")
        machine.succeed("pkill -KILL -f 'lys-shell shell'")
        dispatch('hl.dsp.exec_cmd("lys-shell shell > /tmp/lys/shell2.log 2>&1")')
        wait("the restarted shell to find the lock held", lambda: logged("shell2", "re-acquiring") and logged("shell2", "another lys-login already holds the session locked"))
        machine.screenshot("10-locked-after-a-shell-restart")
        machine.send_chars("${password}\n")
        wait("the unlock after the restart", lambda: count("shell", "lock: session unlocked") == 2)
        wait("the bar after the restart", lambda: bars_on(names))
        machine.screenshot("11-unlocked-after-restart")

        hypr("output remove HEADLESS-2")
        wait("the unplugged output's bar to go", lambda: "lys-shell-bar-HEADLESS-2" not in namespaces())
        if not bars_on(["Virtual-1"]):
            raise Exception("unplugging an output took the remaining bar with it")

        print(machine.execute("cat /tmp/lys/shell.log /tmp/lys/shell2.log")[1])

    start_all()
    machine.wait_for_unit("multi-user.target")
    machine.succeed("test -e /dev/dri/card0", "test -e /dev/dri/renderD128")

    with subtest("the shell on the session's Hyprland"):
        shell_scenarios()

    with subtest("the shell's session hands the outputs over"):
        machine.succeed("systemctl stop getty@tty1.service")
        machine.succeed("loginctl terminate-user ${user}")
        machine.wait_until_fails("pgrep -x Hyprland")

    with subtest("the ui headless suite on its own Hyprland"):
        ui_suite()
  '';
}
