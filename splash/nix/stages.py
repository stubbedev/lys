import json
import re

gate_marker = "greeter-gate: the greeter may start"

modeset = re.compile(r"Set \[MODE:|Link \[CONNECTOR:|mode changed|active changed|enable changed|connectors changed|requires full modeset")


def whole_output(m, command):
    name = f"whole-output-{m.name}"
    m.succeed(f"{{ {command}; }} > /tmp/shared/{name}")
    return (m.shared_dir / name).read_text(errors="replace")


def dump(m, i):
    path = m.out_dir / f"handover-{i:04d}.ppm"
    m.send_monitor_command(f"screendump {path}")
    return path


background = (0, 0, 0)


def black(img):
    return all(high < 16 for _, high in img.getextrema())


taken_over = re.compile(r"greeter: the first frame takes over the splash's: the logo at \((\d+),(\d+)\)-\((\d+),(\d+)\) on (\d+)x(\d+)")


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


def reboot_shows_splash(m, splash, logo):
    m.execute("systemctl --no-block reboot", check_return=False, check_output=False)
    m.wait_for_console_text(r'msg="showing the splash" mode=reboot', timeout=seconds(30))
    m.wait_for_console_text(r"reboot: Restarting system", timeout=seconds(60))
    await_until(10, lambda: (None, None if "paused" in m.send_monitor_command("info status") else "the VM has not paused at the reset"))
    assert_same(screenshot(m, "rebooting").crop(logo), splash.crop(logo), f"the logo in {logo} at the reset against the splash's")
    assert "msg=quitting mode=reboot" not in m.get_console_log(), "the reboot splash quit before the reset"


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


def in_order(m, *steps):
    entries = journal(m)
    times = [when(entries, unit, pattern) for unit, pattern in steps]
    for i in range(1, len(steps)):
        assert times[i - 1] < times[i], f"{steps[i]} at {times[i]}us came before {steps[i - 1]} at {times[i - 1]}us"
