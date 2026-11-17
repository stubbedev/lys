with stage(machine, "LUKS setup"):
    encrypt(machine, ["/dev/vdb"])

with stage(machine, "text passphrase"):
    type_passphrase(machine)
    unlocked(machine, [("crypt0", "/")])
    machine.succeed("test ! -e /dev/dri/card0")
    assert started(machine, "lys-splash-ask-password.service"), "the splash's ask-password agent never started"
    assert not started(machine, "systemd-ask-password-console.service"), "systemd's console agent started beside the splash's"
