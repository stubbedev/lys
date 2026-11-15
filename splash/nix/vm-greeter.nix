# lys-splash hands the screen to lys-login's greeter on lys-wm without a
# blank: the splash boots through the NixOS module on virtio-gpu, and
# lys-login, the display manager, follows the contract in
# docs/splash/guide/how-it-works.md. The greeter's start waits on the test,
# which then dumps the screen as fast as the VM keeps up from the splash's
# last frame until the greeter shows, and reads the kernel's DRM log from that
# point on.
#
# No dump is black, the console or lys-wm's empty background. The splash fades
# its status group out and leaves its logo alone on black; the greeter's first
# frame draws that picture again, with the logo where the greeter's log says
# it put it, so from the splash's last frame on no dump changes the logo's box
# by more than a step of the greeter's fade, which greeter.nix stretches to two
# seconds so the dumps can follow it. The quit came from lys-login with retain,
# lys-splash-quit and the getty on tty1 never ran, and lys-wm's first commit
# after the splash dropped master is a nonblocking flip with no modeset, so the
# kernel's fbdev emulation never restored its console in between.
{
  pkgs,
  module,
  splash,
  self,
  wmPackage,
}:
pkgs.testers.runNixOSTest {
  name = "lys-splash-greeter";
  extraPythonPackages = p: [p.pillow];
  nodes.machine = {
    imports = [module (import ./greeter.nix {inherit pkgs splash self wmPackage;})];
    boot.lys-splash = {
      enable = true;
      package = splash;
    };
    boot.initrd.kernelModules = ["virtio_gpu"];
    virtualisation.qemu.options = ["-vga none" "-device virtio-gpu-pci"];
  };
  testScript =
    builtins.readFile ./screens.py
    + builtins.readFile ./stages.py
    + builtins.readFile ./vm-greeter.py;
}
