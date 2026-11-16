pkgs:
with pkgs; [
  just
  wayland-utils
  sway
  grim
  libxkbcommon
  weston
  foot
  wl-clipboard
  xinput
  dbus
  mesa-demos
  glslang
  spirv-tools
  gtk3.dev
  gtk4.dev
  (import ./alacritty-swrast.nix pkgs)
]
