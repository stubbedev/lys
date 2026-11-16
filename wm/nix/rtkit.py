[compositor] = machine.succeed("pgrep -x lys-wm").split()
machine.succeed(f"grep -E '^Max realtime priority +0 +0 ' /proc/{compositor}/limits")
machine.succeed(f"grep -E '^Max nice priority +0 +0 ' /proc/{compositor}/limits")
machine.succeed("journalctl -b --no-pager -t lys-wm | grep 'the compositor thread runs at SCHED_RR through rtkit'")
machine.succeed(f"grep -E '^Max realtime timeout +[0-9]+ +[0-9]+ ' /proc/{compositor}/limits")
rr = f"ps -L -o cls=,rtprio= -p {compositor} | grep -Eq '^ *RR +1$'"


def realtime():
    try:
        machine.wait_until_succeeds(rr, timeout=30)
    except Exception:
        print(machine.execute(f"ps -L -o tid=,cls=,rtprio= -p {compositor}; journalctl -b --no-pager -t lys-wm -t rtkit-daemon")[1])
        raise


realtime()
machine.succeed(f"ps -L -o cls=,rtprio= -p {compositor} | grep -Eq '^ *RR +2$'")
[spawner] = machine.succeed("pgrep -x lys-wm-spawner").split()
assert machine.succeed(f"ps -o ppid= -p {spawner}").strip() == compositor, "the spawner is not lys-wm's child"
machine.succeed(f"grep -E '^Max realtime timeout +unlimited +unlimited ' /proc/{spawner}/limits")
machine.succeed(as_alice("lys-wmctl dispatch exec sleep 4242"))
[client] = machine.wait_until_succeeds("pgrep -f '^sleep 4242$'", timeout=30).split()
limits = machine.succeed(f"grep -E '^Max realtime timeout ' /proc/{client}/limits")
assert limits.split()[3:5] == ["unlimited", "unlimited"], f"a client lys-wm spawned under its lowered RLIMIT_RTTIME got {limits!r}, not the unlimited one lys-wm started with"
machine.succeed(f"kill {client}")
demoted = "journalctl -b --no-pager -t lys-wm | grep -E 'the compositor thread ran [^ ]+ at SCHED_RR without blocking, past the 20ms its guard allows under RLIMIT_RTTIME'"
ordinary = machine.execute(demoted)[1]
if ordinary:
    print(machine.execute("journalctl -b --no-pager -t lys-wm -t rtkit-daemon")[1])
assert not ordinary, f"the compositor thread was demoted while it drew ordinary frames: {ordinary}"
threads = f"ps -L -o cls=,rtprio= -p {compositor} | grep -Ec '^ *RR +1$'"
raised = machine.succeed(threads).strip()
machine.succeed(as_alice("lys-wmctl dispatch debug spin 500"))
try:
    machine.wait_until_succeeds(f"{demoted} | grep -c . | grep -qx 1", timeout=30)
    machine.succeed("journalctl -b --no-pager -t lys-wm | grep -E 'internal/compositor\\.spinPreemptibly\\('")
    assert machine.succeed("pgrep -x lys-wm").split() == [compositor], "lys-wm did not survive a 500 ms realtime spin under rtkit's 200 ms RLIMIT_RTTIME"
    machine.wait_until_succeeds("journalctl -b --no-pager -t lys-wm | grep -q 'the compositor thread runs at SCHED_RR again'", timeout=30)
    machine.wait_until_succeeds(f"{threads} | grep -qx {raised}", timeout=30)
except Exception:
    print(machine.execute(f"ps -L -o tid=,cls=,rtprio= -p {compositor}; journalctl -b --no-pager -t lys-wm -t rtkit-daemon")[1])
    raise
