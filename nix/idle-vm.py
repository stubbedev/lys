import json

BUDGETS = params["budgets"]
WINDOW = BUDGETS["window_seconds"]
PROBE = BUDGETS["probe_seconds"]
FIRST_WINDOWS = BUDGETS["first_minute_windows"]
CLOCK_REDRAW = BUDGETS["clock_redraw_seconds"]
CONFIG = f"/home/{USER}/.config/lys-shell/config.yaml"


class OverBudget(AssertionError):
    pass


def lys_threads(report, wm):
    def unit(name):
        return [app for app in report["Apps"] if app["Cgroup"].endswith(f"/{name}")]

    def threads(apps, keep):
        return [t for app in apps for t in app["Threads"] if keep(t)], apps

    session = [app for app in report["Apps"] if wm in (app["PIDs"] or [])]
    return {
        "lys-wm": threads(session, lambda t: t["PID"] == wm),
        "lys-shell": threads(unit("lys-shell.service"), lambda t: True),
        "lys-power": threads(unit("lys-power.service"), lambda t: True),
    }


def flips():
    return {o["output"]: o["flips"] for o in json.loads(machine.succeed(as_alice("lys-wmctl flips")))}


def minute(back=0):
    return (int(machine.succeed("date +%s")) - back) // 60


def clear_of_the_clock(seconds):
    left = 60 - int(machine.succeed("date +%s")) % 60
    if left > seconds + CLOCK_REDRAW:
        return
    before = json.dumps(flips())
    machine.wait_until_succeeds(f"{as_alice('lys-wmctl flips')} | jq -e --argjson before '{before}' 'all(.[]; .flips > $before[.output])'", timeout=90)


def measure(window, unit=None):
    wm = int(machine.succeed("pgrep -x lys-wm").strip())
    began = minute(CLOCK_REDRAW)
    flipped = flips()
    report = json.loads(machine.succeed(f"lys-power energy -for {window}s -threads -json"))
    flipped = {output: count - flipped.get(output, 0) for output, count in flips().items()}
    ticks = minute() - began
    seconds = report["Covered"] / 1e9
    found = {}
    if unit is not None:
        apps = [app for app in report["Apps"] if app["Cgroup"].endswith(f"/{unit}")]
        found[unit] = {"wakeups": round(sum(t["Wakeups"] for app in apps for t in app["Threads"]) * seconds)}
    for name, (threads, apps) in lys_threads(report, wm).items():
        found[name] = {
            "wakeups": round(sum(t["Wakeups"] for t in threads) * seconds),
            "threads": sorted(((round(t["Wakeups"] * seconds), t["Name"], t["TID"]) for t in threads), reverse=True),
            "gpu_seconds": sum(app["GPU"] for app in apps) * seconds,
        }
    found["lys-wm"]["flips"] = flipped
    found["lys-wm"]["clock_ticks"] = ticks
    return seconds, found


def idle_wakeups(window):
    return {name: whole * window / WINDOW for name, whole in BUDGETS["wakeups"].items()}


def over_flips(found):
    got = found["lys-wm"]
    allowed = got["clock_ticks"] * BUDGETS["flips_per_clock_tick"]
    return [
        f"lys-wm flipped {output} {count} times while still, over the {allowed} its {got['clock_ticks']} clock minute ticks allow"
        for output, count in sorted(got["flips"].items())
        if count > allowed
    ]


def over_budget(found, window, wakeups):
    share = window / WINDOW
    problems = []
    for name, budget in sorted(wakeups.items()):
        got = found[name]
        if got["wakeups"] > budget:
            top = ", ".join(f"thread {tid} {thread} {count}" for count, thread, tid in got["threads"][:3])
            problems.append(f"{name} woke {got['wakeups']} times in {window}s, over its budget of {budget:g}: {top}")
        gpu = BUDGETS["gpu_seconds"][name] * share
        if got["gpu_seconds"] > gpu:
            problems.append(f"{name} kept the GPU busy {got['gpu_seconds']:.3f}s in {window}s, over its budget of {gpu:g}s")
    return problems


def gate(label, window=WINDOW, wakeups=idle_wakeups(WINDOW), still=True):
    seconds, found = measure(window)
    print(f"{label} over {seconds:.1f}s:")
    for name, got in sorted(found.items()):
        print(f"  {name}: {got['wakeups']} wakeups, {got['gpu_seconds']:.3f}s GPU; threads {got['threads'][:5]}")
    print(f"  lys-wm flips {found['lys-wm']['flips']} across {found['lys-wm']['clock_ticks']} clock minute ticks")
    problems = over_budget(found, window, wakeups) + (over_flips(found) if still else [])
    if problems:
        raise OverBudget("; ".join(problems))


SHELL_STARTED = as_alice("busctl --user get-property dev.stubbe.lys.Shell1 /dev/stubbe/lys/Shell dev.stubbe.lys.Shell1 Started")


def invocations():
    return {
        "lys-power": machine.succeed("systemctl show -P InvocationID lys-power.service").strip(),
        "lys-shell": machine.succeed(as_alice("systemctl --user show -P InvocationID lys-shell.service")).strip(),
    }


def started():
    machine.wait_until_succeeds("systemctl is-active --quiet lys-power.service", timeout=60)
    machine.wait_until_succeeds(f"{SHELL_STARTED} | grep -qx 'b true'", timeout=60)
    return invocations()


def still_running(since):
    restarted = sorted(name for name, invocation in invocations().items() if invocation != since[name])
    assert not restarted, f"{', '.join(restarted)} restarted while the gate measured it"


STARTED = started()
gate(f"startup, the first {PROBE}s once lys-power and lys-shell said they started", PROBE, BUDGETS["startup_wakeups"], still=False)
for window in range(FIRST_WINDOWS):
    gate(f"idle baseline, the first minute after startup, window {window + 1} of {FIRST_WINDOWS}")

UNRELATED = BUDGETS["unrelated_uevents"]
clear_of_the_clock(PROBE)
machine.succeed(
    f"sh -c 'for i in $(seq {UNRELATED['count']}); do udevadm trigger --settle --action=change /sys/devices/virtual/mem/null && echo >> /tmp/unrelated-uevents; done'"
    " >/dev/null 2>&1 & echo $! > /tmp/unrelated-uevents.pid"
)
seconds, found = measure(PROBE)
machine.succeed("while kill -0 $(cat /tmp/unrelated-uevents.pid) 2>/dev/null; do sleep 0.1; done")
fired = int(machine.succeed("wc -l < /tmp/unrelated-uevents").strip())
assert fired == UNRELATED["count"], f"{fired} of {UNRELATED['count']} unrelated uevents fired"
unrelated = {name: idle_wakeups(PROBE)[name] for name in UNRELATED["processes"]}
print(f"{fired} unrelated uevents over {seconds:.1f}s: " + ", ".join(f"{name} {found[name]['wakeups']} wakeups" for name in sorted(found)))
problems = over_budget({name: found[name] for name in unrelated}, PROBE, unrelated)
assert not problems, f"uevents of a subsystem nothing acts on woke lys: {'; '.join(problems)}"

BUSY = "app-busy-1.scope"
WINDOWS = as_alice("lys-wmctl windows")


def busy_property(name):
    return machine.succeed(as_alice(f"systemctl --user show -P {name} {BUSY}")).strip()


def busy_slack():
    pid = machine.succeed(f"pgrep -u {USER} -x weston-presenta").split()[0]
    return int(machine.succeed(f"cat /proc/{pid}/timerslack_ns"))


def asleep():
    return machine.succeed(f"{WINDOWS} | jq -c '[.[].asleep]'").strip()


machine.succeed(as_alice(f"WAYLAND_DISPLAY={display} systemd-run --user --scope --unit={BUSY} weston-presentation-shm") + " > /tmp/busy.log 2>&1 &")
try:
    machine.wait_until_succeeds(f"[ \"$({WINDOWS} | jq length)\" = 1 ]", timeout=60)
except Exception:
    print(machine.execute(f"cat /tmp/busy.log; {WINDOWS}")[1])
    raise
shown = measure(PROBE, BUSY)[1][BUSY]["wakeups"]
assert shown * WINDOW / PROBE >= BUDGETS["shown_client_wakeups"], f"a shown busy client woke only {shown} times in {PROBE}s: it is not busy, so hiding it proves nothing"
assert busy_property("CPUWeight") == "[not set]", f"a shown app has CPUWeight {busy_property('CPUWeight')}"

machine.succeed(as_alice("lys-wmctl dispatch workspace 9"))
machine.wait_until_succeeds(f"[ \"$({WINDOWS} | jq -c '[.[].asleep]')\" = '[true]' ]", timeout=30)
machine.wait_until_succeeds(f"[ \"$({as_alice(f'systemctl --user show -P CPUWeight {BUSY}')})\" = 20 ]", timeout=30)
assert busy_property("IOWeight") == "20", f"a napping app has IOWeight {busy_property('IOWeight')}"
assert busy_slack() == 50_000_000, f"a napping app's timer slack is {busy_slack()} ns"
seconds, found = measure(WINDOW, BUSY)
hidden = found.pop(BUSY)["wakeups"]
print(f"a hidden busy client: {hidden} wakeups and lys-wm {found['lys-wm']['wakeups']} over {WINDOW}s; shown it woke {shown} times in {PROBE}s")
assert hidden <= BUDGETS["hidden_client_wakeups"], f"a hidden busy client woke {hidden} times in {WINDOW}s, over {BUDGETS['hidden_client_wakeups']}"
problems = over_budget(found, WINDOW, idle_wakeups(WINDOW)) + over_flips(found)
assert not problems, f"with a busy client hidden: {'; '.join(problems)}"

machine.succeed(as_alice("lys-wmctl dispatch workspace 1"))
machine.wait_until_succeeds(f"[ \"$({as_alice(f'systemctl --user show -P CPUWeight {BUSY}')})\" = '[not set]' ]", timeout=10)
assert asleep() == "[false]", f"a shown window is still asleep: {asleep()}"
assert busy_slack() == 50_000, f"a woken app's timer slack is {busy_slack()} ns, want the default"
machine.succeed(f"pkill -u {USER} -x weston-presenta")
machine.wait_until_succeeds(f"[ \"$({WINDOWS} | jq length)\" = 0 ]", timeout=30)

machine.succeed(f"install -m 644 -o {USER} -g users {params['ticking']} {CONFIG}")
try:
    gate("idle baseline, a clock that shows seconds")
except OverBudget as caught:
    if "lys-shell" not in str(caught):
        raise AssertionError(f"a shell ticking every second failed the gate without naming lys-shell: {caught}")
    if "lys-wm flipped" not in str(caught):
        raise AssertionError(f"a clock redrawing every second failed the gate without naming lys-wm's flips: {caught}")
    print(f"the gate caught the ticking shell: {caught}")
else:
    raise AssertionError("a shell ticking every second passed the idle gate")

machine.succeed("busctl --system call dev.stubbe.lys.Energy1 /dev/stubbe/lys/Energy dev.stubbe.lys.Energy1 Watch")
machine.wait_until_succeeds(
    "busctl --system --json=short get-property dev.stubbe.lys.Energy1 /dev/stubbe/lys/Energy dev.stubbe.lys.Energy1 Report"
    " | jq -e '.data[0] > 0 and any(.data[3][]; .[0] | endswith(\"/lys-shell.service\"))'",
    timeout=30,
)
still_running(STARTED)
