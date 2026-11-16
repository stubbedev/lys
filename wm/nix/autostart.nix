pkgs: let
  entry = name: extra: ''
    [Desktop Entry]
    Type=Application
    Name=${name}
    Exec=${pkgs.bash}/bin/sh -c "printenv WAYLAND_DISPLAY > /tmp/autostart-${name}; exec sleep infinity"
    ${extra}
  '';
  system = {
    plain = "";
    only-lys-wm = "OnlyShowIn=GNOME;lys-wm;";
    hidden = "Hidden=true";
    only-gnome = "OnlyShowIn=GNOME;";
    not-lys-wm = "NotShowIn=lys-wm;";
    gnome-enabled-false = "X-GNOME-Autostart-enabled=false";
    systemd-skip = "X-systemd-skip=true";
    overridden = "";
  };
  user = {
    user = "";
    user-hidden = "Hidden=true";
    overridden = "Hidden=true";
  };
in {
  name = "autostart";
  machineConfig.environment.etc = pkgs.lib.mapAttrs' (name: extra: pkgs.lib.nameValuePair "xdg/autostart/${name}.desktop" {text = entry name extra;}) system;
  home = pkgs.lib.mapAttrs' (name: extra: pkgs.lib.nameValuePair ".config/autostart/${name}.desktop" (entry name extra)) user;
  params = {
    started = ["plain" "only-lys-wm" "gnome-enabled-false" "user"];
    skipped = ["hidden" "only-gnome" "not-lys-wm" "systemd-skip" "user-hidden" "overridden"];
  };
  scripts = [./autostart.py];
}
