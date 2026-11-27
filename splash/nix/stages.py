import json

gate_marker = "greeter-gate: the greeter may start"

modeset = re.compile(r"Set \[MODE:|Link \[CONNECTOR:|mode changed|active changed|enable changed|connectors changed|requires full modeset")


def whole_output(m, command):
    name = f"whole-output-{m.name}"
    m.succeed(f"{{ {command}; }} > /tmp/shared/{name}")
    return (m.shared_dir / name).read_text(errors="replace")


def dump(m, i):
    path = m.out_dir / f"{m.name}-handover-{i:04d}.ppm"
    m.send_monitor_command(f"screendump {path}")
    return path


background = (0, 0, 0)


taken_over = re.compile(r"greeter: the first frame on \S+ takes over the splash's: the logo at \((\d+),(\d+)\)-\((\d+),(\d+)\) on (\d+)x(\d+)")


def taken_over_logo(m):
    log = whole_output(m, "journalctl -b -t lys-login -o cat --no-pager")
    found = taken_over.findall(log)
    assert len(found) == 1, f"the greeter said {len(found)} times where it took the splash's logo over, want once"
    x0, y0, x1, y1, w, h = (int(n) for n in found[0])
    return (x0, y0, x1, y1), (w, h)


placed_line = re.compile(r'msg="placed the logo" output=\S+ at=\((\d+),(\d+)\)-\((\d+),(\d+)\) size=\((\d+),(\d+)\)')


def placed_logo(m, size):
    log = whole_output(m, "journalctl -b -u lys-splash-start -o cat --no-pager")
    boxes = {tuple(int(n) for n in found[:4]) for found in placed_line.findall(log) if (int(found[4]), int(found[5])) == size}
    assert len(boxes) == 1, f"lys-splash placed its logo on a {size[0]}x{size[1]} frame at {sorted(boxes)}, want one place"
    return boxes.pop()


def inside(box, outer):
    return outer[0] <= box[0] and outer[1] <= box[1] and box[2] <= outer[2] and box[3] <= outer[3]


def logo_area(box):
    return (box[2] - box[0]) * (box[3] - box[1])


def jumps(a, b):
    return sum(ImageChops.difference(a, b).convert("L").histogram()[97:])


greeter_logs = "journalctl -b --no-pager -o short-monotonic -u 'lys-splash*' -u lys-login -u greeter-gate -u user@998; journalctl -b --no-pager -o short-monotonic _UID=998 | tail -50"


greeter_background = (0x2a, 0x4a, 0x3a)
session_background = (0x30, 0x50, 0xa0)
console_framebuffer = "[fbcon]"


def screen_records(records, size, what, outputs=1):
    planes = sorted({r["plane"] for r in records if r.get("size") == size})
    assert len(planes) == outputs, f"{what}: planes {planes} scanned out {size[0]}x{size[1]} frames, want {outputs} primary planes"
    screens = []
    for primary in planes:
        shown = [r for r in records if r["plane"] == primary]
        on = f"{what} on plane {primary}"
        off = [r["line"] for r in shown if r["fb"] == 0]
        assert not off, f"{on}: the primary plane went dark: {off[:4]}"
        console = [r["line"] for r in shown if r.get("by") == console_framebuffer]
        assert not console, f"{on}: the kernel's console took the screen: {console[:4]}"
        unread = [r["line"] for r in shown if "mean" not in r]
        assert not unread, f"{on}: frames the watch could not read: {unread[:4]}"
        black = [r["line"] for r in shown if r["dark"] == 1000]
        assert not black, f"{on}: black frames: {black[:4]}"
        screens.append((on, shown))
    return screens


def blend_of(colour, a, b):
    seg = [b[i] - a[i] for i in range(3)]
    t = sum((colour[i] - a[i]) * seg[i] for i in range(3)) / sum(x * x for x in seg)
    return t, max(abs(colour[i] - (a[i] + t * seg[i])) for i in range(3))


def assert_fades(shown, a, b, what):
    start = next((i for i, r in enumerate(shown) if blend_of(r["mean"], a, b)[1] <= 4 and abs(blend_of(r["mean"], a, b)[0]) <= 0.02), None)
    assert start is not None, f"{what}: no frame shows {a} alone: {[r['line'] for r in shown[-8:]]}"
    last, between = 0.0, []
    for r in shown[start:]:
        t, off = blend_of(r["mean"], a, b)
        assert off <= 6 and -0.02 <= t <= 1.02, f"{what}: {r['line']} is no blend of {a} and {b}"
        assert t >= last - 0.02, f"{what}: {r['line']} steps back toward {a} from {last:.2f} of the way"
        if 0.05 < t < 0.95:
            between.append(r["line"])
        last = max(last, t)
    assert last >= 0.98, f"{what}: the frames end {last:.2f} of the way from {a} to {b}"
    assert between, f"{what}: the screen cuts from {a} to {b} with no frame between"
    return len(shown) - start, len(between)


def await_gate(m):
    m.wait_for_unit("lys-splash-read-write.service", timeout=seconds(90))
    m.wait_until_succeeds("systemctl show -P ActiveState greeter-gate | grep -qx activating", timeout=seconds(60))


def no_quit_unit(m, when):
    for unit in ("lys-splash-quit", "getty@tty1"):
        state = m.succeed(f"systemctl show -P ActiveState {unit}").strip()
        assert state == "inactive", f"{unit} is {state} {when}"


def hand_over(m):
    pid = m.succeed("cat /run/plymouth/pid").strip()
    m.succeed("lys-splash pause-progress")
    m.sleep(1)
    splash = screenshot(m, "splash")
    content(splash, background)
    placed = placed_logo(m, splash.size)
    no_quit_unit(m, "under lys-login")

    watch_scanout(m)
    m.succeed("touch /run/greeter-go")
    paths = []
    quit_at = None
    deadline = time.monotonic() + 180
    while quit_at is None or len(paths) < quit_at + 40:
        paths.append(dump(m, len(paths)))
        time.sleep(0.04)
        if quit_at is None and len(paths) % 10 == 0 and m.execute(f"kill -0 {pid}")[0] != 0:
            quit_at = len(paths)
        assert time.monotonic() < deadline, f"the splash still runs 180s after lys-login was let start, {len(paths)} dumps in"

    screen_records(scanned_out(m), splash.size, "the splash's hand-over")
    frames = [load(p) for p in paths]
    for i, frame in enumerate(frames):
        assert not black(frame), f"dump {i} of {len(frames)} is black"
    left = next((i for i, frame in enumerate(frames) if differences(frame, splash) is not None), None)
    assert left is not None, f"none of {len(frames)} dumps shows anything but the splash"
    again = [i for i in range(left, len(frames)) if differences(frames[i], splash) is None]
    assert not again, f"dumps {again[:8]} show the splash again after it began to leave in dump {left}"

    logo, size = taken_over_logo(m)
    assert size == splash.size, f"the greeter took the splash's frame over at {size}, the screen is {splash.size}"
    assert logo == placed, f"the greeter put the splash's logo at {logo}, the splash placed it at {placed}"
    last = [i for i in range(left, len(frames)) if inside(content(frames[i], background), logo)]
    assert last, f"no dump shows the splash's last frame, its logo alone in {logo}"
    picture = frames[last[0]]
    picture.save(m.out_dir / "splash-last.png")
    assert_same(picture.crop(logo), splash.crop(logo), f"the logo in {logo} as the splash left it against the splash before it left")
    for i in range(last[0] + 1, len(frames)):
        jumped = jumps(frames[i - 1].crop(logo), frames[i].crop(logo))
        assert jumped <= logo_area(logo) // 50, f"between dumps {i - 1} and {i}, {jumped} pixels of the logo's box {logo} jump: the greeter's first frame does not hold the splash's logo there"
    frames[-1].save(m.out_dir / "greeter-last.png")
    print(f"{len(frames)} dumps: the splash until dump {left}, its logo alone in {logo} from dump {last[0]}, the greeter fading in over it after")

    kernel = whole_output(m, "dmesg --notime")
    assert gate_marker in kernel, "the gate left no mark in the kernel log"
    drm = [line for line in kernel[kernel.rindex(gate_marker):].splitlines()[1:] if "[drm:" in line]
    commits = [line for line in drm if "] committing " in line]
    assert commits, "nothing committed after the splash dropped master"
    assert commits[0].endswith(" nonblocking"), f"the first commit after the splash dropped master is not lys-wm's flip: {commits[0]}"
    modesets = [line for line in drm if modeset.search(line)]
    assert not modesets, f"the hand-over modeset: {modesets[:8]}"

    m.succeed("journalctl -b -u lys-splash-start -g 'msg=quitting mode=boot retain=true'")
    no_quit_unit(m, "after the hand-over")
    m.succeed("pgrep -u lys-greeter -f 'lys-login greet'")
    return splash, logo


def log_in(m, size, outputs=1):
    m.wait_until_succeeds("journalctl -b -t lys-login -o cat | grep -q 'greeter: ready: .* the username field'", timeout=seconds(60))
    m.succeed("echo 'lys-splash vm: alice logs in' > /dev/kmsg")
    watch_scanout(m)
    m.send_chars("alice\nlys-splash-vm\n")
    m.wait_until_succeeds("pgrep -u alice swaybg", timeout=seconds(120))

    def desktop():
        last = {r["plane"]: r for r in scanned_lines(m) if r.get("size") == size}
        if len(last) < outputs:
            return None, f"{len(last)} of {outputs} planes show {size[0]}x{size[1]} frames"
        unsettled = [r["line"] for r in last.values() if max(abs(c - w) for c, w in zip(r["mean"], session_background)) > 2]
        return None, f"the screen shows {unsettled}" if unsettled else None

    await_until(30, desktop)
    fades = [(on, assert_fades(shown, greeter_background, session_background, on)) for on, shown in screen_records(scanned_out(m), size, "the login", outputs)]
    m.wait_until_succeeds("journalctl -b -t lys-login -o cat | grep -q 'bound the framebuffer console .* again: a compositor replaced the greeter.s last frame'", timeout=seconds(60))
    bound = m.succeed("grep -l 'frame buffer device' /sys/class/vtconsole/*/name | xargs -n1 dirname | xargs -I{} cat {}/bind").split()
    assert bound and all(b == "1" for b in bound), f"the framebuffer console is not bound again once the desktop shows: {bound}"
    for on, (flips, between) in fades:
        print(f"{on}: {flips} flips from the greeter's last frame to the desktop, {between} of them between the two")


def survives_switch(m):
    daemon = m.succeed("systemctl show -P MainPID lys-login").strip()
    session = m.succeed("pgrep -u alice swaybg").strip()
    m.wait_until_fails("systemctl is-active --quiet user@$(id -u lys-greeter).service", timeout=seconds(60))
    m.succeed("/run/current-system/bin/switch-to-configuration test")
    m.succeed("systemctl start multi-user.target")
    state = m.succeed("systemctl show -P ActiveState lys-login").strip()
    assert state == "active", f"lys-login is {state} after a switch"
    after = m.succeed("systemctl show -P MainPID lys-login").strip()
    assert after == daemon, f"a switch replaced lys-login's daemon {daemon} with {after}"
    m.succeed(f"kill -0 {session}")
    no_quit_unit(m, "after a switch")


def reboot_shows_splash(m, splash, logo):
    m.execute("systemctl --no-block reboot", check_return=False, check_output=False)
    m.wait_for_console_text(r'msg="showing the splash" mode=reboot', timeout=seconds(30))
    await_until(180, lambda: (None, None if "paused" in m.send_monitor_command("info status") else "the VM has not paused at the reset"))
    stopped = m.wait_for_qmp_event(lambda e: e["event"] == "SHUTDOWN", timeout=seconds(10))
    assert stopped["data"]["reason"] == "guest-reset", f"the VM stopped for {stopped['data']}, not for the guest's reset"
    rebooting = screenshot(m, "rebooting")
    console = m.get_console_log()
    assert 'msg="fading the status group out for the reset" mode=reboot' in console, "systemd-shutdown's hook never asked the reboot splash to fade its status group out"
    assert 'msg="the status group is gone, the frame stays until the reset" mode=reboot' in console, "the reboot splash did not finish fading its status group out before the reset"
    assert 'msg="the splash faded its status group out"' in console, "systemd-shutdown's hook did not hear the reboot splash finish its fade"
    assert "msg=\"unbound the kernel's framebuffer console, so a switch to text mode leaves the frame on screen\"" in console, "the reboot splash left the kernel's framebuffer console bound, so the exitrd's switch to text mode takes the screen"
    assert_same(rebooting.crop(logo), splash.crop(logo), f"the logo in {logo} at the reset against the splash's")
    shown = content(rebooting, background)
    assert inside(shown, logo), f"at the reset the screen shows {shown}, more than the logo in {logo}: the status group did not fade out"
    assert "msg=quitting mode=reboot" not in console, "the reboot splash quit before the reset"


def journal_entries(out):
    entries = []
    for line in filter(None, out.split("\n")):
        try:
            e = json.loads(line)
        except json.JSONDecodeError as err:
            raise ValueError(f"the journal has a line that is not JSON ({err}): {line!r}") from err
        if "__MONOTONIC_TIMESTAMP" not in e:
            continue
        msg = e.get("MESSAGE") or ""
        if isinstance(msg, list):
            msg = bytes(msg).decode(errors="replace")
        entries.append((int(e["__MONOTONIC_TIMESTAMP"]), e.get("UNIT") or e.get("_SYSTEMD_UNIT") or "", msg))
    return entries


def journal(m):
    return journal_entries(whole_output(m, "journalctl -b --no-pager -o json --output-fields=MESSAGE,UNIT,_SYSTEMD_UNIT"))


def when(entries, unit, pattern):
    for t, u, msg in entries:
        if (unit is None or u == unit) and re.search(pattern, msg):
            return t
    raise AssertionError(f"the journal has no {pattern!r}" + (f" from {unit}" if unit else ""))


def started(m, unit):
    return m.succeed(f"systemctl show -P ExecMainStartTimestampMonotonic {unit}").strip() not in ("", "0")


def in_order(m, *steps):
    entries = journal(m)
    times = [when(entries, unit, pattern) for unit, pattern in steps]
    for i in range(1, len(steps)):
        assert times[i - 1] < times[i], f"{steps[i]} at {times[i]}us came before {steps[i - 1]} at {times[i - 1]}us"


def failure_logs(m, logs):
    console = f"--- {m.name}'s console\n" + m.get_console_log()[-20000:]
    if not m.is_up():
        return console
    try:
        _, out = m.execute(f"{{ {logs}; }} 2>&1", timeout=seconds(30))
    except BaseException:
        return console
    return f"--- {m.name}: {logs}\n{out}"


@contextlib.contextmanager
def stage(m, name, logs="journalctl -b --no-pager -o short-monotonic -u 'lys-splash*' -u 'systemd-cryptsetup@*'"):
    try:
        yield
    except BaseException as e:
        print(failure_logs(m, logs))
        raise AssertionError(f"{m.name}: the {name} stage failed: {e}") from e


def encrypt(m, disks):
    m.wait_for_unit("lys-splash-read-write.service", timeout=seconds(120))
    for i, disk in enumerate(disks):
        m.succeed(
            f"echo -n {passphrase} | cryptsetup luksFormat -q --iter-time=1 {disk} -",
            f"echo -n {passphrase} | cryptsetup luksOpen -q {disk} crypt{i}",
            f"mkfs.ext4 -q /dev/mapper/crypt{i}",
        )
    m.succeed("/run/current-system/specialisation/luks/bin/switch-to-configuration boot", "sync")
    m.crash()
    m.start()


def await_passphrase_prompt(m):
    m.wait_for_console_text(r"Cryptography Setup for ", timeout=seconds(120))
    time.sleep(3)


def type_passphrase(m):
    await_passphrase_prompt(m)
    m.send_chars(passphrase + "\n")


def unlocked(m, mounts):
    m.wait_for_unit("lys-splash-read-write.service", timeout=seconds(120))
    table = m.succeed("mount")
    for device, at in mounts:
        assert f"/dev/mapper/{device} on {at} type ext4" in table, f"/dev/mapper/{device} is not mounted on {at}:\n{table}"
