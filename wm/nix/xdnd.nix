pkgs: let
  launch = import ./launch.nix pkgs;
  dragon = backend: args: "${pkgs.coreutils}/bin/env GDK_BACKEND=${backend} ${pkgs.dragon-drop}/bin/dragon-drop ${args}";
in {
  name = "xdnd";
  machineConfig = {
    environment.systemPackages = [pkgs.xwininfo pkgs.xdotool];
    fonts.packages = [pkgs.dejavu_fonts];
  };
  params = {
    xwindows = import ./xwindows.nix pkgs;
    launchers = {
      wayland-source = launch "wayland-source" (dragon "wayland" "/tmp/from-wayland.txt");
      x-target = launch "x-target" (dragon "x11" "--target --and-exit --print-path");
      x-source = launch "x-source" (dragon "x11" "/tmp/from-x.txt");
      wayland-target = launch "wayland-target" (dragon "wayland" "--target --and-exit --print-path");
    };
  };
  scripts = [./xwayland-lib.py ./xdnd.py];
}
