{pkgs, ...}: let
  user = "rita";
  password = "lys-rekey-vm-old";
  newPassword = "Lys-rekey-VM-new-2026";
in {
  name = "rekey";
  specialisation = "rekey";
  sessions.${user} = pkgs.writeShellScript "lys-login-vm-rekey-session" ''
    echo "$XDG_SESSION_ID" > /tmp/lys-login/session
    until [ -e /tmp/lys-login/end-session ]; do sleep 0.2; done
    rm -f /tmp/lys-login/end-session /tmp/lys-login/session
  '';
  module = {
    environment.systemPackages = [pkgs.libsecret];
    specialisation.rekey.configuration.programs.lys-login.greeter.settings.login.greeter.show-user-list = false;
    users.users.${user} = {
      isNormalUser = true;
      inherit password;
    };
  };
  script = _: ''
    def journal():
        return machine.succeed("scenario-journal -b -o cat --no-pager")

    def as_user(command):
        return f"su ${user} -s /bin/sh -c 'export XDG_RUNTIME_DIR=/run/user/$(id -u ${user}); {command}'"

    def unlocked():
        machine.wait_until_succeeds(as_user("busctl --user get-property org.freedesktop.secrets /org/freedesktop/secrets/collection/login org.freedesktop.Secret.Collection Locked | grep -qx \"b false\""), timeout=60)

    def logout():
        machine.succeed("touch /tmp/lys-login/end-session")
        machine.wait_until_succeeds("! test -e /tmp/lys-login/session", timeout=60)

    machine.wait_for_unit("lys-login.service")
    await_ready(machine, "username", 1)
    machine.send_chars("${user}\n${password}\n")
    machine.wait_until_succeeds("test -s /tmp/lys-login/session", timeout=90)
    unlocked()
    machine.succeed(as_user("printf lys-secret | secret-tool store --label=vm service lys-vm key k"))
    logout()

    machine.succeed("chage -d 0 ${user}")
    await_ready(machine, "password", 1)
    machine.send_chars("${password}\n")
    for answer in ("${password}", "${newPassword}", "${newPassword}"):
        machine.sleep(4)
        machine.send_chars(answer + "\n")
    machine.wait_until_succeeds("test -s /tmp/lys-login/session", timeout=90)
    if "password changed for ${user}" not in journal():
        raise Exception("PAM did not change ${user}'s password")
    machine.wait_until_succeeds("scenario-journal -b -o cat --no-pager | grep -q 'lys-secrets moved the login collection to the new password'", timeout=60)
    unlocked()
    machine.succeed(as_user("secret-tool lookup service lys-vm key k | grep -qx lys-secret"))
    logout()

    await_ready(machine, "password", 2)
    machine.send_chars("${newPassword}\n")
    machine.wait_until_succeeds("test -s /tmp/lys-login/session", timeout=90)
    unlocked()
    machine.succeed(as_user("secret-tool lookup service lys-vm key k | grep -qx lys-secret"))
    if "login keyring of ${user} stays locked" in journal():
        raise Exception("a login warned that the keyring stayed locked")
    print(journal()[-3000:])
  '';
}
