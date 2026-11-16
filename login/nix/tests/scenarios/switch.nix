{
  pkgs,
  lib,
  vm,
  ...
}: let
  users = ["carl" "dana"];
  password = "lys-switch-vm";

  agent = pkgs.writeShellScript "lys-login-vm-switch-agent" ''
    me=$(id -un)
    echo "$XDG_SESSION_ID $XDG_VTNR" > "/tmp/lys-login/$me-session"
    until [ -e /tmp/lys-login/stop-agents ]; do
      if [ -e "/tmp/lys-login/$me-lock" ]; then
        rm -f "/tmp/lys-login/$me-lock"
        lys-login lock > "/tmp/lys-login/$me-lock.out" 2> "/tmp/lys-login/$me-lock.err" &
      fi
      sleep 0.1
    done
  '';

  session = vm.lysWmSession {
    name = "switch";
    inherit agent;
  };
in {
  name = "switch";
  sessions = lib.genAttrs users (_: session);
  module = {
    users.users = lib.genAttrs users (_: {
      isNormalUser = true;
      inherit password;
    });
  };
  script = nodes: ''
    import time

    UPGRADE = "${nodes.machine.systemd.services.lys-login.serviceConfig.ExecReload}"

    def journal():
        return machine.succeed("scenario-journal -b -u lys-login -o cat --no-pager")

    def active_vt():
        return int(machine.succeed("cat /sys/class/tty/tty0/active").strip().removeprefix("tty"))

    def await_vt(vt, what):
        end = time.monotonic() + 60
        while active_vt() != vt:
            if time.monotonic() >= end:
                raise Exception(f"{what}: VT {active_vt()} shows, want {vt}")
            time.sleep(0.3)

    def session_of(user):
        machine.wait_until_succeeds(f"test -s /tmp/lys-login/{user}-session", timeout=90)
        sid, vt = machine.succeed(f"cat /tmp/lys-login/{user}-session").split()
        return sid, int(vt)

    def workers():
        return sorted(int(p) for p in machine.succeed("pgrep -f '^/nix/store/[^ ]*/lys-login worker session' || true").split())

    def greeters():
        return machine.execute("pgrep -u lys-greeter -f 'lys-login greet'")[0] == 0

    def as_user(user, command):
        uid = machine.succeed(f"id -u {user}").strip()
        return f"su {user} -s /bin/sh -c 'XDG_RUNTIME_DIR=/run/user/{uid} {command}'"

    def switch_user(user, greeters):
        machine.succeed("rm -f /var/lib/lys-greeter/last-user")
        machine.succeed(as_user(user, "lys-login switch-user"))
        await_ready(machine, "username", greeters)

    def pick(user, secret):
        machine.send_chars(f"{user}\n{secret}\n")

    def lock(user, sid):
        machine.succeed(f"touch /tmp/lys-login/{user}-lock")
        machine.wait_until_succeeds(f"grep -qx locked /tmp/lys-login/{user}-lock.out", timeout=60)
        machine.wait_until_succeeds(f"[ \"$(loginctl show-session {sid} -P LockedHint)\" = yes ]", timeout=30)

    def locker(user):
        return int(machine.succeed(f"pgrep -u {user} -f 'lys-login lock$' | head -n 1").strip())

    def alive(pid, what):
        if machine.execute(f"test -e /proc/{pid}")[0] != 0:
            raise Exception(f"{what} (pid {pid}) is gone")

    machine.wait_for_unit("lys-login.service")
    for user in ("carl", "dana"):
        machine.succeed(f"install -D -o {user} -m 0644 ${vm.lockConfig} /home/{user}/.config/lys-shell/config.yaml")
    await_ready(machine, "username", 1)
    machine.send_chars("carl\n${password}\n")
    carl_sid, carl_vt = session_of("carl")
    await_vt(carl_vt, "carl's session")
    carl_worker = workers()
    lock("carl", carl_sid)
    carl_lock = locker("carl")

    switch_user("carl", 2)
    if "a greeter for another user on VT" not in journal():
        raise Exception("the daemon did not say it showed a greeter for another user")
    if active_vt() == carl_vt:
        raise Exception("the greeter for another user shows on carl's VT")
    pick("dana", "${password}")
    dana_sid, dana_vt = session_of("dana")
    if dana_vt == carl_vt:
        raise Exception("dana's session runs on carl's VT")
    await_vt(dana_vt, "dana's session")
    alive(carl_lock, "carl's lock screen")
    if machine.succeed(f"loginctl show-session {carl_sid} -P State").strip() not in ("online", "active"):
        raise Exception("carl's session stopped while dana logged in")
    machine.screenshot("switch-01-dana")
    two = workers()
    if len(two) != 2:
        raise Exception(f"{len(two)} session workers run, want carl's and dana's")

    switch_user("dana", 3)
    pick("carl", "anything")
    machine.wait_until_succeeds("scenario-journal -b -u lys-login -o cat --no-pager | grep -q 'carl already has a session'", timeout=60)
    await_vt(carl_vt, "the switch back to carl")
    if workers() != two:
        raise Exception(f"picking carl started a worker for him: {workers()}, were {two}")
    if "login of carl: " in journal().split("carl already has a session")[-1]:
        raise Exception("carl was authenticated again")
    alive(carl_lock, "carl's lock screen")
    machine.screenshot("switch-02-back-to-carl-locked")

    daemon = int(machine.succeed("systemctl show -P MainPID lys-login.service"))
    machine.succeed(UPGRADE)
    if int(machine.succeed("systemctl show -P MainPID lys-login.service")) != daemon:
        raise Exception("the upgrade changed the daemon's pid")
    if "dm: upgraded in place from" not in journal():
        raise Exception("the upgrade is not in the journal")
    for user in ("carl", "dana"):
        if f"dm: following the session of {user}" not in journal():
            raise Exception(f"the upgraded daemon does not follow {user}'s session")
    if workers() != two:
        raise Exception(f"the upgrade changed the session workers: {workers()}, were {two}")

    machine.succeed(f"chvt {dana_vt}")
    await_vt(dana_vt, "dana's session again")
    dana_worker = [p for p in two if p not in carl_worker][0]
    compositor = int(machine.succeed(f"pgrep -P {dana_worker} | head -n 1").strip())
    machine.succeed(f"kill -TERM {compositor}")
    machine.wait_until_succeeds(f"! test -e /proc/{dana_worker}", timeout=30)
    await_vt(carl_vt, "dana's logout")
    time.sleep(2)
    if greeters():
        raise Exception("dana's logout showed a greeter instead of carl's session")
    alive(carl_lock, "carl's lock screen")

    machine.succeed("rm /tmp/lys-login/dana-session")
    switch_user("carl", 4)
    pick("dana", "${password}")
    dana_sid, dana_vt = session_of("dana")
    if len(workers()) != 2:
        raise Exception(f"{workers()} after dana's second login")
    machine.succeed("touch /tmp/lys-login/stop-agents")
    machine.execute("pkill -u carl -f 'lys-login lock'")
    begun = time.monotonic()
    machine.succeed("systemctl stop lys-login.service")
    took = time.monotonic() - begun
    if took >= 20:
        raise Exception(f"stopping with two sessions took {took:.0f}s")
    machine.wait_until_succeeds("[ -z \"$(pgrep -f '^/nix/store/[^ ]*/lys-login worker session')\" ]", timeout=15)
    for sid in (carl_sid, dana_sid):
        machine.wait_until_succeeds(f"! loginctl show-session {sid}", timeout=30)
    log = journal()
    for absent in ("panic", "dm: greeter connection:"):
        if absent in log:
            raise Exception(f"the journal has {absent!r}")
    print(log[-4000:])
  '';
}
