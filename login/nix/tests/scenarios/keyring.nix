{
  pkgs,
  lib,
  ...
}: let
  password = "lys-login-vm";
  newPassword = "changed-password";

  session = pkgs.writeShellScript "lys-login-vm-keyring-session" ''
    echo up > /tmp/lys-login/session
    sleep infinity
  '';

  loginCollectionLocked =
    pkgs.writers.writePython3 "lys-login-vm-keyring-locked" {
      libraries = [pkgs.python3Packages.jeepney];
    } ''
      import sys

      from jeepney import DBusAddress, MessageType, new_method_call
      from jeepney.io.blocking import open_dbus_connection

      SECRETS = "org.freedesktop.secrets"
      service = DBusAddress(
          "/org/freedesktop/secrets",
          bus_name=SECRETS,
          interface="org.freedesktop.Secret.Service",
      )
      collection = DBusAddress(
          "/org/freedesktop/secrets/collection/login",
          bus_name=SECRETS,
          interface="org.freedesktop.DBus.Properties",
      )


      def call(conn, message):
          reply = conn.send_and_get_reply(message, timeout=10)
          if reply.header.message_type != MessageType.method_return:
              sys.exit(f"{message.header.fields}: {reply.body}")
          return reply.body


      with open_dbus_connection("SESSION") as conn:
          call(conn, new_method_call(service, "ReadAlias", "s", ("default",)))
          (signature, locked), = call(conn, new_method_call(
              collection,
              "Get",
              "ss",
              ("org.freedesktop.Secret.Collection", "Locked"),
          ))
      if signature != "b":
          sys.exit(f"Locked has signature {signature}, not a boolean")
      print("b true" if locked else "b false")
    '';

  helpers = user: ''
    import json

    def login(m, field, occurrence, typed):
        m.wait_for_unit("lys-login.service")
        await_ready(m, field, occurrence)
        m.send_chars(typed + "\n")
        m.wait_until_succeeds("test -s /tmp/lys-login/session", timeout=60)

    def logout(m):
        m.succeed("rm /tmp/lys-login/session")
        m.succeed("loginctl terminate-user ${user}")
        m.wait_until_succeeds("! pgrep -u ${user}", timeout=60)

    def warnings(m):
        rows = m.succeed("scenario-journal -b -u lys-login -p warning -o json --no-pager").splitlines()
        return [r for r in map(json.loads, rows) if "stays locked" in r["MESSAGE"]]

    def await_warning(m, reason):
        m.wait_until_succeeds("scenario-journal -b -u lys-login -p warning -o cat --no-pager | grep -q 'stays locked'", timeout=100)
        found = warnings(m)
        if len(found) != 1:
            raise Exception(f"want one warning, got {found}")
        row = found[0]
        if reason not in row["MESSAGE"]:
            raise Exception(f"the warning does not say {reason!r}: {row['MESSAGE']}")
        if row["_SYSTEMD_UNIT"] != "lys-login.service" or row["PRIORITY"] != "4":
            raise Exception(f"the warning is not at warning in the unit's log: {row}")
        print("warning:", row["MESSAGE"])

    def collection_locked(uid):
        return (
            f"su ${user} -s /bin/sh -c 'DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/{uid}/bus ${loginCollectionLocked}'"
        )
  '';

  scenario = {
    name,
    user,
    keyring,
    script,
  }: {
    inherit name;
    specialisation = name;
    sessions.${user} = session;
    module = {
      users.users.${user} = {
        isNormalUser = true;
        inherit password;
      };
      specialisation.${name}.configuration.services = {
        lys-secrets.enable = lib.mkForce false;
        gnome.gnome-keyring.enable = keyring;
      };
    };
    script = _: helpers user + script;
  };
in [
  (scenario {
    name = "keyring";
    user = "kim";
    keyring = true;
    script = ''
      login(machine, "username", 1, "kim\n${password}")
      uid = machine.succeed("id -u kim").strip()
      machine.fail("grep -q pam_gnome_keyring /etc/pam.d/lys-login")
      control = f"/run/user/{uid}/keyring/control"

      machine.wait_until_succeeds(f"test -S {control}", timeout=60)
      machine.wait_until_succeeds("test -s /home/kim/.local/share/keyrings/login.keyring", timeout=30)
      if warnings(machine):
          raise Exception(f"the first login warned: {warnings(machine)}")

      logout(machine)
      login(machine, "password", 1, "${password}")
      machine.wait_until_succeeds(f"test -S {control}", timeout=60)
      machine.wait_until_succeeds(collection_locked(uid) + " | grep -qx 'b false'", timeout=30)
      if warnings(machine):
          raise Exception(f"the login with the keyring's password warned: {warnings(machine)}")
      machine.screenshot("keyring-session")

      logout(machine)
      machine.succeed("echo 'kim:${newPassword}' | chpasswd")
      login(machine, "password", 2, "${newPassword}")
      await_warning(machine, "denied (keyring password differs from login password)")
      machine.wait_until_succeeds(f"test -S {control}", timeout=30)
      machine.succeed(collection_locked(uid) + " | grep -qx 'b true'")
    '';
  })
  (scenario {
    name = "no_keyring";
    user = "noel";
    keyring = false;
    script = ''
      login(machine, "username", 1, "noel\n${password}")
      await_warning(machine, "neither lys-secrets nor gnome-keyring is running")
    '';
  })
]
