{
  pkgs,
  vm,
  ...
}: let
  user = "hugo";
  password = "lys-handover-vm";
  greeterColour = "2a4a3a";
  sessionColour = "3050a0";

  greeterBackground = pkgs.writeText "lys-login-vm-handover-runtime.yaml" ''
    login:
      background-mode: color
      background-color: "#${greeterColour}"
  '';

  session = vm.lysWmSession {
    name = "handover";
    agent = "${pkgs.swaybg}/bin/swaybg -c '#${sessionColour}'";
    config.animations = {
      enabled = true;
      events.fade_in = {
        curve = "linear";
        duration = 1500;
      };
    };
  };

  rgb = hex: "(0x${builtins.substring 0 2 hex}, 0x${builtins.substring 2 2 hex}, 0x${builtins.substring 4 2 hex})";
in {
  name = "handover";
  sessions.${user} = session;
  module = {
    users.users.${user} = {
      isNormalUser = true;
      inherit password;
    };
  };
  script = _: ''
    import time

    GREETER = ${rgb greeterColour}
    SESSION = ${rgb sessionColour}
    SEGMENT = tuple(s - g for s, g in zip(SESSION, GREETER))

    def background(shot):
        return shot.pixel(shot.width // 10, shot.height // 2)

    def along(colour):
        t = sum((c - g) * d for c, g, d in zip(colour, GREETER, SEGMENT)) / sum(d * d for d in SEGMENT)
        return t, max(abs(c - (g + t * d)) for c, g, d in zip(colour, GREETER, SEGMENT))

    def dark(shot):
        samples = [shot.pixel(x, y) for y in range(0, shot.height, 16) for x in range(0, shot.width, 16)]
        return sum(max(p) < 16 for p in samples) / len(samples)

    def keep(shot, what):
        kept = os.path.join(machine.out_dir, f"handover-{what}.ppm")
        os.replace(shot.path, kept)
        return kept

    def journal():
        return machine.succeed("scenario-journal -u lys-login -o cat --no-pager")

    machine.wait_for_unit("lys-login.service")
    await_ready(machine, "username", 1)
    machine.succeed("install -D -m 0644 ${greeterBackground} /etc/lys-shell/runtime.yaml")
    machine.succeed("systemctl restart lys-login.service")
    await_ready(machine, "username", 2)
    end = time.monotonic() + 30
    while background(Screendump(machine, "handover")) != GREETER:
        if time.monotonic() >= end:
            machine.screenshot("handover-timeout-greeter")
            raise Exception(f"the greeter does not show its background {GREETER} 30s after it was ready")
        time.sleep(0.5)
    machine.screenshot("handover-01-greeter")

    machine.send_chars("${user}\n${password}")
    machine.send_key("ret")
    shown = []
    end = time.monotonic() + 120
    while True:
        shot = Screendump(machine, "handover")
        colour, black = background(shot), dark(shot)
        t, off = along(colour)
        if black >= 0.99:
            raise Exception(f"screendump {len(shown)} after the login is black, {black:.1%} of it dark; kept as {keep(shot, 'black')}")
        if off > 8 or not -0.02 <= t <= 1.02:
            raise Exception(f"screendump {len(shown)} after the login shows {colour}, {black:.1%} of it dark: neither the greeter's {GREETER}, the session's {SESSION} nor a blend of them; kept as {keep(shot, 'foreign')}")
        shown.append(t)
        if t >= 0.98 and off <= 3:
            break
        if time.monotonic() >= end:
            raise Exception(f"the session's frame did not show within 120s of the login: {len(shown)} screendumps, the last {t:.2f} of the way from the greeter's")
    os.remove(shot.path)
    machine.screenshot("handover-02-session")

    if shown[0] > 0.02:
        raise Exception(f"the first screendump after the login is {shown[0]:.2f} of the way to the session's frame, not the greeter's")
    back = [i for i in range(1, len(shown)) if shown[i] < shown[i - 1] - 0.02]
    if back:
        raise Exception(f"screendumps {back[:8]} step back toward the greeter's frame: {shown}")
    between = sum(1 for t in shown if 0.05 < t < 0.95)
    if between == 0:
        raise Exception(f"the session cuts from the greeter's frame with no screendump between the two: {shown}")

    machine.wait_until_succeeds("scenario-journal -u lys-login -o cat --no-pager | grep -q 'dm: bound the framebuffer console .* again: a compositor replaced the greeter.s last frame'", timeout=30)
    log = journal()
    for want in ("dm: unbound the framebuffer console", "dm: the compositor of the greeter stopped"):
        if want not in log:
            raise Exception(f"the journal lacks {want!r}:\n" + log[-4000:])
    cuts = machine.execute("scenario-journal -o cat --no-pager --grep 'instead of fading'")[1].strip()
    if cuts:
        raise Exception("the session's lys-wm cut from the greeter's frame instead of fading from it:\n" + cuts)
    print(f"the login: {len(shown)} screendumps from the greeter's frame to the session's, {between} of them between the two")
  '';
}
