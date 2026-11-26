# The variables that force each render layer onto its driver-free path:
# lys-ui's CPU renderer (shared memory, no Vulkan) for the greeter client and
# lys-wm's lavapipe renderer (Vulkan on the CPU, into the card's dumb buffers,
# shared through /dev/udmabuf) for its compositor.
# Neither needs a GPU vendor driver, so the greeter always renders — on a VM, a
# driverless box, or a non-NixOS install without nixGL.
{
  lys-wm = {
    LYS_UI_RENDERER = "cpu";
    LYS_WM_RENDERER = "lavapipe";
  };
}
