self: let
  user = "pauser";
  uid = 1600;
  hog = "battery-hog.service";
  essential = "dbus.service";
  env = "XDG_RUNTIME_DIR=/run/user/${toString uid}";
  pause = "dev.stubbe.lys.Pause1 /dev/stubbe/lys/Pause dev.stubbe.lys.Pause1";
in {
  name = "lys-power freezes a user's picked unit on battery and thaws it on AC";
  module = {pkgs, ...}: {
    imports = [self.nixosModules.lys-power];
    services.lys-power.enable = true;
    boot.kernelModules = ["test_power"];
    boot.extraModprobeConfig = "options test_power usb_online=off";
    users.users.${user} = {
      isNormalUser = true;
      inherit uid;
    };
    systemd.user.services.battery-hog.serviceConfig.ExecStart = "${pkgs.coreutils}/bin/sleep infinity";
  };
  script = ''
    import json

    def as_user(cmd):
        return f"runuser -u ${user} -- env ${env} {cmd}"

    def freezer_of(unit):
        return as_user(f"systemctl --user show -p FreezerState --value {unit}")

    def freezer(unit):
        return machine.succeed(freezer_of(unit)).strip()

    def state():
        return json.loads(machine.succeed("busctl --system --json=short get-property ${pause} State"))["data"]

    def set_ac(online):
        machine.succeed(f"echo {online} > /sys/module/test_power/parameters/ac_online")
        machine.wait_until_succeeds(f"grep -qx {1 if online == 'on' else 0} /sys/class/power_supply/test_ac/online", timeout=30)

    def waits_for(unit, want):
        try:
            machine.wait_until_succeeds(f"{freezer_of(unit)} | grep -qx {want}", timeout=60)
        except Exception:
            print(machine.execute("journalctl -b --no-pager -u lys-power.service -u upower.service")[1])
            raise

    machine.succeed("grep -qx Mains /sys/class/power_supply/test_ac/type", "grep -qx 1 /sys/class/power_supply/test_ac/online")
    machine.succeed("grep -qx 0 /sys/class/power_supply/test_usb/online")
    machine.succeed("systemctl start lys-power.service")
    machine.wait_for_unit("lys-power.service")
    machine.wait_for_unit("upower.service")
    machine.wait_until_succeeds("busctl --system status dev.stubbe.lys.Pause1", timeout=30)

    machine.succeed("systemctl start user@${toString uid}.service")
    machine.wait_for_unit("user@${toString uid}.service")
    machine.succeed(as_user("systemctl --user start ${hog} ${essential}"))
    assert freezer("${hog}") == "running", freezer("${hog}")
    assert freezer("${essential}") == "running", freezer("${essential}")

    refused = machine.fail(as_user("busctl --system call ${pause} PickUser as 2 ${hog} ${essential} 2>&1"))
    assert "the session or the system needs it running" in refused, refused
    assert state()[0] == [], f"a refused pick changed the picks: {state()}"

    machine.succeed(as_user("busctl --system call ${pause} PickUser as 1 ${hog}"))
    picks, entries, resumed = state()
    assert picks == ["user/${toString uid}/${hog}"], picks
    assert entries == [] and not resumed, state()
    assert freezer("${hog}") == "running", "paused on AC"

    set_ac("off")
    waits_for("${hog}", "frozen")
    picks, entries, resumed = state()
    assert entries == [["user/${toString uid}/${hog}", "paused", ""]], entries
    assert freezer("${essential}") == "running", "an essential unit was frozen"

    set_ac("on")
    waits_for("${hog}", "running")
    assert state()[1] == [], state()
    assert freezer("${essential}") == "running", "an essential unit was frozen"
    machine.succeed(as_user("systemctl --user is-active ${hog}"))
    journal = machine.succeed("journalctl -b --no-pager -u lys-power.service")
    assert "pausing on battery" not in journal, journal
  '';
}
