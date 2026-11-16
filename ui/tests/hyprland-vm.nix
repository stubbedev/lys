{
  pkgs,
  lib,
  lysGo,
}: let
  hyprLua = pkgs.writeText "lys-ui-vm-hypr.lua" ''
    hl.monitor({ output = "", mode = "preferred", position = "auto", scale = 1 })

    hl.config({
      animations = { enabled = false },
      input = { repeat_delay = 2000 },
      decoration = { rounding = 0, shadow = { enabled = false } },
      misc = {
        disable_splash_rendering = true,
        force_default_wallpaper = 0,
        focus_on_activate = true,
      },
      debug = { disable_logs = false, enable_stdout_logs = true },
    })

    hl.window_rule({ name = "lys-ui-hello", match = { class = [[^(dev\.stubbe\.lys\.ui\.hello)$]] }, float = true, size = "640 470", move = "0 0" })
    hl.window_rule({ name = "lys-ui-states", match = { class = [[^(dev\.stubbe\.lys\.ui\.states)$]] }, float = true, size = "420 280", move = "40 40" })
    hl.window_rule({ name = "lys-ui-multilist", match = { class = [[^(dev\.stubbe\.lys\.ui\.multilist)$]] }, float = true, size = "300 300", move = "0 0" })
    hl.window_rule({ name = "lys-ui-modal-dialog", match = { title = [[^modal dialog$]] }, float = true, move = "340 0" })
  '';

  suite = lysGo.build {
    pname = "lys-ui-headless-suite";
    goPackages = ["ui/cmd/..."];
    goTests = ["ui/internal/headlesstest"];
    doCheck = false;
    buildPhase = ''
      runHook preBuild
      go test -c -o headlesstest.test ./ui/internal/headlesstest
      mkdir clients
      go build -tags lysdebug -o clients/ ./ui/cmd/...
      runHook postBuild
    '';
    installPhase = ''
      runHook preInstall
      install -Dm755 -t $out/bin headlesstest.test
      install -Dm755 -t $out/clients clients/*
      runHook postInstall
    '';
  };

  user = "lys-ui";
  uid = 1001;
  runtimeDir = "/run/user/${toString uid}";
  bus = "unix:path=${runtimeDir}/lys-ui-bus";
  busConfig = pkgs.writeText "lys-ui-vm-bus.conf" ''
    <!DOCTYPE busconfig PUBLIC "-//freedesktop//DTD D-BUS Bus Configuration 1.0//EN" "http://www.freedesktop.org/standards/dbus/1.0/busconfig.dtd">
    <busconfig>
      <type>session</type>
      <listen>${bus}</listen>
      <auth>EXTERNAL</auth>
      <policy context="default">
        <allow send_destination="*" eavesdrop="true"/>
        <allow eavesdrop="true"/>
        <allow own="*"/>
      </policy>
    </busconfig>
  '';
  sig = "lys-ui-vm";
  log = "/tmp/lysuitest";
in {
  module = {
    virtualisation.graphics = true;
    virtualisation.cores = 4;
    virtualisation.memorySize = 4096;
    virtualisation.qemu.options = [
      "-vga none"
      "-device virtio-gpu-pci"
    ];
    hardware.graphics.enable = true;
    hardware.graphics.extraPackages = [pkgs.mesa];
    services.seatd.enable = true;
    users.users.${user} = {
      isNormalUser = true;
      uid = lib.mkForce uid;
      extraGroups = [
        "seat"
        "video"
        "render"
        "input"
      ];
    };
    environment.systemPackages = [
      pkgs.hyprland
      pkgs.vanilla-dmz
    ];
  };

  testScript = ''
    def ui_suite():
        machine.succeed("install -d -m 0700 -o ${user} -g users ${runtimeDir}")
        machine.succeed("install -d -m 0755 -o ${user} -g users ${log}")
        machine.succeed(
          "install -m 0644 -o ${user} -g users ${hyprLua} ${runtimeDir}/hypr.lua"
        )

        verdict = machine.succeed(
          "su - ${user} -c 'XDG_RUNTIME_DIR=${runtimeDir} Hyprland --verify-config -c ${runtimeDir}/hypr.lua 2>&1' | tail -n 5"
        )
        if "config ok" not in verdict:
            raise Exception("the ui suite's Hyprland config does not verify:\n" + verdict)

        machine.succeed(
          "su - ${user} -c '${pkgs.dbus}/bin/dbus-daemon --config-file=${busConfig} --fork --nopidfile'"
        )

        machine.succeed(
          "su - ${user} -c '"
          "XDG_RUNTIME_DIR=${runtimeDir} DBUS_SESSION_BUS_ADDRESS=${bus} HYPRLAND_INSTANCE_SIGNATURE=${sig} LIBGL_ALWAYS_SOFTWARE=1 "
          "Hyprland -c ${runtimeDir}/hypr.lua >${runtimeDir}/hyprland.log 2>&1 "
          "& echo $! > ${runtimeDir}/hyprland.pid'"
        )

        def boot_logs():
            print(machine.execute("cat ${runtimeDir}/hyprland.log")[1])
            print(machine.execute("cat ${runtimeDir}/hypr/*/hyprland.log 2>/dev/null | tail -n 60")[1])
            print(machine.execute("cat /home/${user}/.cache/hyprland/crash-reports/*.txt 2>/dev/null | head -n 80")[1])

        for attempt in range(60):
            if machine.execute("ls ${runtimeDir}/wayland-*")[0] == 0:
                break
            time.sleep(2)
        else:
            boot_logs()
            raise Exception("the ui suite's Hyprland never created its Wayland socket in ${runtimeDir}")

        hypr_sig = machine.succeed("ls ${runtimeDir}/hypr").split()[0]
        hyprctl = (
          "su - ${user} -c 'XDG_RUNTIME_DIR=${runtimeDir} HYPRLAND_INSTANCE_SIGNATURE="
          + hypr_sig
          + " hyprctl "
        )
        print(f"ui suite Hyprland instance signature: {hypr_sig}")

        for attempt in range(30):
            if machine.execute(hyprctl + "version'")[0] == 0:
                break
            time.sleep(2)
        else:
            boot_logs()
            raise Exception(f"hyprctl never answered the ui suite's Hyprland {hypr_sig}")
        print(machine.succeed(hyprctl + "monitors'"))
        print(machine.succeed(hyprctl + "rules'"))

        wayland_display = machine.succeed("basename $(ls ${runtimeDir}/wayland-* | head -1)").strip()
        print(f"ui suite Hyprland up on {wayland_display}")

        suite_env = (
          "XDG_RUNTIME_DIR=${runtimeDir} DBUS_SESSION_BUS_ADDRESS=${bus} LYS_TEST_SESSION=${runtimeDir} WAYLAND_DISPLAY="
          + wayland_display
          + " HYPRLAND_INSTANCE_SIGNATURE="
          + hypr_sig
          + " LYS_TEST_HEADLESS=1 LYS_UI_TEST_COMPOSITOR=hyprland LYS_UI_TEST_CLIENTS=${suite}/clients "
          "XCURSOR_PATH=${pkgs.vanilla-dmz}/share/icons XCURSOR_THEME=Vanilla-DMZ "
        )

        kill_test = "TestHeadlessCompositorKillExitsCleanly"
        try:
            for selection in ["-test.skip=" + kill_test, "-test.run=" + kill_test]:
                machine.succeed(
                  "su - ${user} -c 'cd ${lysGo.src}/ui/internal/headlesstest && "
                  + suite_env
                  + "${suite}/bin/headlesstest.test -test.count=1 -test.timeout=10m -test.paniconexit0 "
                  + selection
                  + " >>${log}/suite.log 2>&1'"
                )
        except Exception:
            print(machine.execute("cat ${log}/suite.log")[1])
            print(machine.execute(f"tail -n 60 ${runtimeDir}/hypr/{hypr_sig}/hyprland.log")[1])
            print(machine.execute(
              "for f in ${runtimeDir}/client-*.log; do echo \"== $f\"; "
              "grep -E 'keymap|keyboard|popover key|wire key|modifiers|demo: key' $f | head -n 16; done"
            )[1])
            print(machine.execute(f"grep -niE 'session_lock|sessionlock|unlock|refusing' ${runtimeDir}/hypr/{hypr_sig}/hyprland.log | head -n 120")[1])
            print(machine.execute("journalctl _UID=${toString uid} --no-pager | tail -n 80")[1])
            raise Exception("the ui compositor-in-the-loop suite failed on Hyprland")
        print(machine.succeed("tail -n 200 ${log}/suite.log"))
  '';
}
