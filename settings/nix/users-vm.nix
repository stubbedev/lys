{
  self,
  pkgs,
}: let
  helper = "${self.packages.${pkgs.stdenv.hostPlatform.system}.lys-settings}/libexec/lys-users-helper";
  openssl = "${pkgs.openssl}/bin/openssl";
in {
  name = "lys-users-helper changes accounts through pkexec with no AccountsService";
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

    def users_run(user, cls, op, request):
        body = shlex.quote(json.dumps({"op": op, "request": request}))
        return machine.execute(f"printf %s {body} | runuser -u {user} -- pkexec --disable-internal-agent ${helper} {cls}")

    def users_change(user, cls, op, request):
        status, out = users_run(user, cls, op, request)
        if status != 0:
            raise Exception(f"{op} as {user} exited {status}: {out}")

    def users_refused(user, cls, op, request, kind):
        status, out = users_run(user, cls, op, request)
        if status != 1 or json.loads(out)["kind"] != kind:
            raise Exception(f"{op} as {user} exited {status} with {out!r}, want a {kind} refusal")

    def users_gecos(user):
        return machine.succeed(f"getent passwd {user} | cut -d: -f5").strip()

    def users_admin(user):
        return "wheel" in machine.succeed(f"id -nG {user}").split()

    def users_shadow(user):
        return machine.succeed(f"getent shadow {user} | cut -d: -f2").strip()

    machine.wait_for_unit("multi-user.target")
    machine.fail("systemctl cat accounts-daemon.service")
    machine.succeed("test ! -e /run/current-system/sw/share/dbus-1/system-services/org.freedesktop.Accounts.service")

    users_change("ursula", "self", "rename", {"realName": "Ursula K. Le Guin"})
    assert users_gecos("ursula") == "Ursula K. Le Guin", users_gecos("ursula")

    users_change("ursula", "self", "picture", {"picture": USERS_PNG})
    assert machine.succeed("stat -c %a /var/lib/lys/avatars/ursula").strip() == "644"
    assert machine.succeed("stat -c %a /var/lib/lys/avatars").strip() == "755"
    assert machine.succeed("head -c 8 /var/lib/lys/avatars/ursula | od -An -tx1").split() == ["89", "50", "4e", "47", "0d", "0a", "1a", "0a"]

    users_hash = machine.succeed("${openssl} passwd -6 zoe-vm-password").strip()
    users_change("ursula", "others", "create", {"login": "zoe", "realName": "Zoë", "kind": "admin", "hash": users_hash})
    assert users_gecos("zoe") == "Zoë" and users_admin("zoe") and users_shadow("zoe") == users_hash
    machine.succeed("test -d /home/zoe")
    users_refused("ursula", "others", "create", {"login": "zoe", "realName": "", "kind": "standard", "hash": users_hash}, "taken")

    users_other = machine.succeed("${openssl} passwd -6 victor-vm-password").strip()
    users_change("ursula", "others", "edit", {"login": "victor", "realName": "Victor Frankenstein", "kind": "admin", "hash": users_other})
    assert users_gecos("victor") == "Victor Frankenstein" and users_admin("victor") and users_shadow("victor") == users_other
    users_change("ursula", "others", "edit", {"login": "victor", "kind": "standard"})
    assert not users_admin("victor")

    users_change("ursula", "others", "delete", {"login": "zoe", "removeHome": True})
    machine.fail("getent passwd zoe")
    machine.fail("test -e /home/zoe")

    users_before = machine.succeed("getent passwd victor; getent shadow victor; id -nG victor")
    users_status, users_out = users_run("mallory", "others", "edit", {"login": "victor", "realName": "Pwned"})
    if users_status != 127:
        raise Exception(f"pkexec let mallory run the helper: {users_status} {users_out}")
    users_status, users_out = users_run("mallory", "self", "rename", {"realName": "Mallory"})
    if users_status not in (126, 127):
        raise Exception(f"pkexec let mallory rename without authenticating: {users_status} {users_out}")
    users_refused("ursula", "others", "delete", {"login": "root"}, "not-a-person")
    users_refused("ursula", "others", "delete", {"login": "ursula"}, "own-account")
    users_refused("ursula", "others", "edit", {"login": "victor", "realName": "x:0:0"}, "invalid")
    users_refused("ursula", "self", "delete", {"login": "victor"}, "invalid")
    users_refused("ursula", "others", "edit", {"login": "victor", "hash": "victor:$6$x\nroot:"}, "invalid")
    assert machine.succeed("getent passwd victor; getent shadow victor; id -nG victor") == users_before
    machine.succeed("getent passwd root")
  '';
}
