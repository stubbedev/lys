self: let
  user = "alice";
  uid = 1000;
  env = "XDG_RUNTIME_DIR=/run/user/${toString uid} DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/${toString uid}/bus";
in {
  name = "a call to the vault D-Bus-activates lys-secrets under dbus-broker";
  module = {
    imports = [self.nixosModules.lys-secrets];
    services.lys-secrets.enable = true;
    services.dbus.implementation = "broker";
    users.users.${user} = {
      isNormalUser = true;
      inherit uid;
      linger = true;
    };
  };
  script = ''
    def as_user(cmd):
        return f"su - ${user} -c '${env} {cmd}'"

    machine.wait_for_unit("user@${toString uid}.service")
    machine.fail(as_user("systemctl --user is-active lys-secrets.service"))
    machine.succeed(as_user("busctl --user call dev.stubbe.lys.Vault1 /dev/stubbe/lys/Vault org.freedesktop.DBus.Peer Ping"))
    machine.wait_until_succeeds(as_user("systemctl --user is-active lys-secrets.service"))
    machine.succeed(as_user("busctl --user call org.freedesktop.secrets /org/freedesktop/secrets org.freedesktop.DBus.Peer Ping"))
    machine.succeed(as_user("systemctl --user show -p ExecStart lys-secrets.service | grep -q -- '--vault-lock-after=15m'"))

    journal = machine.succeed("journalctl -b --no-pager")
    assert "is not named after the D-Bus name" not in journal, "dbus-broker warned about an activation file's name"
  '';
}
