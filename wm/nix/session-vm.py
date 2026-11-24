import re
import sys
import traceback

reported: list[BaseException] = []
KERNEL_MODULES = sorted({module for scenario in SCENARIOS for module in scenario["kernelModules"]})


def report_failure():
    failure = sys.exception()
    if failure is None or any(seen is failure for seen in reported):
        return
    reported.append(failure)
    traceback.print_exception(failure)


def as_alice(command):
    return f"su {USER} -c 'XDG_RUNTIME_DIR={RUNTIME} DBUS_SESSION_BUS_ADDRESS=unix:path={RUNTIME}/bus {command}'"


def lys_wm_journal():
    return machine.succeed("journalctl -b --no-pager -t lys-wm")


def end_session():
    machine.succeed("systemctl stop greetd.service", f"systemctl stop user@$(id -u {USER}).service")
    machine.execute(f"pkill -KILL -u {USER}")
    machine.wait_until_fails(f"pgrep -u {USER}", timeout=60)
    machine.wait_until_fails(f"test -e {RUNTIME}", timeout=60)
    machine.succeed(f"{BASE}/bin/switch-to-configuration test")


def clear_tmp():
    mounts = [m for m in machine.succeed("findmnt -rno TARGET").split() if m.startswith("/tmp/")]
    kept = " ".join(f"! -path {m}" for m in mounts)
    machine.succeed(f"find /tmp -mindepth 1 -maxdepth 1 {kept} -exec rm -rf --one-file-system {{}} +")


def fresh_home(home):
    machine.succeed(
        f"rm -rf --one-file-system /home/{USER}",
        f"install -d -m 700 -o {USER} -g users /home/{USER}",
        f"cp -rL --no-preserve=mode,ownership {home}/. /home/{USER}/",
        f"chown -R {USER}:users /home/{USER}",
    )


def restore_hardware(scenario):
    machine.succeed(
        "echo 1 > /sys/bus/pci/rescan",
        "udevadm settle --timeout=60",
        "for status in /sys/class/drm/card*-*/status; do echo detect > \"$status\"; done",
    )
    for module in KERNEL_MODULES:
        machine.succeed(f"! test -e /sys/module/{module}/initstate || modprobe -r {module}")
    for module, options in sorted(scenario["kernelModules"].items()):
        machine.succeed(f"modprobe {module} {options}")
    machine.succeed("udevadm settle --timeout=60", f"echo {scenario['drm_debug']} > /sys/module/drm/parameters/debug")


def start_session(scenario):
    machine.succeed("journalctl --rotate --vacuum-size=1")
    clear_tmp()
    fresh_home(scenario["home"])
    restore_hardware(scenario)
    machine.succeed("rm -f /run/greetd.run", f"{BASE}/specialisation/{scenario['name']}/bin/switch-to-configuration test", "systemctl start greetd.service")


def session_started(scenario):
    want = scenario["outputs"]
    outputs = as_alice("lys-wmctl outputs")
    machine.wait_for_unit("greetd.service")
    try:
        machine.wait_until_succeeds(f"{outputs} | jq -e 'length == {len(want)}'", timeout=120)
        machine.wait_until_succeeds("journalctl -b --no-pager -t lys-wm | grep -q 'lys-wm: listening on '", timeout=30)
    except Exception:
        print(machine.execute("journalctl -b --no-pager -t lys-wm -t greetd")[1])
        raise
    journal = lys_wm_journal()
    print(journal)
    framed = sorted(set(re.findall(r"drawing the cursor in the frame on ([^:]+):", journal)))
    assert framed == sorted(scenario["framed_cursors"]), f"the cursor is drawn in the frame on {framed}"
    machine.succeed(f"journalctl -b --no-pager -t lys-wm | grep 'rendering on {scenario['renders_on']},'")
    machine.succeed(f"journalctl -b --no-pager -t lys-wm | grep 'lys-wm: {scenario['renders_on']}.s frames are drawn on the CPU by llvmpipe'")
    machine.fail("journalctl -b --no-pager -t lys-wm | grep CREATE_DUMB")
    if scenario["sole_compositor"]:
        machine.wait_until_succeeds("[ \"$(pgrep -cx lys-wm)\" = 1 ] && ls -l /proc/$(pgrep -x lys-wm)/fd | grep -q /dev/dri/card", timeout=60)
        compositor = machine.succeed("pgrep -x lys-wm").split()
        assert len(compositor) == 1, f"a session without --holder runs {compositor}"
        status, refusal = machine.execute(f"{as_alice('lys-wmctl upgrade /run/current-system/sw/bin/lys-wm')} 2>&1")
        assert status != 0 and "--holder" in refusal, f"an upgrade without a holder: {refusal}"
    got = sorted(machine.succeed(f"{outputs} | jq -r '.[].name'").split())
    assert got == sorted(want), f"outputs {got}, want {want}"
    display = machine.succeed(f"ls {RUNTIME} | grep -m1 '^wayland-[0-9]*$'").strip()
    for output in want:
        machine.succeed(f"grep -qx enabled /sys/class/drm/card*-{output}/enabled")
        shot = f"/tmp/{scenario['name']}-{output}.png"
        machine.succeed(as_alice(f"WAYLAND_DISPLAY={display} grim -o {output} {shot}"))
        machine.copy_from_machine(shot)
        if scenario["background_only"]:
            colours = f"{as_alice(f'WAYLAND_DISPLAY={display} grim -t ppm -o {output} -')} | tail -n +4 | od -An -v -tx1 -w3 | sort -u | tr -s ' ' | sed '/^ *$/d'"
            try:
                machine.wait_until_succeeds(f"[ \"$({colours})\" = ' 1e 1e 2e' ]", timeout=30)
            except Exception:
                raise AssertionError(f"{output} shows {machine.succeed(colours).splitlines()[:4]} besides the background once the fade from the frame it inherited is over")
    return display


def run_scripts(scenario, display):
    namespace = dict(globals())
    namespace.update(params=scenario["params"], display=display)
    for path in scenario["scripts"]:
        with open(path) as source:
            exec(compile(source.read(), path, "exec"), namespace)


def unplug(card, outputs):
    machine.succeed(f"echo 1 > /sys/class/drm/{card}/device/remove")
    machine.wait_until_succeeds(f"{as_alice('lys-wmctl outputs')} | jq -e --argjson want '{json.dumps(outputs)}' 'map(.name) == $want'", timeout=60)
    machine.succeed("pgrep -x lys-wm")
    print(machine.succeed("journalctl -b --no-pager -t lys-wm | tail -5"))


machine.wait_for_unit("multi-user.target")
failed = []
for index, scenario in enumerate(SCENARIOS):
    name = scenario["name"]
    try:
        with subtest(f"{name}: a fresh session"):
            if index:
                end_session()
            start_session(scenario)
    except Exception:
        report_failure()
        raise Exception(f"the machine could not be reset for the scenario {name}")
    try:
        with subtest(name):
            display = session_started(scenario)
            run_scripts(scenario, display)
            if scenario["unplug"]:
                unplug(scenario["unplug"]["card"], scenario["unplug"]["outputs"])
    except Exception:
        report_failure()
        failed.append(name)
assert not failed, f"these scenarios failed: {', '.join(failed)}"
