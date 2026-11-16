with stage(machine, "LUKS setup"):
    encrypt(machine, ["/dev/vdb"])

with stage(machine, "text passphrase"):
    type_passphrase(machine)
    unlocked(machine, [("crypt0", "/")])
    machine.succeed("test ! -e /dev/dri/card0")
    machine.succeed("journalctl -b -u lys-splash-ask-password.service -g 'Started '")
    machine.fail("journalctl -b -u systemd-ask-password-console.service -g 'Started '")
