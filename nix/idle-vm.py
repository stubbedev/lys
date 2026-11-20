import json
import time

BUDGETS = params["budgets"]
WINDOW = BUDGETS["window_seconds"]
PROBE = BUDGETS["probe_seconds"]
SETTLE_LIMIT = BUDGETS["settle_limit_seconds"]
QUIET_PROBES = BUDGETS["quiet_probes"]
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


def measure(window):
    wm = int(machine.succeed("pgrep -x lys-wm").strip())
    report = json.loads(machine.succeed(f"lys-power energy -for {window}s -threads -json"))
    seconds = report["Covered"] / 1e9
    found = {}
    for name, (threads, apps) in lys_threads(report, wm).items():
        found[name] = {
            "wakeups": round(sum(t["Wakeups"] for t in threads) * seconds),
            "threads": sorted(((round(t["Wakeups"] * seconds), t["Name"], t["TID"]) for t in threads), reverse=True),
            "gpu_seconds": sum(app["GPU"] for app in apps) * seconds,
        }
    return seconds, found


def over_budget(found, window):
    share = window / WINDOW
    problems = []
    for name, whole in sorted(BUDGETS["wakeups"].items()):
        got = found[name]
        budget = whole * share
        if got["wakeups"] > budget:
            top = ", ".join(f"thread {tid} {thread} {count}" for count, thread, tid in got["threads"][:3])
            problems.append(f"{name} woke {got['wakeups']} times in {window}s, over its budget of {budget:g}: {top}")
        gpu = BUDGETS["gpu_seconds"][name] * share
        if got["gpu_seconds"] > gpu:
            problems.append(f"{name} kept the GPU busy {got['gpu_seconds']:.3f}s in {window}s, over its budget of {gpu:g}s")
    return problems


def settle():
    quiet = 0
    deadline = time.monotonic() + SETTLE_LIMIT
    while quiet < QUIET_PROBES and time.monotonic() < deadline:
        _, found = measure(PROBE)
        busy = over_budget(found, PROBE)
        quiet = 0 if busy else quiet + 1
        if busy:
            print(f"not settled yet: {'; '.join(busy)}")


def gate(label):
    seconds, found = measure(WINDOW)
    print(f"idle baseline ({label}) over {seconds:.1f}s:")
    for name, got in sorted(found.items()):
        print(f"  {name}: {got['wakeups']} wakeups, {got['gpu_seconds']:.3f}s GPU; threads {got['threads'][:5]}")
    problems = over_budget(found, WINDOW)
    if problems:
        raise OverBudget("; ".join(problems))


machine.wait_until_succeeds("busctl --system status dev.stubbe.lys.Energy1", timeout=60)
settle()
gate("still session")

machine.succeed(f"install -m 644 -o {USER} -g users {params['ticking']} {CONFIG}")
try:
    gate("a clock that shows seconds")
except OverBudget as caught:
    if "lys-shell" not in str(caught):
        raise AssertionError(f"a shell ticking every second failed the gate without naming lys-shell: {caught}")
    print(f"the gate caught the ticking shell: {caught}")
else:
    raise AssertionError("a shell ticking every second passed the idle gate")

machine.succeed("busctl --system call dev.stubbe.lys.Energy1 /dev/stubbe/lys/Energy dev.stubbe.lys.Energy1 Watch")
machine.wait_until_succeeds(
    "busctl --system --json=short get-property dev.stubbe.lys.Energy1 /dev/stubbe/lys/Energy dev.stubbe.lys.Energy1 Report"
    " | jq -e '.data[0] > 0 and any(.data[3][]; .[0] | endswith(\"/lys-shell.service\"))'",
    timeout=30,
)
