pkgs: let
  launch = import ./launch.nix pkgs;
  checker = pkgs.runCommand "checker.ppm" {} ''
    ${pkgs.python3}/bin/python3 -c '
    import sys
    size = 64
    pixels = b"".join(b"\xff\x00\x00" if (x + y) % 2 == 0 else b"\x00\x00\xff" for y in range(size) for x in range(size))
    sys.stdout.buffer.write(b"P6\n%d %d\n255\n" % (size, size) + pixels)
    ' > $out
  '';
  feh = name: launch name "${pkgs.feh}/bin/feh --class ${name} --image-bg black ${checker}";
in {
  name = "xwayland";
  outputs = ["Virtual-1" "Virtual-2"];
  lysWMConfig = ''
    outputs:
      - {match: Virtual-1, scale: 1.5}
      - {match: Virtual-2, scale: 1}
  '';
  machineConfig.environment.systemPackages = [pkgs.xwininfo pkgs.xdotool];
  params = {
    xwindows = import ./xwindows.nix pkgs;
    wlrctl = pkgs.lib.getExe pkgs.wlrctl;
    heads = {
      Virtual-1 = "gpu0";
      Virtual-2 = "gpu1";
    };
    launchers = {
      Virtual-1 = feh "checker-hidpi";
      Virtual-2 = feh "checker-plain";
    };
  };
  scripts = [./xwayland-lib.py ./xwayland.py];
}
