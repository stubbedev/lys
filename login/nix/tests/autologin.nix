{
  pkgs,
  lib,
  self,
  vm,
}: let
  password = "lys-autologin-vm";
  otherPassphrase = "other-luks-key";

  agent = pkgs.writeShellScript "lys-login-vm-autologin-agent" ''
    env | sort > /tmp/lys-login/session-env
    echo "$XDG_SESSION_ID" > /tmp/lys-login/session
    until [ -e /tmp/lys-login/session-over ]; do
      if [ -e /tmp/lys-login/lock-now ]; then
        rm -f /tmp/lys-login/lock-now
        lys-login lock > /tmp/lys-login/lock.out 2> /tmp/lys-login/lock.err
        echo $? > /tmp/lys-login/lock.rc
      fi
      sleep 0.1
    done
  '';
in
  vm.suite {
    name = "lys-login-vms-autologin";
    displayManager = "lys-login.service";
    journal = "-t lys-login";
    globalTimeout = 900;
    node = {
      imports = [
        self.nixosModules.lys-login
        self.nixosModules.lys-wm
        self.nixosModules.lys-secrets
        vm.module
      ];
      virtualisation = {
        emptyDiskImages = [1024];
        useBootLoader = true;
        useEFIBoot = true;
        mountHostNixStore = true;
      };
      boot.loader.systemd-boot.enable = true;
      boot.tmp.cleanOnBoot = true;
      environment.systemPackages = [pkgs.cryptsetup pkgs.keyutils pkgs.libsecret];
      programs.lys-wm.enable = true;
      services.lys-secrets.enable = true;
      programs.lys-login = {
        lock.enable = true;
        autologin.user = "alice";
        greeter = {
          enable = true;
          session = {
            dirs = [];
            x11Dirs = [];
            command = "${vm.lysWmSession {
              name = "autologin";
              inherit agent;
            }}";
          };
        };
      };
      users.users.alice = {
        isNormalUser = true;
        inherit password;
      };
      specialisation.luks.configuration = {
        boot.initrd.luks.devices = lib.mkVMOverride {cryptroot.device = "/dev/vdb";};
        virtualisation.rootDevice = "/dev/mapper/cryptroot";
      };
    };
    scenarios = [
      {
        name = "autologin";
        script = nodes: ''
          import datetime
          import shlex
          import time

          SECRETS = ("${password}", "${otherPassphrase}")

          def journal():
              return machine.succeed("scenario-journal -b -o cat --no-pager")

          def as_alice(command):
              return "su alice -s /bin/sh -c " + shlex.quote(f"export XDG_RUNTIME_DIR=/run/user/$(id -u alice); {command}")

          def collection(state):
              return as_alice(f"busctl --user get-property org.freedesktop.secrets /org/freedesktop/secrets/collection/login org.freedesktop.Secret.Collection Locked | grep -qx 'b {state}'")

          def await_session():
              machine.wait_until_succeeds("test -s /tmp/lys-login/session", timeout=120)
              return machine.succeed("cat /tmp/lys-login/session").strip()

          def no_greeter_before_the_session():
              log = journal()
              if "dm: autologin of alice into" not in log:
                  raise Exception("the daemon did not log alice in:\n" + log[-3000:])
              if "lys-greeter starting" in log:
                  raise Exception("a greeter came up before the autologin")

          def secrets_nowhere():
              for secret in SECRETS:
                  if secret in journal():
                      raise Exception("a passphrase is in the journal")
                  machine.succeed(f"printf '%s\\n' {secret} > /tmp/lys-login/pattern")
                  hits = machine.execute("grep -l -a -F -f /tmp/lys-login/pattern /proc/[0-9]*/cmdline /proc/[0-9]*/environ 2>/dev/null")[1].strip()
                  machine.succeed("rm /tmp/lys-login/pattern")
                  if hits:
                      raise Exception(f"a passphrase is in {hits}")

          def boot_luks(passphrase):
              machine.succeed("sync")
              machine.crash()
              machine.start()
              machine.wait_for_console_text("Please enter passphrase for disk cryptroot", timeout=datetime.timedelta(minutes=3))
              machine.send_console(passphrase + "\n")
              machine.wait_for_unit("multi-user.target")
              machine.succeed("install -D -o alice -m 0644 ${vm.lockConfig} /home/alice/.config/lys-shell/config.yaml")

          machine.wait_for_unit("lys-login.service")
          sid = await_session()
          no_greeter_before_the_session()
          if "no disk passphrase in the kernel keyring" not in journal():
              raise Exception("the worker did not say the keyring stays locked without a passphrase")
          machine.screenshot("autologin-01-autologin")
          machine.succeed("touch /tmp/lys-login/session-over")
          machine.succeed("kill -TERM $(pgrep -u alice -x lys-wm || pgrep -u alice -f 'lys-wm --config')")
          machine.wait_until_succeeds(f"! loginctl show-session {sid}", timeout=60)
          await_ready(machine, "username", 1)
          machine.screenshot("autologin-02-greeter-after-logout")
          if journal().count("dm: autologin of alice into") != 1:
              raise Exception("the autologin ran again after the logout")

          machine.succeed("echo -n ${password} | cryptsetup luksFormat -q --iter-time=1 /dev/vdb -")
          machine.succeed("echo -n ${otherPassphrase} > /tmp/other && echo -n ${password} | cryptsetup luksAddKey -q --iter-time=1 --key-file=- /dev/vdb /tmp/other && rm /tmp/other")
          machine.succeed("echo -n ${password} | cryptsetup luksOpen -q /dev/vdb cryptroot")
          machine.succeed("mkfs.ext4 -q /dev/mapper/cryptroot && cryptsetup luksClose cryptroot")
          machine.succeed("${nodes.machine.specialisation.luks.configuration.system.build.toplevel}/bin/switch-to-configuration boot")

          boot_luks("${password}")
          await_session()
          no_greeter_before_the_session()
          machine.wait_until_succeeds(collection("false"), timeout=60)
          machine.succeed(as_alice("printf lys-secret | secret-tool store --label=vm service lys-vm key k"))
          secrets_nowhere()

          boot_luks("${password}")
          await_session()
          no_greeter_before_the_session()
          if "the disk passphrase from the kernel keyring goes to the login keyring" not in journal():
              raise Exception("the worker did not say it hands the passphrase over")
          machine.wait_until_succeeds(as_alice("secret-tool lookup service lys-vm key k | grep -qx lys-secret"), timeout=60)
          machine.screenshot("autologin-03-keyring-open")
          secrets_nowhere()

          boot_luks("${otherPassphrase}")
          sid = await_session()
          no_greeter_before_the_session()
          machine.wait_until_succeeds("scenario-journal -b -u lys-login -p warning -o cat --no-pager | grep -q 'the login keyring of alice stays locked'", timeout=60)
          warnings = machine.succeed("scenario-journal -b -u lys-login -p warning -o cat --no-pager | grep 'login keyring'").strip().splitlines()
          if len(warnings) != 1:
              raise Exception(f"want one warning naming the mismatch, got {warnings}")
          if machine.execute(collection("true"))[0] != 0:
              raise Exception("a passphrase that is not the keyring's opened it")
          machine.succeed("touch /tmp/lys-login/lock-now")
          machine.wait_until_succeeds("grep -qx locked /tmp/lys-login/lock.out", timeout=60)
          machine.wait_until_succeeds("grep -q 'lock: session locked' /tmp/lys-login/lock.err", timeout=10)
          end = time.monotonic() + 60
          while machine.execute("test -e /tmp/lys-login/lock.rc")[0] != 0:
              if time.monotonic() >= end:
                  raise Exception("the lock screen did not unlock:\n" + machine.succeed("cat /tmp/lys-login/lock.err"))
              machine.send_chars("${password}\n")
              time.sleep(2)
          machine.wait_until_succeeds(as_alice("secret-tool lookup service lys-vm key k | grep -qx lys-secret"), timeout=60)
          secrets_nowhere()
          print(journal()[-3000:])
        '';
      }
    ];
  }
