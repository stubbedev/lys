pkgs: {
  XKB_CONFIG_ROOT = "${pkgs.xkeyboard-config}/share/X11/xkb";
  LYS_WM_TEST_XCURSOR_PATH = "${pkgs.vanilla-dmz}/share/icons";
  FONTCONFIG_FILE = pkgs.makeFontsConf {fontDirectories = [pkgs.dejavu_fonts];};
  LYS_WM_LIBGBM = "${pkgs.libgbm}/lib/libgbm.so.1";
  GBM_BACKENDS_PATH = "${pkgs.mesa}/lib/gbm:/run/opengl-driver/lib/gbm";
  LYS_LIBVULKAN = "${pkgs.vulkan-loader}/lib/libvulkan.so.1";
  LYS_WM_LIBINPUT = "${pkgs.libinput.out}/lib/libinput.so.10";
  LYS_WM_XWAYLAND = "${pkgs.xwayland}/bin/Xwayland";
  __EGL_VENDOR_LIBRARY_DIRS = "${pkgs.mesa}/share/glvnd/egl_vendor.d";
  LIBGL_ALWAYS_SOFTWARE = "1";
  VK_DRIVER_FILES = "${pkgs.mesa}/share/vulkan/icd.d/lvp_icd.${pkgs.stdenv.hostPlatform.parsed.cpu.name}.json";
}
