try:
    for name in params["started"]:
        machine.wait_for_file(f"/tmp/autostart-{name}", timeout=60)
        got = machine.succeed(f"cat /tmp/autostart-{name}").strip()
        assert got == display, f"{name} started with WAYLAND_DISPLAY={got!r}, the session's is {display!r}"
    machine.wait_until_succeeds(as_alice("systemctl --user is-active xdg-desktop-autostart.target"), timeout=30)
    for name in params["skipped"]:
        machine.fail(f"test -e /tmp/autostart-{name}")
except Exception:
    units = as_alice("systemctl --user list-units --all 'app-*' xdg-desktop-autostart.target lys-wm-session.target")
    print(machine.execute(f"{units}; {as_alice('journalctl --user -b --no-pager')} | tail -80")[1])
    raise
machine.succeed(f"{as_alice('systemctl --user show -p After xdg-desktop-autostart.target')} | grep -q lys-wm-session.target")
