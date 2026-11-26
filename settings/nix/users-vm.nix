{
  self,
  pkgs,
}: let
  gdbus = "${pkgs.glib.bin}/bin/gdbus";
  openssl = "${pkgs.openssl}/bin/openssl";
in {
  name = "lys-users changes accounts over the system bus with no AccountsService or shadow tools";
  module = {
    imports = [self.nixosModules.lys-settings];
    programs.lys-settings.enable = true;
    services.accounts-daemon.enable = false;
    users.users = {
      ursula = {
        isNormalUser = true;
        description = "Ursula";
      };
      victor = {
        isNormalUser = true;
        description = "Victor";
      };
      mallory = {
        isNormalUser = true;
        description = "Mallory";
      };
    };
    security.polkit.extraConfig = ''
      polkit.addRule(function(action, subject) {
        if (action.id.indexOf("dev.stubbe.lys.settings.users.") == 0 && subject.user == "ursula") {
          return polkit.Result.YES;
        }
      });
    '';
  };
  script = ''
    import json
    import shlex

    USERS_PNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
    USERS_ERROR = "dev.stubbe.lys.Users1.Error."

    def users_run(user, op, request):
        body = json.dumps({"op": op, "request": request}).encode()
        arg = shlex.quote("@ay [" + ", ".join(str(b) for b in body) + "]")
        return machine.execute(f"runuser -u {user} -- ${gdbus} call --system --dest dev.stubbe.lys.Users1 --object-path /dev/stubbe/lys/Users1 --method dev.stubbe.lys.Users1.Change {arg} 2>&1")

    def users_change(user, op, request):
        status, out = users_run(user, op, request)
        if status != 0:
            raise Exception(f"{op} as {user} exited {status}: {out}")

    def users_refused(user, op, request, error):
        status, out = users_run(user, op, request)
        if status == 0 or f"GDBus.Error:{USERS_ERROR}{error}:" not in out:
            raise Exception(f"{op} as {user} exited {status} with {out!r}, want {USERS_ERROR}{error}")

    def users_field(db, user, field):
        return machine.succeed(f"getent {db} {user} | cut -d: -f{field}").strip()

    def users_admin(user):
        return "wheel" in machine.succeed(f"id -nG {user}").split()

    def users_files():
        return machine.succeed("cat /etc/passwd /etc/shadow /etc/group; stat -c '%n %U:%G %a' /etc/passwd /etc/shadow /etc/group")

    machine.wait_for_unit("multi-user.target")
    machine.fail("systemctl cat accounts-daemon.service")
    machine.succeed("test ! -e /run/current-system/sw/share/dbus-1/system-services/org.freedesktop.Accounts.service")
    shadow_owner = machine.succeed("stat -c '%U:%G %a' /etc/shadow").strip()

    users_change("ursula", "picture", {"picture": USERS_PNG})
    machine.succeed("systemctl is-active lys-users.service")
    assert machine.succeed("stat -c %a /var/lib/lys/avatars/ursula").strip() == "644"
    assert machine.succeed("stat -c %a /var/lib/lys/avatars").strip() == "755"
    assert machine.succeed("head -c 8 /var/lib/lys/avatars/ursula | od -An -tx1").split() == ["89", "50", "4e", "47", "0d", "0a", "1a", "0a"]

    declared_before = users_files()
    users_refused("ursula", "rename", {"realName": "Ursula K. Le Guin"}, "Declarative")
    users_refused("ursula", "edit", {"login": "victor", "realName": "Victor Frankenstein"}, "Declarative")
    users_refused("ursula", "edit", {"login": "victor", "kind": "admin"}, "Declarative")
    users_refused("ursula", "delete", {"login": "victor"}, "Declarative")
    assert users_files() == declared_before

    users_hash = machine.succeed("${openssl} passwd -6 zoe-vm-password").strip()
    users_change("ursula", "create", {"login": "zoe", "realName": "Zoë", "kind": "admin", "hash": users_hash})
    assert users_field("passwd", "zoe", 5) == "Zoë" and users_admin("zoe") and users_field("shadow", "zoe", 2) == users_hash
    assert users_field("passwd", "zoe", 7) == machine.succeed(". /etc/default/useradd && echo $SHELL").strip()
    assert machine.succeed("stat -c %U /home/zoe").strip() == "zoe"
    assert machine.succeed("stat -c '%U:%G %a' /etc/shadow").strip() == shadow_owner
    machine.fail("ls -a /etc | grep -F .lys-")
    machine.succeed("test -e /etc/.pwd.lock")
    users_refused("ursula", "create", {"login": "zoe", "realName": "", "kind": "standard", "hash": users_hash}, "Taken")

    users_other = machine.succeed("${openssl} passwd -6 zoe-second-password").strip()
    users_change("ursula", "edit", {"login": "zoe", "realName": "Zoë Washburne", "kind": "standard", "hash": users_other})
    assert users_field("passwd", "zoe", 5) == "Zoë Washburne" and not users_admin("zoe") and users_field("shadow", "zoe", 2) == users_other
    victor_hash = machine.succeed("${openssl} passwd -6 victor-vm-password").strip()
    users_change("ursula", "edit", {"login": "victor", "hash": victor_hash})
    assert users_field("shadow", "victor", 2) == victor_hash

    machine.succeed("/run/current-system/activate")
    assert users_field("passwd", "zoe", 5) == "Zoë Washburne", "activation dropped the account lys-users made"
    assert users_field("shadow", "victor", 2) == victor_hash, "activation reset the password lys-users set"

    users_before = users_files()
    users_refused("mallory", "edit", {"login": "zoe", "realName": "Pwned"}, "NotAuthorized")
    users_refused("mallory", "picture", {"picture": USERS_PNG}, "NotAuthorized")
    users_refused("ursula", "delete", {"login": "root"}, "NotAPerson")
    users_refused("ursula", "delete", {"login": "ursula"}, "OwnAccount")
    users_refused("ursula", "edit", {"login": "zoe", "realName": "x:0:0"}, "Invalid")
    users_refused("ursula", "edit", {"login": "zoe", "hash": "zoe:$6$x\nroot:"}, "Invalid")
    assert users_files() == users_before

    users_change("ursula", "delete", {"login": "zoe", "removeHome": True})
    machine.fail("getent passwd zoe")
    machine.fail("getent group zoe")
    machine.fail("test -e /home/zoe")
    machine.fail("grep -w zoe /etc/passwd /etc/shadow /etc/group")
    machine.succeed("getent passwd root")
  '';
}
