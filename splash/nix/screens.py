import contextlib
import re
import threading
import time
from datetime import timedelta

from PIL import Image, ImageChops

splash_logs = "journalctl --no-pager -o short-monotonic -u splash; dmesg --notime | grep -iE 'drm|fb|vkms|virtio|simple' | tail -n 80"


@contextlib.contextmanager
def logs_on_failure(machine, command=splash_logs):
    try:
        yield
    except BaseException:
        _, logs = machine.execute(f"{{ {command}; }} 2>&1")
        print(f"--- {command}\n{logs}")
        raise


def seconds(n):
    return timedelta(seconds=n)


def load(path):
    with Image.open(path) as img:
        return img.convert("RGB")


def screenshot(machine, name):
    name = f"{machine.name}-{name}"
    machine.screenshot(name)
    return load(machine.out_dir / f"{name}.png")


def grab(machine, card, name):
    name = f"{machine.name}-{name}"
    machine.succeed("rm -rf /tmp/grab && mkdir /tmp/grab")
    paths = machine.succeed(f"drm-grab {card} /tmp/grab").split()
    shots = {}
    for path in paths:
        machine.copy_from_machine(path, name)
        output = path.removeprefix("/tmp/grab/").removesuffix(".ppm")
        shots[output] = load(machine.out_dir / name / f"{output}.ppm")
    return shots


def no_splash(machine):
    if machine.execute("lys-splash --ping")[0] == 0:
        machine.succeed("lys-splash quit")
    machine.wait_until_fails("lys-splash --ping", timeout=seconds(10))
    machine.wait_until_fails("systemctl is-active splash", timeout=seconds(10))


def differences(a, b):
    if a.size != b.size:
        return f"{a.size} against {b.size}"
    box = ImageChops.difference(a, b).getbbox()
    if box is None:
        return None
    return f"pixels in {box}"


def assert_same(a, b, what):
    diff = differences(a, b)
    assert diff is None, f"{what}: they differ in {diff}"


def content(img, background):
    box = ImageChops.difference(img, Image.new("RGB", img.size, background)).getbbox()
    assert box is not None, f"a {img.size} frame shows nothing but the background {background}"
    return box


def black(img):
    return all(high < 16 for _, high in img.getextrema())


def console_text(img):
    strip = img.crop((0, 0, max(1, img.width // 10), img.height))
    return ImageChops.difference(strip, Image.new("RGB", strip.size, strip.getpixel((0, 0)))).getbbox()


def unshown(img):
    if black(img):
        return "black"
    return None if console_text(img) is None else "console text"


@contextlib.contextmanager
def screen_recorded(m, what, keep=8):
    path = m.out_dir / f"{m.name}-recording.ppm"
    stop = threading.Event()
    timeline, wrong, failed = [], [], []
    dumps, gap, gap_at, size = 0, 0.0, 0.0, None
    started = time.monotonic()

    def record():
        nonlocal dumps, gap, gap_at, size
        last = started
        try:
            while not stop.is_set():
                answer = m.qmp_client.send("screendump", {"filename": str(path)})
                assert "return" in answer, f"QMP's screendump answered {answer}"
                img = load(path)
                now = time.monotonic()
                if now - last > gap:
                    gap, gap_at = now - last, last - started
                last = now
                size = size or img.size
                problem = unshown(img) or (None if img.size == size else f"not the first dump's {size[0]}x{size[1]}")
                kind = f"{img.width}x{img.height} {problem or 'shown'}"
                if timeline and timeline[-1][0] == kind:
                    timeline[-1][2] += 1
                else:
                    timeline.append([kind, dumps, 1])
                if problem is not None:
                    if len(wrong) < keep:
                        img.save(m.out_dir / f"{m.name}-recording-{dumps:05d}.png")
                    wrong.append(f"dump {dumps}: {problem}")
                dumps += 1
        except BaseException as e:
            failed.append(e)

    thread = threading.Thread(target=record, name=f"{m.name}-screen-recording", daemon=True)
    thread.start()
    try:
        yield
    finally:
        stop.set()
        thread.join(timeout=60)
        assert not thread.is_alive(), f"{what}: the screen recording did not stop within 60s"
        assert not failed, f"{what}: the screen recording failed: {failed[0]!r}"
        assert dumps, f"{what}: the screen recording took no dump"
        path.unlink()
        print(f"{what}: {dumps} dumps in {time.monotonic() - started:.1f}s, at most {gap:.3f}s apart, from {gap_at:.1f}s in: " + "; ".join(f"{kind} from dump {i} for {n}" for kind, i, n in timeline))
        assert not wrong, f"{what}: {len(wrong)} of {dumps} dumps show the console or nothing: {wrong[:keep]}"


def await_until(seconds, attempt):
    deadline = time.monotonic() + seconds
    while True:
        result, unmet = attempt()
        if unmet is None:
            return result
        if time.monotonic() > deadline:
            raise AssertionError(f"not within {seconds}s: {unmet}")
        time.sleep(0.25)


def await_screen(machine, name, settled, seconds):
    shots = []

    def attempt():
        shots.append(screenshot(machine, name))
        prev = shots[-2] if len(shots) > 1 else None
        return shots[-1], settled(prev, shots[-1])

    return await_until(seconds, attempt)


def steady_and(check):
    def settled(prev, shot):
        if prev is None:
            return "one screenshot is not enough to call it steady"
        diff = differences(prev, shot)
        if diff is not None:
            return f"two screenshots in a row differ in {diff}"
        return check(shot)

    return settled


def matching(expected, what):
    def settled(_, shot):
        diff = differences(expected, shot)
        return None if diff is None else f"the screen and {what} differ in {diff}"

    return settled


scanout_log = "/tmp/scanout-watch"
scanned = re.compile(r"^(\d+) plane=(\d+) crtc=(\d+) fb=(\d+) (.*)$")
scanned_content = re.compile(r"by=(\S+) (\d+)x(\d+) .* mean=(\d+),(\d+),(\d+) dark=(\d+)$")


def watch_scanout(m):
    n = m.succeed("for c in /dev/dri/card*; do n=${c#/dev/dri/card}; if test -e /sys/kernel/debug/dri/$n/crtc-0; then echo $n; fi; done | sort -n | tail -1").strip()
    assert n, "no DRM card has a CRTC to watch"
    m.succeed(f"systemd-run --unit=scanout-watch -p StandardOutput=truncate:{scanout_log} scanout-watch /dev/dri/card{n} /sys/kernel/debug/dri/{n}")
    m.wait_until_succeeds(f"grep -q ' start$' {scanout_log}", timeout=seconds(30))


def scanned_out(m):
    m.succeed("systemctl stop scanout-watch")
    return scanned_lines(m)


def scanned_lines(m):
    records = []
    for line in m.succeed(f"cat {scanout_log}").splitlines():
        found = scanned.match(line)
        if found is None:
            continue
        t, plane, crtc, fb, rest = found.groups()
        record = {"t": int(t), "plane": int(plane), "crtc": int(crtc), "fb": int(fb), "line": line}
        content = scanned_content.search(rest)
        if content is not None:
            by, w, h, r, g, b, dark = content.groups()
            record.update(by=by, size=(int(w), int(h)), mean=(int(r), int(g), int(b)), dark=int(dark))
        records.append(record)
    return records
