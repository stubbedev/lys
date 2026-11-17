{self, ...}: {
  perSystem = {
    lib,
    lysGo,
    pkgs,
    self',
    ...
  }: let
    testBinaries = binaries:
      self'.packages.lys-splash.overrideAttrs {
        pname = "lys-splash-tests";
        goTests = map (pkg: "splash/${pkg}") (lib.attrValues binaries);
        outputs = ["out"];
        postInstall = "";
        buildPhase = ''
          runHook preBuild
          ${lib.concatStrings (lib.mapAttrsToList (name: pkg: "go test -c -o ${name}.test ./splash/${pkg}\n") binaries)}
          runHook postBuild
        '';
        doCheck = false;
        installPhase = ''
          runHook preInstall
          ${lib.concatStrings (lib.mapAttrsToList (name: _: "install -Dm755 ${name}.test $out/bin/${name}.test\n") binaries)}
          runHook postInstall
        '';
      };
    module = self.nixosModules.lys-splash;
    splash = self'.packages.lys-splash;
    wmPackage = self'.packages.lys-wm;
    tests = testBinaries {
      kms = "internal/kms";
      lys-splash = "cmd/lys-splash";
    };
    drmTools = pkgs.callPackage ./drm-tools.nix {};
    greeter = import ./greeter.nix {inherit pkgs splash self wmPackage drmTools;};
    plymouthLink = pkgs.runCommand "lys-splash-plymouth-link" {} ''
      mkdir -p $out/bin
      ln -s ${lib.getExe splash} $out/bin/plymouth
    '';
    passphrase = "lys-e2e-passphrase";

    bootSplash = {
      imports = [module];
      boot.lys-splash = {
        enable = true;
        package = splash;
      };
    };
    accentSplash = {
      imports = [bootSplash];
      boot.lys-splash.theme.accent = "#ffb454";
      environment.systemPackages = [plymouthLink];
      virtualisation.qemu.virtioKeyboard = false;
    };
    rebootLogsOnConsole = {
      systemd.services.lys-splash-reboot.serviceConfig.StandardError = "file:/dev/kmsg";
    };
    noTty1 = {
      systemd.services."getty@tty1".enable = false;
      systemd.services."autovt@tty1".enable = false;
    };
    noDisplay = {
      virtualisation.graphics = false;
      virtualisation.qemu.options = ["-vga none"];
    };
    luks = disks: {
      imports = [bootSplash];
      virtualisation = {
        emptyDiskImages = map (_: 512) disks;
        useBootLoader = true;
        useEFIBoot = true;
        mountHostNixStore = true;
        qemu.virtioKeyboard = false;
      };
      boot.loader.systemd-boot.enable = true;
      environment.systemPackages = [pkgs.cryptsetup];
      specialisation.luks.configuration = {
        boot.kernelParams = ["systemd.journald.forward_to_console=1"];
        boot.initrd.luks.devices = lib.mkVMOverride (lib.listToAttrs (lib.imap0 (i: d: lib.nameValuePair "crypt${toString i}" {device = d;}) disks));
        virtualisation.rootDevice = "/dev/mapper/crypt0";
        virtualisation.fileSystems = lib.optionalAttrs (builtins.length disks > 1) {
          "/data" = {
            device = "/dev/mapper/crypt1";
            fsType = "ext4";
          };
        };
      };
    };

    nodes = {
      boot = {
        imports = [accentSplash rebootLogsOnConsole];
        virtualisation.qemu.options = ["-vga none" "-device virtio-gpu-pci" "-action shutdown=pause"];
        boot.initrd.kernelModules = ["virtio_gpu"];
        console.keyMap = "de-latin1";
        systemd.services.greeter = {
          wantedBy = ["multi-user.target"];
          after = ["lys-splash-start.service" "getty@tty1.service" "lys-splash-quit.service" "systemd-user-sessions.service"];
          conflicts = ["lys-splash-quit.service" "getty@tty1.service"];
          path = [pkgs.coreutils pkgs.systemd];
          serviceConfig = {
            Type = "notify";
            NotifyAccess = "all";
            TimeoutStartSec = "infinity";
            ExecStartPre = [
              "${pkgs.runtimeShell} -c 'until test -e /run/greeter-go; do sleep 0.1; done'"
              "-${lib.getExe splash} deactivate"
            ];
            ExecStart = "${pkgs.runtimeShell} -c 'exec 3<>/dev/dri/card0; systemd-notify --ready; exec sleep infinity'";
            ExecStartPost = "-${lib.getExe splash} quit --retain-splash";
          };
        };
      };
      text = {
        imports = [accentSplash noTty1 noDisplay];
        systemd.services.lys-splash-quit.wantedBy = lib.mkForce [];
      };
      greeter = {
        imports = [bootSplash greeter];
        boot.initrd.kernelModules = ["virtio_gpu"];
        virtualisation.qemu.options = ["-vga none" "-device virtio-gpu-pci"];
      };
      vkms = {
        imports = [greeter noDisplay];
        boot.kernelPackages = pkgs.linuxPackages_latest;
        boot.kernelModules = ["vkms"];
        boot.extraModprobeConfig = "options vkms create_default_dev=0";
        boot.kernelParams = ["video=Virtual-2:panel_orientation=right_side_up"];
        environment.systemPackages = [splash];
        systemd.services.vkms-outputs = {
          wantedBy = ["multi-user.target"];
          after = ["systemd-modules-load.service" "sys-kernel-config.mount"];
          requires = ["sys-kernel-config.mount"];
          serviceConfig = {
            Type = "oneshot";
            RemainAfterExit = true;
          };
          script = ''
            set -euo pipefail
            d=/sys/kernel/config/vkms/splash
            mkdir "$d"
            for i in 1 2; do
              mkdir "$d/planes/plane$i" "$d/crtcs/crtc$i" "$d/encoders/encoder$i" "$d/connectors/connector$i"
              echo 1 > "$d/planes/plane$i/type"
              ln -s "$d/crtcs/crtc$i" "$d/planes/plane$i/possible_crtcs"
              ln -s "$d/crtcs/crtc$i" "$d/encoders/encoder$i/possible_crtcs"
              ln -s "$d/encoders/encoder$i" "$d/connectors/connector$i/possible_encoders"
            done
            echo 1 > "$d/enabled"
          '';
        };
      };
      luks = {
        imports = [(luks ["/dev/vdb" "/dev/vdc"]) greeter rebootLogsOnConsole];
        virtualisation.qemu.options = ["-vga none" "-device virtio-vga" "-action shutdown=pause"];
        boot.kernelParams = ["modprobe.blacklist=virtio_gpu"];
        systemd.services.native-gpu = {
          wantedBy = ["multi-user.target"];
          before = ["greeter-gate.service"];
          serviceConfig = {
            Type = "oneshot";
            RemainAfterExit = true;
            ExecStart = "${pkgs.kmod}/bin/modprobe virtio_gpu";
          };
        };
      };
      headless = {
        imports = [(luks ["/dev/vdb"]) noDisplay];
      };
      firmware = {
        imports = [noTty1];
        virtualisation.useEFIBoot = true;
        virtualisation.qemu.options = ["-vga none" "-device virtio-vga"];
        virtualisation.qemu.virtioKeyboard = false;
        boot.kernelParams = ["modprobe.blacklist=virtio_gpu" "fbcon=map:1"];
        systemd.suppressedSystemUnits = [
          "systemd-ask-password-console.path"
          "systemd-ask-password-console.service"
          "systemd-ask-password-wall.path"
          "systemd-ask-password-wall.service"
        ];
        environment.systemPackages = [splash];
        systemd.tmpfiles.rules = [
          "L+ /run/plymouth-link/plymouth - - - - ${splash}/bin/lys-splash"
          "L+ /run/plymouth-link/plymouthd - - - - ${splash}/bin/lys-splash"
        ];
      };
    };

    daemon = ''
      machine.wait_for_unit("multi-user.target")
      assert "virtio-keyboard" not in machine.send_monitor_command("info qtree"), "machine types through QEMU's virtio keyboard, which drops keys while the guest probes it"
      machine.succeed("systemd-run --no-ask-password --unit=splash -p Type=notify -p KillMode=mixed lys-splash --mode=boot --pid-file=/run/lys-splash/pid")
      machine.succeed("test -s /run/lys-splash/pid")
      machine.succeed("lys-splash --ping")
      machine.succeed("/run/plymouth-link/plymouth --ping")
      machine.succeed("lys-splash --has-active-vt")
      machine.succeed("lys-splash display-message --text='Starting Disk Encryption'")
      machine.succeed("systemd-run --no-ask-password --unit=ask -p StandardOutput=file:/tmp/answer lys-splash ask-for-password --prompt='Disk password'")
      machine.sleep(1)
      screenshot(machine, "prompt")
      machine.send_chars("hunter2\n")
      machine.wait_until_succeeds("test \"$(cat /tmp/answer)\" = hunter2")
      machine.succeed("test \"$(lys-splash ask-for-password --command='sh -c cat' 2>&1)\" = hunter2")
      machine.succeed("systemd-run --no-ask-password --unit=agent-request -p StandardOutput=file:/tmp/agent systemd-ask-password --no-tty 'Root disk:'")
      machine.wait_until_succeeds("ls /run/systemd/ask-password/ask.*")
      machine.succeed("systemd-run --no-ask-password --unit=agent systemd-tty-ask-password-agent --plymouth --query")
      machine.sleep(1)
      machine.send_chars("s3cret\n")
      machine.wait_until_succeeds("test \"$(cat /tmp/agent)\" = s3cret")
      machine.succeed("lys-splash quit --retain-splash")
      machine.wait_until_fails("lys-splash --ping")
      machine.fail("test -e /run/lys-splash/pid")
      screenshot(machine, "retained")
      rc, _ = machine.execute("lys-splash --ping")
      assert rc == 3, f"--ping with no daemon exited {rc}, want 3"
      machine.succeed("echo 'colour: not-a-field' > /tmp/bad-theme.yaml")
      machine.succeed("/run/plymouth-link/plymouthd --mode=boot --theme=/tmp/bad-theme.yaml")
      machine.succeed("lys-splash --ping")
      machine.succeed("dmesg | grep 'lys-splash.*falling back to the default theme'")
      machine.wait_until_succeeds("journalctl -b -t lys-splash -p warning -g 'falling back to the default theme'", timeout=10)
      machine.succeed("lys-splash quit")
      machine.wait_until_fails("lys-splash --ping", timeout=10)
    '';

    vms = group: scenarios: let
      used = lib.unique (lib.concatMap (s: lib.attrValues s.on) scenarios);
      fn = s: "scenario_${lib.replaceStrings ["-"] ["_"] s.name}";
      indent = text: lib.concatMapStrings (line: lib.optionalString (line != "") "    ${line}" + "\n") (lib.splitString "\n" text);
      body = s: indent ((s.setup or "") + "\n" + (s.script or (builtins.readFile s.file)));
      def = s: "def ${fn s}(${lib.concatStringsSep ", " (lib.attrNames s.on)}):\n${body s}\n\n";
      call = s: "run(${builtins.toJSON s.name}, ${fn s}, ${lib.concatStringsSep ", " (lib.attrValues s.on)})\n";
    in
      assert lib.assertMsg (lib.allUnique (map (s: s.name) scenarios)) "splash vms-${group}: scenario names repeat";
        pkgs.testers.runNixOSTest {
          name = "lys-splash-vms-${group}";
          globalTimeout = 900;
          enableOCR = lib.any (s: s.ocr or false) scenarios;
          extraPythonPackages = p: [p.pillow];
          nodes = lib.getAttrs used nodes;
          testScript =
            ''
              import traceback

              kms_test = "${tests}/bin/kms.test"
              splash_test = "${tests}/bin/lys-splash.test"
              units = "${splash}/lib/systemd/system"
              plymouth = "${lib.getExe' pkgs.plymouth "plymouth"}"
              passphrase = "${passphrase}"
            ''
            + builtins.readFile ./screens.py
            + builtins.readFile ./stages.py
            + ''

              failures = []


              def run(name, scenario, *machines):
                  print(f"=== scenario {name} on {', '.join(m.name for m in machines)}")
                  try:
                      scenario(*machines)
                  except Exception as e:
                      traceback.print_exc()
                      failures.append(f"scenario {name} failed: {e}")


            ''
            + lib.concatMapStrings def scenarios
            + "start_all()\n"
            + lib.concatMapStrings call scenarios
            + ''
              if failures:
                  raise Exception("\n".join(failures))
            '';
        };
  in {
    checks =
      {
        package = self'.packages.lys-splash;
        plymouth-client = self'.packages.lys-splash.overrideAttrs (old: {
          pname = "lys-splash-plymouth-client";
          goTests = ["splash/internal/server"];
          env = old.env // {LYS_SPLASH_PLYMOUTH = lib.getExe' pkgs.plymouth "plymouth";};
          doCheck = true;
          checkPhase = lysGo.test ["-run=^TestPlymouthClient$" "-v" "./splash/internal/server"];
        });
        units = import ./units-check.nix {inherit pkgs splash;};
        module = import ./module-check.nix {inherit pkgs module;};
      }
      // lib.optionalAttrs pkgs.stdenv.hostPlatform.isx86_64 {
        vms-greeter = vms "greeter" [
          {
            name = "boot";
            on = {
              machine = "boot";
              text = "text";
            };
            file = ./vm-boot.py;
            ocr = true;
          }
          {
            name = "greeter";
            on.machine = "greeter";
            file = ./vm-greeter.py;
          }
          {
            name = "kms";
            on.machine = "vkms";
            file = ./vm-kms.py;
          }
          {
            name = "greeter-outputs";
            on.machine = "vkms";
            setup = "no_splash(machine)";
            file = ./vm-greeter-outputs.py;
          }
          {
            name = "journal";
            on.machine = "vkms";
            file = ./vm-journal.py;
          }
        ];
        vms-luks = vms "luks" [
          {
            name = "e2e";
            on.machine = "luks";
            file = ./vm-e2e.py;
          }
        ];
        vms-luks-headless = vms "luks-headless" [
          {
            name = "e2e-headless";
            on.machine = "headless";
            file = ./vm-e2e-headless.py;
          }
        ];
        vms-firmware = vms "firmware" [
          {
            name = "kms-switch";
            on.machine = "firmware";
            file = ./vm-kms-switch.py;
          }
          {
            name = "daemon";
            on.machine = "firmware";
            setup = ''
              no_splash(machine)
              machine.succeed("modprobe virtio_gpu", "udevadm settle --timeout=10")
            '';
            script = daemon;
          }
        ];
      };
  };
}
