with stage(machine, "LUKS setup"):
    encrypt(machine, ["/dev/vdb", "/dev/vdc"])

with stage(machine, "passphrase prompt"):
    await_passphrase_prompt(machine)

with screen_recorded(machine, "from the passphrase to the greeter"):
    with stage(machine, "passphrase"):
        machine.send_chars(passphrase + "\n")
        machine.screenshot("typed")

    with stage(machine, "cached passphrase"):
        unlocked(machine, [("crypt0", "/"), ("crypt1", "/data")])

    with stage(machine, "simpledrm to virtio-gpu"):
        machine.wait_until_succeeds("test $(journalctl -b -o cat -u lys-splash-start -g 'lit an output' | wc -l) -ge 2", timeout=60)
        machine.succeed("journalctl -b -o cat -u lys-splash-start -g 'a card went away'")
        machine.succeed("test $(cat /sys/firmware/acpi/bgrt/status) = 1")

    with stage(machine, "survival"):
        pid = machine.succeed("cat /run/plymouth/pid").strip()
        in_order(machine,
            ("lys-splash-start.service", rf'msg="showing the splash" mode=boot pid={pid}$'),
            (None, r"^Switching root\.$"),
            ("lys-splash-read-write.service", r"^Finished "))
        assert machine.succeed(f"head -c1 /proc/{pid}/cmdline") == "@", "the splash in the real root is not marked to survive the switch"
        machine.succeed(f"test \"$(readlink /proc/{pid}/root)\" = /")

    with stage(machine, "memory lock"):
        locked = machine.succeed(f"grep ^VmLck: /proc/{pid}/status").split()
        assert locked[1] == "0", f"the boot splash locked {locked[1:]} of its memory"
        out = machine.succeed(f"{splash_test} -test.v -test.count=1 -test.run '^TestOnlyTheStoppingModesLockTheirMemory$' 2>&1")
        print(out)
        for mode in ["boot", "updates", "shutdown", "reboot", "kexec"]:
            assert f"--- PASS: TestOnlyTheStoppingModesLockTheirMemory/{mode} " in out, f"the memory lock test did not pass in {mode} mode"

    with stage(machine, "compatibility"):
        machine.succeed(f"{plymouth} --ping", f"{plymouth} display-message --text='nixpkgs plymouth'")
        assert started(machine, "lys-splash-ask-password.service"), "the splash's ask-password agent never started"

    with stage(machine, "hand-over", greeter_logs):
        await_gate(machine)
        splash, logo = hand_over(machine)

with stage(machine, "shutdown"):
    try:
        reboot_shows_splash(machine, splash, logo)
    finally:
        machine.crash()
